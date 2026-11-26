package api

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/inboxes-cig/bastion/internal/apikeys"
	"github.com/inboxes-cig/bastion/internal/audit"
	"github.com/inboxes-cig/bastion/internal/auth"
	"github.com/inboxes-cig/bastion/internal/db"
	"github.com/inboxes-cig/bastion/internal/iplist"
	"gorm.io/gorm"
)

type APIKeyHandler struct {
	DB   *gorm.DB
	Keys *apikeys.Service
}

type apiKeyRequest struct {
	Name         string     `json:"name" binding:"required"`
	ExpiresAt    *time.Time `json:"expires_at"`
	RateLimitRPM *int       `json:"rate_limit_rpm"`
}

func (h *APIKeyHandler) Create(c *gin.Context) {
	cl := auth.ClaimsFromContext(c)
	if cl == nil {
		Unauthorized(c, "unauthorized")
		return
	}
	if cl.RoleName != "superadmin" && cl.RoleName != "tenant_admin" {
		Forbidden(c, "forbidden")
		return
	}
	var req apiKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		BadRequest(c, "name required")
		return
	}
	if req.ExpiresAt != nil && !req.ExpiresAt.After(time.Now()) {
		BadRequest(c, "expires_at must be in the future")
		return
	}
	if req.RateLimitRPM != nil && *req.RateLimitRPM < 0 {
		BadRequest(c, "rate_limit_rpm must not be negative")
		return
	}
	plain, hash, pfx := apikeys.Generate()
	k := db.APIKey{
		UserID:       cl.UserID,
		Name:         name,
		KeyHash:      hash,
		Prefix:       pfx,
		ExpiresAt:    req.ExpiresAt,
		RateLimitRPM: req.RateLimitRPM,
	}
	if err := h.DB.Create(&k).Error; err != nil {
		BadRequest(c, err.Error())
		return
	}
	audit.SetDetail(c, "API-Key \""+name+"\" erstellt")
	c.JSON(201, gin.H{"data": gin.H{
		"key":            plain,
		"id":             k.ID,
		"name":           k.Name,
		"prefix":         k.Prefix,
		"expires_at":     k.ExpiresAt,
		"rate_limit_rpm": k.RateLimitRPM,
	}})
}

func (h *APIKeyHandler) List(c *gin.Context) {
	cl := auth.ClaimsFromContext(c)
	if cl == nil {
		Unauthorized(c, "unauthorized")
		return
	}
	if cl.RoleName != "superadmin" && cl.RoleName != "tenant_admin" {
		Forbidden(c, "forbidden")
		return
	}
	var keys []db.APIKey
	if err := h.DB.Where("user_id = ?", cl.UserID).Order("created_at DESC").Find(&keys).Error; err != nil {
		InternalError(c, err.Error())
		return
	}
	OK(c, keys)
}

type apiKeyAdminRow struct {
	db.APIKey
	Email string `json:"email"`
}

func (h *APIKeyHandler) AdminList(c *gin.Context) {
	if !requireSuperadmin(c) {
		return
	}
	var keys []db.APIKey
	if err := h.DB.Order("created_at DESC").Find(&keys).Error; err != nil {
		InternalError(c, err.Error())
		return
	}
	emails := map[uuid.UUID]string{}
	var users []db.User
	_ = h.DB.Select("id, email").Find(&users).Error
	for _, u := range users {
		emails[u.ID] = u.Email
	}
	out := make([]apiKeyAdminRow, 0, len(keys))
	for _, k := range keys {
		out = append(out, apiKeyAdminRow{APIKey: k, Email: emails[k.UserID]})
	}
	OK(c, out)
}

func (h *APIKeyHandler) Delete(c *gin.Context) {
	cl := auth.ClaimsFromContext(c)
	if cl == nil {
		Unauthorized(c, "unauthorized")
		return
	}
	if cl.RoleName != "superadmin" && cl.RoleName != "tenant_admin" {
		Forbidden(c, "forbidden")
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		BadRequest(c, "invalid id")
		return
	}
	var k db.APIKey
	if err := h.DB.First(&k, "id = ?", id).Error; err != nil {
		NotFound(c, "key not found")
		return
	}
	if cl.RoleName != "superadmin" && k.UserID != cl.UserID {
		Forbidden(c, "forbidden")
		return
	}
	now := time.Now()
	if err := h.DB.Model(&db.APIKey{}).Where("id = ?", id).Update("revoked_at", &now).Error; err != nil {
		InternalError(c, err.Error())
		return
	}
	audit.SetDetail(c, "API-Key \""+k.Name+"\" widerrufen")
	c.Status(204)
}

