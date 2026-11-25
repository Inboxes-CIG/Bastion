package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/inboxes-cig/bastion/internal/db"
	"github.com/inboxes-cig/bastion/internal/iplist"
	"gorm.io/gorm"
)

type IPListHandler struct {
	DB  *gorm.DB
	IPs *iplist.List
}

func (h *IPListHandler) List(c *gin.Context) {
	if !requireSuperadmin(c) {
		return
	}
	var entries []db.IPListEntry
	q := h.DB.Where("expires_at IS NULL OR expires_at > ?", time.Now())
	if err := q.Order("created_at DESC").Find(&entries).Error; err != nil {
		InternalError(c, err.Error())
		return
	}
	OK(c, entries)
}

type ipListRequest struct {
	Kind       string     `json:"kind" binding:"required"`
	Value      string     `json:"value" binding:"required"`
	Note       string     `json:"note"`
	TTLSeconds *int       `json:"ttl_seconds"`
	ExpiresAt  *time.Time `json:"expires_at"`
}

// expiryFromRequest resolves ttl_seconds / expires_at into an absolute
// expiry. keep=true means the request carried neither field (PATCH-like
// semantics for Update: leave the stored expiry untouched).
func expiryFromRequest(req ipListRequest) (exp *time.Time, keep bool, err error) {
	if req.ExpiresAt != nil {
		t := req.ExpiresAt.UTC()
		if !t.After(time.Now()) {
			return nil, false, fmt.Errorf("expires_at must be in the future")
		}
		return &t, false, nil
	}
	if req.TTLSeconds != nil {
		if *req.TTLSeconds < 0 {
			return nil, false, fmt.Errorf("ttl_seconds must not be negative")
		}
		if *req.TTLSeconds == 0 {
			return nil, false, nil // explicit "permanent"
		}
		t := time.Now().Add(time.Duration(*req.TTLSeconds) * time.Second)
		return &t, false, nil
	}
	return nil, true, nil
}

func (h *IPListHandler) Create(c *gin.Context) {
	if !requireSuperadmin(c) {
		return
	}
	var req ipListRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}
	if req.Kind != "allow" && req.Kind != "deny" {
		BadRequest(c, "kind must be allow or deny")
		return
	}
	if err := iplist.Validate(req.Value); err != nil {
		BadRequest(c, err.Error())
		return
	}
	exp, _, err := expiryFromRequest(req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	e := db.IPListEntry{Kind: req.Kind, Value: strings.TrimSpace(req.Value), Note: req.Note, ExpiresAt: exp, Source: "manual"}
	if err := h.DB.Create(&e).Error; err != nil {
		BadRequest(c, err.Error())
		return
	}
	if h.IPs != nil {
		_ = h.IPs.Reload()
	}
	Created(c, e)
}

func (h *IPListHandler) Update(c *gin.Context) {
	if !requireSuperadmin(c) {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		BadRequest(c, "invalid id")
		return
	}
	var e db.IPListEntry
	if err := h.DB.First(&e, "id = ?", id).Error; err != nil {
		NotFound(c, "entry not found")
		return
	}
	var req ipListRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}
	if req.Kind != "allow" && req.Kind != "deny" {
		BadRequest(c, "kind must be allow or deny")
		return
	}
	if err := iplist.Validate(req.Value); err != nil {
		BadRequest(c, err.Error())
		return
	}
	exp, keep, err := expiryFromRequest(req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	e.Kind = req.Kind
	e.Value = strings.TrimSpace(req.Value)
	e.Note = req.Note
	if !keep {
		e.ExpiresAt = exp
	}
	if err := h.DB.Save(&e).Error; err != nil {
		InternalError(c, err.Error())
		return
	}
	if h.IPs != nil {
		_ = h.IPs.Reload()
	}
	OK(c, e)
}

func (h *IPListHandler) Delete(c *gin.Context) {
	if !requireSuperadmin(c) {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		BadRequest(c, "invalid id")
		return
	}
	if err := h.DB.Delete(&db.IPListEntry{}, "id = ?", id).Error; err != nil {
		InternalError(c, err.Error())
		return
	}
	if h.IPs != nil {
		_ = h.IPs.Reload()
	}
	c.Status(http.StatusNoContent)
}
