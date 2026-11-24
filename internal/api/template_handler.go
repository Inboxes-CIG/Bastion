package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/inboxes-cig/bastion/internal/audit"
	"github.com/inboxes-cig/bastion/internal/auth"
	"github.com/inboxes-cig/bastion/internal/db"
	"gorm.io/gorm"
)

type TemplateHandler struct {
	DB *gorm.DB
}

func (h *TemplateHandler) List(c *gin.Context) {
	cl := auth.ClaimsFromContext(c)
	var templates []db.SiteTemplate
	q := h.DB
	if cl != nil && cl.RoleName != "superadmin" && cl.TenantID != nil {
		q = q.Where("tenant_id = ?", *cl.TenantID)
	}
	if err := q.Find(&templates).Error; err != nil {
		InternalError(c, err.Error())
		return
	}
	OK(c, templates)
}

func (h *TemplateHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		BadRequest(c, "invalid id")
		return
	}
	var t db.SiteTemplate
	if err := h.DB.First(&t, "id = ?", id).Error; err != nil {
		NotFound(c, "template not found")
		return
	}
	OK(c, t)
}

type templateRequest struct {
	TenantID                   string   `json:"tenant_id"`
	Name                       string   `json:"name" binding:"required"`
	Description                string   `json:"description"`
	UpstreamURL                *string  `json:"upstream_url"`
	UpstreamHost               *string  `json:"upstream_host"`
	UpstreamInsecureSkipVerify *bool    `json:"upstream_insecure_skip_verify"`
	GeoIPEnabled               *bool    `json:"geoip_enabled"`
	GeoIPMode                  *string  `json:"geoip_mode"`
	GeoIPCountries             []string `json:"geoip_countries"`
	HTTP2Enabled               *bool    `json:"http2_enabled"`
	HTTP3Enabled               *bool    `json:"http3_enabled"`
	ForceHTTPS                 *bool    `json:"force_https"`
	HSTSEnabled                *bool    `json:"hsts_enabled"`
	HSTSMaxAge                 *int     `json:"hsts_max_age"`
	WebSocketEnabled           *bool    `json:"websocket_enabled"`
	CompressionEnabled         *bool    `json:"compression_enabled"`
	RateLimitRPS               *int     `json:"rate_limit_rps"`
	RateLimitBurst             *int     `json:"rate_limit_burst"`
	MaxBodySizeMB              *int     `json:"max_body_size_mb"`
	CacheEnabled               *bool    `json:"cache_enabled"`
	CacheTTL                   *int     `json:"cache_ttl"`
	CacheMaxObjectKB           *int     `json:"cache_max_object_kb"`
	AuthEnabled                *bool    `json:"auth_enabled"`
}

func (h *TemplateHandler) Create(c *gin.Context) {
	var req templateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}
	cl := auth.ClaimsFromContext(c)
	tenantID := req.TenantID
	if cl != nil && cl.RoleName != "superadmin" && cl.TenantID != nil {
		tenantID = cl.TenantID.String()
	}
	if tenantID == "" {
		BadRequest(c, "tenant_id required")
		return
	}
	tid, err := uuid.Parse(tenantID)
	if err != nil {
		BadRequest(c, "invalid tenant_id")
		return
	}
	t := db.SiteTemplate{
		TenantID:                   tid,
		Name:                       req.Name,
		Description:                req.Description,
		UpstreamURL:                req.UpstreamURL,
		UpstreamHost:               req.UpstreamHost,
		UpstreamInsecureSkipVerify: req.UpstreamInsecureSkipVerify,
		GeoIPEnabled:               req.GeoIPEnabled,
		GeoIPMode:                  req.GeoIPMode,
		GeoIPCountries:             db.StringArray(req.GeoIPCountries),
		HTTP2Enabled:               req.HTTP2Enabled,
		HTTP3Enabled:               req.HTTP3Enabled,
		ForceHTTPS:                 req.ForceHTTPS,
		HSTSEnabled:                req.HSTSEnabled,
		HSTSMaxAge:                 req.HSTSMaxAge,
		WebSocketEnabled:           req.WebSocketEnabled,
		CompressionEnabled:         req.CompressionEnabled,
		RateLimitRPS:               req.RateLimitRPS,
		RateLimitBurst:             req.RateLimitBurst,
		MaxBodySizeMB:              req.MaxBodySizeMB,
		CacheEnabled:               req.CacheEnabled,
		CacheTTL:                   req.CacheTTL,
		CacheMaxObjectKB:           req.CacheMaxObjectKB,
		AuthEnabled:                req.AuthEnabled,
	}
	if err := h.DB.Create(&t).Error; err != nil {
		BadRequest(c, err.Error())
		return
	}
	Created(c, t)
}

func (h *TemplateHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		BadRequest(c, "invalid id")
		return
	}
	var t db.SiteTemplate
	if err := h.DB.First(&t, "id = ?", id).Error; err != nil {
		NotFound(c, "template not found")
		return
	}
	var req templateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}
	if req.Name != "" {
		t.Name = req.Name
	}
	t.Description = req.Description
	// Update pointer fields (nil means clear override -> inherit nothing, template always defines)
	t.UpstreamURL = req.UpstreamURL
	t.UpstreamHost = req.UpstreamHost
	t.UpstreamInsecureSkipVerify = req.UpstreamInsecureSkipVerify
	t.GeoIPEnabled = req.GeoIPEnabled
	t.GeoIPMode = req.GeoIPMode
	t.GeoIPCountries = db.StringArray(req.GeoIPCountries)
	t.HTTP2Enabled = req.HTTP2Enabled
	t.HTTP3Enabled = req.HTTP3Enabled
	t.ForceHTTPS = req.ForceHTTPS
	t.HSTSEnabled = req.HSTSEnabled
	t.HSTSMaxAge = req.HSTSMaxAge
	t.WebSocketEnabled = req.WebSocketEnabled
	t.CompressionEnabled = req.CompressionEnabled
	t.RateLimitRPS = req.RateLimitRPS
	t.RateLimitBurst = req.RateLimitBurst
	t.MaxBodySizeMB = req.MaxBodySizeMB
	t.CacheEnabled = req.CacheEnabled
	t.CacheTTL = req.CacheTTL
	t.CacheMaxObjectKB = req.CacheMaxObjectKB
	t.AuthEnabled = req.AuthEnabled

	if err := h.DB.Save(&t).Error; err != nil {
		InternalError(c, err.Error())
		return
	}
	OK(c, t)
}

func (h *TemplateHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		BadRequest(c, "invalid id")
		return
	}
	// Detach sites from this template
	var tpl db.SiteTemplate
	if err := h.DB.Select("name").First(&tpl, "id = ?", id).Error; err == nil {
		audit.SetDetail(c, fmt.Sprintf("Template %q gelöscht", tpl.Name))
	}
	h.DB.Model(&db.Site{}).Where("template_id = ?", id).Update("template_id", nil)
	if err := h.DB.Delete(&db.SiteTemplate{}, "id = ?", id).Error; err != nil {
		InternalError(c, err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}
