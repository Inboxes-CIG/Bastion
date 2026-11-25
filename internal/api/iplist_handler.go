package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/inboxes-cig/bastion/internal/db"
	"github.com/inboxes-cig/bastion/internal/iplist"
	"gorm.io/gorm"
)

type IPListHandler struct {
	DB   *gorm.DB
	IPs  *iplist.List
}

func (h *IPListHandler) List(c *gin.Context) {
	if !requireSuperadmin(c) {
		return
	}
	var entries []db.IPListEntry
	if err := h.DB.Order("created_at DESC").Find(&entries).Error; err != nil {
		InternalError(c, err.Error())
		return
	}
	OK(c, entries)
}

type ipListRequest struct {
	Kind  string `json:"kind" binding:"required"`
	Value string `json:"value" binding:"required"`
	Note  string `json:"note"`
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
	e := db.IPListEntry{Kind: req.Kind, Value: strings.TrimSpace(req.Value), Note: req.Note}
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
	e.Kind = req.Kind
	e.Value = strings.TrimSpace(req.Value)
	e.Note = req.Note
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
