package api

import (
	"github.com/gin-gonic/gin"
	"github.com/inboxes-cig/bastion/internal/cache"
)

type CacheHandler struct {
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
	if req.SiteID != "" {
		h.Cache.ClearSite(req.SiteID)
	} else {
		h.Cache.Clear()
	}
	OK(c, gin.H{"cleared": true, "site_id": req.SiteID})
}
