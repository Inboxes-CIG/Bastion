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

type SiteHandler struct {
	DB *gorm.DB
}

func (h *SiteHandler) List(c *gin.Context) {
	cl := auth.ClaimsFromContext(c)
	var sites []db.Site
	q := h.DB
	if cl != nil && cl.RoleName != "superadmin" && cl.TenantID != nil {
		q = q.Where("tenant_id = ?", *cl.TenantID)
	}
	if err := q.Find(&sites).Error; err != nil {
		InternalError(c, err.Error())
		return
	}
	OK(c, sites)
}

type siteRequest struct {
	TenantID                   string   `json:"tenant_id"`
	TemplateID                 *string  `json:"template_id"`
	Domain                     string   `json:"domain" binding:"required"`
	ExtraDomains               []string `json:"extra_domains"`
	UpstreamURL                *string  `json:"upstream_url"`
	UpstreamHost               *string  `json:"upstream_host"`
	UpstreamInsecureSkipVerify *bool    `json:"upstream_insecure_skip_verify"`
	CertID                     *string  `json:"cert_id"`
	Enabled                    *bool    `json:"enabled"`

	// Override fields (nil = inherit from template)
	GeoIPEnabled       *bool     `json:"geoip_enabled"`
	GeoIPMode          *string   `json:"geoip_mode"`
	GeoIPCountries     *[]string `json:"geoip_countries"`
	HTTP2Enabled       *bool     `json:"http2_enabled"`
	HTTP3Enabled       *bool     `json:"http3_enabled"`
	ForceHTTPS         *bool     `json:"force_https"`
	HSTSEnabled        *bool     `json:"hsts_enabled"`
	HSTSMaxAge         *int      `json:"hsts_max_age"`
	WebSocketEnabled   *bool     `json:"websocket_enabled"`
	CompressionEnabled *bool     `json:"compression_enabled"`
	RateLimitRPS       *int      `json:"rate_limit_rps"`
	RateLimitBurst     *int      `json:"rate_limit_burst"`
	MaxBodySizeMB      *int      `json:"max_body_size_mb"`
	CacheEnabled       *bool     `json:"cache_enabled"`
	CacheTTL           *int      `json:"cache_ttl"`
	CacheMaxObjectKB   *int      `json:"cache_max_object_kb"`
	AuthEnabled        *bool     `json:"auth_enabled"`
}

func (h *SiteHandler) Create(c *gin.Context) {
	var req siteRequest
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
	site := db.Site{
		TenantID:                   tid,
		Domain:                     req.Domain,
		ExtraDomains:               db.StringArray(req.ExtraDomains),
		UpstreamURL:                req.UpstreamURL,
		UpstreamHost:               req.UpstreamHost,
		UpstreamInsecureSkipVerify: req.UpstreamInsecureSkipVerify,
		GeoIPEnabled:               req.GeoIPEnabled,
		GeoIPMode:                  req.GeoIPMode,
		GeoIPCountries:             toStrArray(req.GeoIPCountries),
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
		Enabled:                    true,
	}
	if req.Enabled != nil {
		site.Enabled = *req.Enabled
	}
	if req.TemplateID != nil && *req.TemplateID != "" {
		if tid2, err := uuid.Parse(*req.TemplateID); err == nil {
			site.TemplateID = &tid2
		}
	}
	if req.CertID != nil && *req.CertID != "" {
		if cid, err := uuid.Parse(*req.CertID); err == nil {
			site.CertID = &cid
		}
	}
	if err := h.DB.Create(&site).Error; err != nil {
		BadRequest(c, err.Error())
		return
	}
	Created(c, site)
}

func (h *SiteHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		BadRequest(c, "invalid id")
		return
	}
	var site db.Site
	if err := h.DB.Preload("Locations").First(&site, "id = ?", id).Error; err != nil {
		NotFound(c, "site not found")
		return
	}
	OK(c, site)
}

func (h *SiteHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		BadRequest(c, "invalid id")
		return
	}
	var site db.Site
	if err := h.DB.First(&site, "id = ?", id).Error; err != nil {
		NotFound(c, "site not found")
		return
	}
	var req siteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}
	if req.Domain != "" {
		site.Domain = req.Domain
	}
	if req.ExtraDomains != nil {
		site.ExtraDomains = db.StringArray(req.ExtraDomains)
	}
	if req.UpstreamURL != nil {
		site.UpstreamURL = req.UpstreamURL
	}
	if req.UpstreamHost != nil {
		site.UpstreamHost = req.UpstreamHost
	}
	site.UpstreamInsecureSkipVerify = req.UpstreamInsecureSkipVerify
	if req.TemplateID != nil {
		if *req.TemplateID == "" {
			site.TemplateID = nil
		} else if tid2, err := uuid.Parse(*req.TemplateID); err == nil {
			site.TemplateID = &tid2
		}
	}
	if req.CertID != nil {
		if *req.CertID == "" {
			site.CertID = nil
		} else if cid, err := uuid.Parse(*req.CertID); err == nil {
			site.CertID = &cid
		}
	}
	if req.Enabled != nil {
		site.Enabled = *req.Enabled
	}
	// override fields: set pointer (nil clears override -> inherit)
	site.GeoIPEnabled = req.GeoIPEnabled
	site.GeoIPMode = req.GeoIPMode
	site.GeoIPCountries = toStrArray(req.GeoIPCountries)
	site.HTTP2Enabled = req.HTTP2Enabled
	site.HTTP3Enabled = req.HTTP3Enabled
	site.ForceHTTPS = req.ForceHTTPS
	site.HSTSEnabled = req.HSTSEnabled
	site.HSTSMaxAge = req.HSTSMaxAge
	site.WebSocketEnabled = req.WebSocketEnabled
	site.CompressionEnabled = req.CompressionEnabled
	site.RateLimitRPS = req.RateLimitRPS
	site.RateLimitBurst = req.RateLimitBurst
	site.MaxBodySizeMB = req.MaxBodySizeMB
	site.CacheEnabled = req.CacheEnabled
	site.CacheTTL = req.CacheTTL
	site.CacheMaxObjectKB = req.CacheMaxObjectKB
	site.AuthEnabled = req.AuthEnabled

	if err := h.DB.Save(&site).Error; err != nil {
		InternalError(c, err.Error())
		return
	}
	OK(c, site)
}

