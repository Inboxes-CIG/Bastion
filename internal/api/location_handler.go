package api

import (
	"fmt"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/inboxes-cig/bastion/internal/cache"
	"github.com/inboxes-cig/bastion/internal/db"
	"gorm.io/gorm"
)

type LocationHandler struct {
	DB    *gorm.DB
	Cache *cache.Cache
}

type locationRequest struct {
	PathPrefix string `json:"path_prefix"`
	Enabled    *bool  `json:"enabled"`

	UpstreamURL      *string   `json:"upstream_url"`
	UpstreamHost     *string   `json:"upstream_host"`
	GeoIPEnabled     *bool     `json:"geoip_enabled"`
	GeoIPMode        *string   `json:"geoip_mode"`
	GeoIPCountries   *[]string `json:"geoip_countries"`
	HTTP3Enabled     *bool     `json:"http3_enabled"`
	RateLimitRPS     *int      `json:"rate_limit_rps"`
	RateLimitBurst   *int      `json:"rate_limit_burst"`
	CacheEnabled     *bool     `json:"cache_enabled"`
	CacheTTL         *int      `json:"cache_ttl"`
	CacheMaxObjectKB *int      `json:"cache_max_object_kb"`
	AuthEnabled      *bool     `json:"auth_enabled"`
}

func normalizePathPrefix(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", fmt.Errorf("path_prefix required")
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	p = path.Clean(p)
	if p == "." {
		p = "/"
	}
	if strings.ContainsAny(p, "?#") {
		return "", fmt.Errorf("path_prefix must not contain '?' or '#'")
	}
	if len(p) > 512 {
		return "", fmt.Errorf("path_prefix too long")
	}
	return p, nil
}

func (h *LocationHandler) siteID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		BadRequest(c, "invalid site id")
		return uuid.Nil, false
	}
	var site db.Site
	if err := h.DB.Select("id").First(&site, "id = ?", id).Error; err != nil {
		NotFound(c, "site not found")
		return uuid.Nil, false
	}
	return id, true
}

func (h *LocationHandler) clearCache(siteID uuid.UUID) {
	if h.Cache != nil {
		h.Cache.ClearSite(siteID.String())
	}
}

func (h *LocationHandler) List(c *gin.Context) {
	siteID, ok := h.siteID(c)
	if !ok {
		return
	}
	var locs []db.SiteLocation
	if err := h.DB.Where("site_id = ?", siteID).Order("path_prefix ASC").Find(&locs).Error; err != nil {
		InternalError(c, err.Error())
		return
	}
	OK(c, locs)
}

func (h *LocationHandler) Create(c *gin.Context) {
	siteID, ok := h.siteID(c)
	if !ok {
		return
	}
	var req locationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}
	prefix, err := normalizePathPrefix(req.PathPrefix)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	var count int64
	h.DB.Model(&db.SiteLocation{}).Where("site_id = ? AND path_prefix = ?", siteID, prefix).Count(&count)
	if count > 0 {
		BadRequest(c, "a location with this path already exists")
		return
	}
	loc := db.SiteLocation{
		SiteID:           siteID,
		PathPrefix:       prefix,
		Enabled:          true,
		UpstreamURL:      req.UpstreamURL,
		UpstreamHost:     req.UpstreamHost,
		GeoIPEnabled:     req.GeoIPEnabled,
		GeoIPMode:        req.GeoIPMode,
		GeoIPCountries:   toStrArray(req.GeoIPCountries),
		HTTP3Enabled:     req.HTTP3Enabled,
		RateLimitRPS:     req.RateLimitRPS,
		RateLimitBurst:   req.RateLimitBurst,
		CacheEnabled:     req.CacheEnabled,
		CacheTTL:         req.CacheTTL,
		CacheMaxObjectKB: req.CacheMaxObjectKB,
		AuthEnabled:      req.AuthEnabled,
	}
	if req.Enabled != nil {
		loc.Enabled = *req.Enabled
	}
	if err := h.DB.Create(&loc).Error; err != nil {
		BadRequest(c, err.Error())
		return
	}
	h.clearCache(siteID)
	Created(c, loc)
}

func (h *LocationHandler) Update(c *gin.Context) {
	siteID, ok := h.siteID(c)
	if !ok {
		return
	}
	lid, err := uuid.Parse(c.Param("lid"))
	if err != nil {
		BadRequest(c, "invalid location id")
		return
	}
	var loc db.SiteLocation
	if err := h.DB.First(&loc, "id = ? AND site_id = ?", lid, siteID).Error; err != nil {
		NotFound(c, "location not found")
		return
	}
	var req locationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}
	if strings.TrimSpace(req.PathPrefix) != "" {
		prefix, err := normalizePathPrefix(req.PathPrefix)
		if err != nil {
			BadRequest(c, err.Error())
			return
		}
		var count int64
		h.DB.Model(&db.SiteLocation{}).Where("site_id = ? AND path_prefix = ? AND id <> ?", siteID, prefix, lid).Count(&count)
		if count > 0 {
			BadRequest(c, "a location with this path already exists")
			return
		}
		loc.PathPrefix = prefix
	}
	if req.Enabled != nil {
		loc.Enabled = *req.Enabled
	}
	loc.UpstreamURL = req.UpstreamURL
	loc.UpstreamHost = req.UpstreamHost
	loc.GeoIPEnabled = req.GeoIPEnabled
	loc.GeoIPMode = req.GeoIPMode
	loc.GeoIPCountries = toStrArray(req.GeoIPCountries)
	loc.HTTP3Enabled = req.HTTP3Enabled
	loc.RateLimitRPS = req.RateLimitRPS
	loc.RateLimitBurst = req.RateLimitBurst
	loc.CacheEnabled = req.CacheEnabled
	loc.CacheTTL = req.CacheTTL
	loc.CacheMaxObjectKB = req.CacheMaxObjectKB
	loc.AuthEnabled = req.AuthEnabled

	if err := h.DB.Save(&loc).Error; err != nil {
		InternalError(c, err.Error())
		return
	}
	h.clearCache(siteID)
	OK(c, loc)
}

func (h *LocationHandler) Delete(c *gin.Context) {
	siteID, ok := h.siteID(c)
	if !ok {
		return
	}
	lid, err := uuid.Parse(c.Param("lid"))
	if err != nil {
		BadRequest(c, "invalid location id")
		return
	}
	if err := h.DB.Delete(&db.SiteLocation{}, "id = ? AND site_id = ?", lid, siteID).Error; err != nil {
		InternalError(c, err.Error())
		return
	}
	h.clearCache(siteID)
	c.Status(http.StatusNoContent)
}
