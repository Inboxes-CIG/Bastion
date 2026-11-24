package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/inboxes-cig/bastion/internal/auth"
	"github.com/inboxes-cig/bastion/internal/db"
	"github.com/inboxes-cig/bastion/internal/iplist"
	"github.com/inboxes-cig/bastion/internal/siteaccess"
	"gorm.io/gorm"
)

type SiteAccessHandler struct {
	DB     *gorm.DB
	Access *siteaccess.Cache
}

func (h *SiteAccessHandler) siteID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		BadRequest(c, "invalid site id")
		return uuid.Nil, false
	}
	return id, true
}

func (h *SiteAccessHandler) invalidate(sid uuid.UUID) {
	if h.Access != nil {
		h.Access.Invalidate(sid.String())
	}
}

func (h *SiteAccessHandler) ListUsers(c *gin.Context) {
	sid, ok := h.siteID(c)
	if !ok {
		return
	}
	var users []db.SiteAuthUser
	if err := h.DB.Where("site_id = ?", sid).Order("created_at").Find(&users).Error; err != nil {
		InternalError(c, err.Error())
		return
	}
	OK(c, users)
}

type siteUserRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password"`
	Active   *bool  `json:"active"`
}

func (h *SiteAccessHandler) CreateUser(c *gin.Context) {
	sid, ok := h.siteID(c)
	if !ok {
		return
	}
	var req siteUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}
	if req.Password == "" {
		BadRequest(c, "password required")
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		InternalError(c, err.Error())
		return
	}
	u := db.SiteAuthUser{SiteID: sid, Username: req.Username, PasswordHash: hash, Active: true}
	if req.Active != nil {
		u.Active = *req.Active
	}
	if err := h.DB.Create(&u).Error; err != nil {
		BadRequest(c, err.Error())
		return
	}
	h.invalidate(sid)
	Created(c, u)
}

func (h *SiteAccessHandler) UpdateUser(c *gin.Context) {
	sid, ok := h.siteID(c)
	if !ok {
		return
	}
	uid, err := uuid.Parse(c.Param("uid"))
	if err != nil {
		BadRequest(c, "invalid user id")
		return
	}
	var u db.SiteAuthUser
	if err := h.DB.First(&u, "id = ? AND site_id = ?", uid, sid).Error; err != nil {
		NotFound(c, "user not found")
		return
	}
	var req siteUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}
	if req.Username != "" {
		u.Username = req.Username
	}
	if req.Password != "" {
		if hash, err := auth.HashPassword(req.Password); err == nil {
			u.PasswordHash = hash
		}
	}
	if req.Active != nil {
		u.Active = *req.Active
	}
	if err := h.DB.Save(&u).Error; err != nil {
		InternalError(c, err.Error())
		return
	}
	h.invalidate(sid)
	OK(c, u)
}

func (h *SiteAccessHandler) DeleteUser(c *gin.Context) {
	sid, ok := h.siteID(c)
	if !ok {
		return
	}
	uid, err := uuid.Parse(c.Param("uid"))
	if err != nil {
		BadRequest(c, "invalid user id")
		return
	}
	if err := h.DB.Delete(&db.SiteAuthUser{}, "id = ? AND site_id = ?", uid, sid).Error; err != nil {
		InternalError(c, err.Error())
		return
	}
	h.invalidate(sid)
	c.Status(http.StatusNoContent)
}

func (h *SiteAccessHandler) ListIPRules(c *gin.Context) {
	sid, ok := h.siteID(c)
	if !ok {
		return
	}
	var rules []db.SiteIPRule
	if err := h.DB.Where("site_id = ?", sid).Order("created_at").Find(&rules).Error; err != nil {
		InternalError(c, err.Error())
		return
	}
	OK(c, rules)
}

type siteIPRuleRequest struct {
	Kind  string `json:"kind" binding:"required"`
	Value string `json:"value" binding:"required"`
	Note  string `json:"note"`
}

func (h *SiteAccessHandler) CreateIPRule(c *gin.Context) {
	sid, ok := h.siteID(c)
	if !ok {
		return
	}
	var req siteIPRuleRequest
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
	r := db.SiteIPRule{SiteID: sid, Kind: req.Kind, Value: strings.TrimSpace(req.Value), Note: req.Note}
	if err := h.DB.Create(&r).Error; err != nil {
		BadRequest(c, err.Error())
		return
	}
	h.invalidate(sid)
	Created(c, r)
}

func (h *SiteAccessHandler) UpdateIPRule(c *gin.Context) {
	sid, ok := h.siteID(c)
	if !ok {
		return
	}
	rid, err := uuid.Parse(c.Param("rid"))
	if err != nil {
		BadRequest(c, "invalid rule id")
		return
	}
	var r db.SiteIPRule
	if err := h.DB.First(&r, "id = ? AND site_id = ?", rid, sid).Error; err != nil {
		NotFound(c, "rule not found")
		return
	}
	var req siteIPRuleRequest
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
	r.Kind = req.Kind
	r.Value = strings.TrimSpace(req.Value)
	r.Note = req.Note
	if err := h.DB.Save(&r).Error; err != nil {
		InternalError(c, err.Error())
		return
	}
	h.invalidate(sid)
	OK(c, r)
}

func (h *SiteAccessHandler) DeleteIPRule(c *gin.Context) {
	sid, ok := h.siteID(c)
	if !ok {
		return
	}
	rid, err := uuid.Parse(c.Param("rid"))
	if err != nil {
		BadRequest(c, "invalid rule id")
		return
	}
	if err := h.DB.Delete(&db.SiteIPRule{}, "id = ? AND site_id = ?", rid, sid).Error; err != nil {
		InternalError(c, err.Error())
		return
	}
	h.invalidate(sid)
	c.Status(http.StatusNoContent)
}
