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

// scope resolves :id and optionally :lid. A nil location means the site scope.
func (h *SiteAccessHandler) scope(c *gin.Context) (uuid.UUID, *uuid.UUID, bool) {
	sid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		BadRequest(c, "invalid site id")
		return uuid.Nil, nil, false
	}
	var site db.Site
	if err := h.DB.Select("tenant_id").First(&site, "id = ?", sid).Error; err != nil {
		NotFound(c, "site not found")
		return uuid.Nil, nil, false
	}
	if !canAccessTenant(c, &site.TenantID) {
		NotFound(c, "site not found")
		return uuid.Nil, nil, false
	}
	raw := c.Param("lid")
	if raw == "" {
		return sid, nil, true
	}
	lid, err := uuid.Parse(raw)
	if err != nil {
		BadRequest(c, "invalid location id")
		return uuid.Nil, nil, false
	}
	var count int64
	h.DB.Model(&db.SiteLocation{}).Where("id = ? AND site_id = ?", lid, sid).Count(&count)
	if count == 0 {
		NotFound(c, "location not found")
		return uuid.Nil, nil, false
	}
	return sid, &lid, true
}

func scopeWhere(q *gorm.DB, sid uuid.UUID, lid *uuid.UUID) *gorm.DB {
	if lid != nil {
		return q.Where("site_id = ? AND location_id = ?", sid, *lid)
	}
	return q.Where("site_id = ? AND location_id IS NULL", sid)
}

func (h *SiteAccessHandler) invalidate(sid uuid.UUID) {
	if h.Access != nil {
		h.Access.Invalidate(sid.String())
	}
}

func (h *SiteAccessHandler) ListUsers(c *gin.Context) {
	sid, lid, ok := h.scope(c)
	if !ok {
		return
	}
	var users []db.SiteAuthUser
	if err := scopeWhere(h.DB, sid, lid).Order("created_at").Find(&users).Error; err != nil {
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
	sid, lid, ok := h.scope(c)
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
	var count int64
	scopeWhere(h.DB.Model(&db.SiteAuthUser{}), sid, lid).Where("username = ?", req.Username).Count(&count)
	if count > 0 {
		BadRequest(c, "username already exists in this scope")
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		InternalError(c, err.Error())
		return
	}
	u := db.SiteAuthUser{SiteID: sid, LocationID: lid, Username: req.Username, PasswordHash: hash, Active: true}
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
	sid, lid, ok := h.scope(c)
	if !ok {
		return
	}
	uid, err := uuid.Parse(c.Param("uid"))
	if err != nil {
		BadRequest(c, "invalid user id")
		return
	}
	var u db.SiteAuthUser
	if err := scopeWhere(h.DB, sid, lid).First(&u, "id = ?", uid).Error; err != nil {
		NotFound(c, "user not found")
		return
	}
	var req siteUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}
	if req.Username != "" && req.Username != u.Username {
		var count int64
		scopeWhere(h.DB.Model(&db.SiteAuthUser{}), sid, lid).Where("username = ? AND id <> ?", req.Username, uid).Count(&count)
		if count > 0 {
			BadRequest(c, "username already exists in this scope")
			return
		}
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
	sid, lid, ok := h.scope(c)
	if !ok {
		return
	}
	uid, err := uuid.Parse(c.Param("uid"))
	if err != nil {
		BadRequest(c, "invalid user id")
		return
	}
	if err := scopeWhere(h.DB, sid, lid).Delete(&db.SiteAuthUser{}, "id = ?", uid).Error; err != nil {
		InternalError(c, err.Error())
		return
	}
	h.invalidate(sid)
	c.Status(http.StatusNoContent)
}

func (h *SiteAccessHandler) ListIPRules(c *gin.Context) {
	sid, lid, ok := h.scope(c)
	if !ok {
		return
	}
	var rules []db.SiteIPRule
	if err := scopeWhere(h.DB, sid, lid).Order("created_at").Find(&rules).Error; err != nil {
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
	sid, lid, ok := h.scope(c)
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
	r := db.SiteIPRule{SiteID: sid, LocationID: lid, Kind: req.Kind, Value: strings.TrimSpace(req.Value), Note: req.Note}
	if err := h.DB.Create(&r).Error; err != nil {
		BadRequest(c, err.Error())
		return
	}
	h.invalidate(sid)
	Created(c, r)
}

func (h *SiteAccessHandler) UpdateIPRule(c *gin.Context) {
	sid, lid, ok := h.scope(c)
	if !ok {
		return
	}
	rid, err := uuid.Parse(c.Param("rid"))
	if err != nil {
		BadRequest(c, "invalid rule id")
		return
	}
	var r db.SiteIPRule
	if err := scopeWhere(h.DB, sid, lid).First(&r, "id = ?", rid).Error; err != nil {
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
	sid, lid, ok := h.scope(c)
	if !ok {
		return
	}
	rid, err := uuid.Parse(c.Param("rid"))
	if err != nil {
		BadRequest(c, "invalid rule id")
		return
	}
	if err := scopeWhere(h.DB, sid, lid).Delete(&db.SiteIPRule{}, "id = ?", rid).Error; err != nil {
		InternalError(c, err.Error())
		return
	}
	h.invalidate(sid)
	c.Status(http.StatusNoContent)
}