func (h *SiteHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		BadRequest(c, "invalid id")
		return
	}
	var site db.Site
	if err := h.DB.Select("domain").First(&site, "id = ?", id).Error; err == nil {
		audit.SetDetail(c, fmt.Sprintf("Site %q gelöscht", site.Domain))
	}
	if err := h.DB.Delete(&db.Site{}, "id = ?", id).Error; err != nil {
		InternalError(c, err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

func toStrArray(p *[]string) db.StringArray {
	if p == nil {
		return nil
	}
	return db.StringArray(*p)
}

// EffectiveConfig resolves a site's effective configuration by merging template defaults.
func EffectiveSiteConfig(site db.Site, tmpl *db.SiteTemplate) map[string]interface{} {
	getStr := func(siteVal *string, tmplVal *string) *string {
		if siteVal != nil {
			return siteVal
		}
		if tmpl != nil {
			return tmplVal
		}
		return nil
	}
	getBool := func(siteVal *bool, tmplVal *bool) *bool {
		if siteVal != nil {
			return siteVal
		}
		if tmpl != nil {
			return tmplVal
		}
		return nil
	}
	getInt := func(siteVal *int, tmplVal *int) *int {
		if siteVal != nil {
			return siteVal
		}
		if tmpl != nil {
			return tmplVal
		}
		return nil
	}
	cfg := map[string]interface{}{}
	cfg["upstream_url"] = getStr(site.UpstreamURL, nil)
	if tmpl != nil {
		if site.UpstreamURL == nil {
			cfg["upstream_url"] = tmpl.UpstreamURL
		}
	}
	cfg["upstream_insecure_skip_verify"] = getBool(site.UpstreamInsecureSkipVerify, nil)
	cfg["geoip_enabled"] = getBool(site.GeoIPEnabled, nil)
	cfg["geoip_mode"] = getStr(site.GeoIPMode, nil)
	cfg["http2_enabled"] = getBool(site.HTTP2Enabled, nil)
	cfg["http3_enabled"] = getBool(site.HTTP3Enabled, nil)
	cfg["force_https"] = getBool(site.ForceHTTPS, nil)
	cfg["hsts_enabled"] = getBool(site.HSTSEnabled, nil)
	cfg["hsts_max_age"] = getInt(site.HSTSMaxAge, nil)
	cfg["websocket_enabled"] = getBool(site.WebSocketEnabled, nil)
	cfg["compression_enabled"] = getBool(site.CompressionEnabled, nil)
	cfg["rate_limit_rps"] = getInt(site.RateLimitRPS, nil)
	cfg["rate_limit_burst"] = getInt(site.RateLimitBurst, nil)
	cfg["max_body_size_mb"] = getInt(site.MaxBodySizeMB, nil)
	if tmpl != nil {
		if site.GeoIPEnabled == nil {
			cfg["geoip_enabled"] = tmpl.GeoIPEnabled
		}
		if site.UpstreamInsecureSkipVerify == nil {
			cfg["upstream_insecure_skip_verify"] = tmpl.UpstreamInsecureSkipVerify
		}
		if site.GeoIPMode == nil {
			cfg["geoip_mode"] = tmpl.GeoIPMode
		}
		if site.HTTP2Enabled == nil {
			cfg["http2_enabled"] = tmpl.HTTP2Enabled
		}
		if site.HTTP3Enabled == nil {
			cfg["http3_enabled"] = tmpl.HTTP3Enabled
		}
		if site.ForceHTTPS == nil {
			cfg["force_https"] = tmpl.ForceHTTPS
		}
		if site.HSTSEnabled == nil {
			cfg["hsts_enabled"] = tmpl.HSTSEnabled
		}
		if site.HSTSMaxAge == nil {
			cfg["hsts_max_age"] = tmpl.HSTSMaxAge
		}
		if site.WebSocketEnabled == nil {
			cfg["websocket_enabled"] = tmpl.WebSocketEnabled
		}
		if site.CompressionEnabled == nil {
			cfg["compression_enabled"] = tmpl.CompressionEnabled
		}
		if site.RateLimitRPS == nil {
			cfg["rate_limit_rps"] = tmpl.RateLimitRPS
		}
		if site.RateLimitBurst == nil {
			cfg["rate_limit_burst"] = tmpl.RateLimitBurst
		}
		if site.MaxBodySizeMB == nil {
			cfg["max_body_size_mb"] = tmpl.MaxBodySizeMB
		}
		if site.UpstreamURL == nil {
			cfg["upstream_url"] = tmpl.UpstreamURL
		}
	}
	_ = getStr
	_ = getBool
	_ = getInt
	return cfg
}