// ---- API IP allowlist ----

type apiIPRuleRequest struct {
	Value string `json:"value" binding:"required"`
	Note  string `json:"note"`
}

func (h *APIKeyHandler) ListIPRules(c *gin.Context) {
	if !requireSuperadmin(c) {
		return
	}
	var rules []db.APIIPRule
	if err := h.DB.Order("created_at DESC").Find(&rules).Error; err != nil {
		InternalError(c, err.Error())
		return
	}
	OK(c, gin.H{"rules": rules, "current_ip": clientIPFromProxy(c)})
}

func (h *APIKeyHandler) CreateIPRule(c *gin.Context) {
	if !requireSuperadmin(c) {
		return
	}
	var req apiIPRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}
	if err := iplist.Validate(req.Value); err != nil {
		BadRequest(c, err.Error())
		return
	}
	r := db.APIIPRule{Value: strings.TrimSpace(req.Value), Note: req.Note}
	if err := h.DB.Create(&r).Error; err != nil {
		BadRequest(c, err.Error())
		return
	}
	if h.Keys != nil {
		_ = h.Keys.Reload()
	}
	audit.SetDetail(c, "API-IP-Regel "+r.Value+" angelegt")
	Created(c, r)
}

func (h *APIKeyHandler) UpdateIPRule(c *gin.Context) {
	if !requireSuperadmin(c) {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		BadRequest(c, "invalid id")
		return
	}
	var r db.APIIPRule
	if err := h.DB.First(&r, "id = ?", id).Error; err != nil {
		NotFound(c, "entry not found")
		return
	}
	var req apiIPRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}
	if err := iplist.Validate(req.Value); err != nil {
		BadRequest(c, err.Error())
		return
	}
	r.Value = strings.TrimSpace(req.Value)
	r.Note = req.Note
	if err := h.DB.Save(&r).Error; err != nil {
		InternalError(c, err.Error())
		return
	}
	if h.Keys != nil {
		_ = h.Keys.Reload()
	}
	audit.SetDetail(c, "API-IP-Regel "+r.Value+" geändert")
	OK(c, r)
}

func (h *APIKeyHandler) DeleteIPRule(c *gin.Context) {
	if !requireSuperadmin(c) {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		BadRequest(c, "invalid id")
		return
	}
	var r db.APIIPRule
	if err := h.DB.First(&r, "id = ?", id).Error; err != nil {
		NotFound(c, "entry not found")
		return
	}
	if err := h.DB.Delete(&db.APIIPRule{}, "id = ?", id).Error; err != nil {
		InternalError(c, err.Error())
		return
	}
	if h.Keys != nil {
		_ = h.Keys.Reload()
	}
	audit.SetDetail(c, "API-IP-Regel "+r.Value+" gelöscht")
	c.Status(204)
}

// ---- API defaults ----

type apiSettingsRequest struct {
	DefaultRateLimitRPM int `json:"default_rate_limit_rpm"`
}

func (h *APIKeyHandler) GetSettings(c *gin.Context) {
	if !requireSuperadmin(c) {
		return
	}
	var st db.APISettings
	if err := h.DB.Order("created_at ASC").First(&st).Error; err != nil {
		OK(c, gin.H{"default_rate_limit_rpm": 0})
		return
	}
	OK(c, st)
}

func (h *APIKeyHandler) SetSettings(c *gin.Context) {
	if !requireSuperadmin(c) {
		return
	}
	var req apiSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}
	if req.DefaultRateLimitRPM < 0 {
		BadRequest(c, "default_rate_limit_rpm must not be negative")
		return
	}
	var st db.APISettings
	existing := h.DB.Order("created_at ASC").First(&st).Error == nil
	if !existing {
		st = db.APISettings{}
	}
	st.DefaultRateLimitRPM = req.DefaultRateLimitRPM
	if err := h.DB.Save(&st).Error; err != nil {
		InternalError(c, err.Error())
		return
	}
	if h.Keys != nil {
		_ = h.Keys.Reload()
	}
	audit.SetDetail(c, fmt.Sprintf("API-Default-Rate-Limit auf %d/min gesetzt", req.DefaultRateLimitRPM))
	OK(c, st)
}
