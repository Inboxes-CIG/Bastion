package api

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/inboxes-cig/bastion/internal/cache"
	"github.com/inboxes-cig/bastion/internal/db"
	"gorm.io/gorm"
)

type CacheHandler struct {
	DB    *gorm.DB
	Cache *cache.Cache
}

func (h *CacheHandler) Get(c *gin.Context) {
	if h.Cache == nil {
		OK(c, gin.H{"enabled": false})
		return
	}
	OK(c, h.Cache.Stats())
}

func (h *CacheHandler) Update(c *gin.Context) {
	if h.Cache == nil {
		BadRequest(c, "cache not enabled")
		return
	}
	var cfg cache.Config
	if err := c.ShouldBindJSON(&cfg); err != nil {
		BadRequest(c, err.Error())
		return
	}
	if err := h.Cache.UpdateConfig(cfg); err != nil {
		InternalError(c, err.Error())
		return
	}
	OK(c, h.Cache.Stats())
}

func (h *CacheHandler) Clear(c *gin.Context) {
	if h.Cache == nil {
		BadRequest(c, "cache not enabled")
		return
	}
	var req struct {
		SiteID string `json:"site_id"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.SiteID == "" {
		req.SiteID = c.Query("site_id")
	}
	if req.SiteID == "" {
		if !requireSuperadmin(c) {
			return
		}
		h.Cache.Clear()
		OK(c, gin.H{"cleared": true})
		return
	}
	sid, err := uuid.Parse(req.SiteID)
	if err != nil {
		BadRequest(c, "invalid site_id")
		return
	}
	var site db.Site
	if err := h.DB.Select("tenant_id").First(&site, "id = ?", sid).Error; err != nil {
		NotFound(c, "site not found")
		return
	}
	if !canAccessTenant(c, &site.TenantID) {
		NotFound(c, "site not found")
		return
	}
	h.Cache.ClearSite(sid.String())
	OK(c, gin.H{"cleared": true, "site_id": sid.String()})
}
