package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/inboxes-cig/bastion/internal/auth"
	"github.com/inboxes-cig/bastion/internal/db"
	"gorm.io/gorm"
)

type GeoIPHandler struct {
	DB *gorm.DB
}

func (h *GeoIPHandler) ListRules(c *gin.Context) {
	var rules []db.GeoIPRule
	q := tenantScoped(c, h.DB, "tenant_id")
	if siteID := c.Query("site_id"); siteID != "" {
		sid, err := uuid.Parse(siteID)
		if err != nil {
			BadRequest(c, "invalid site_id")
			return
		}
		if !canAccessSite(c, h.DB, sid) {
			NotFound(c, "site not found")
			return
		}
		q = q.Where("site_id = ?", sid)
	}
	if err := q.Find(&rules).Error; err != nil {
		InternalError(c, err.Error())
		return
	}
	OK(c, rules)
}

type geoipRuleRequest struct {
	TenantID    string `json:"tenant_id"`
	SiteID      string `json:"site_id"`
	CountryCode string `json:"country_code" binding:"required"`
	Action      string `json:"action" binding:"required"`
	Priority    int    `json:"priority"`
}

func (h *GeoIPHandler) CreateRule(c *gin.Context) {
	var req geoipRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}
	cl := auth.ClaimsFromContext(c)
	if cl == nil {
		Unauthorized(c, "unauthorized")
		return
	}
	tenantID := req.TenantID
	if cl.RoleName != "superadmin" {
		if cl.TenantID == nil {
			Forbidden(c, "forbidden")
			return
		}
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
	var siteID *uuid.UUID
	if req.SiteID != "" {
		sid, err := uuid.Parse(req.SiteID)
		if err != nil {
			BadRequest(c, "invalid site_id")
			return
		}
		var site db.Site
		if err := h.DB.Select("tenant_id").First(&site, "id = ?", sid).Error; err != nil || site.TenantID != tid {
			NotFound(c, "site not found")
			return
		}
		siteID = &sid
	}
	rule := db.GeoIPRule{
		TenantID:    tid,
		SiteID:      siteID,
		CountryCode: req.CountryCode,
		Action:      req.Action,
		Priority:    req.Priority,
	}
	if err := h.DB.Create(&rule).Error; err != nil {
		BadRequest(c, err.Error())
		return
	}
	Created(c, rule)
}

func (h *GeoIPHandler) DeleteRule(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		BadRequest(c, "invalid id")
		return
	}
	var rule db.GeoIPRule
	if err := h.DB.First(&rule, "id = ?", id).Error; err != nil {
		NotFound(c, "rule not found")
		return
	}
	if !canAccessTenant(c, &rule.TenantID) {
		NotFound(c, "rule not found")
		return
	}
	if err := h.DB.Delete(&db.GeoIPRule{}, "id = ?", id).Error; err != nil {
		InternalError(c, err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}
