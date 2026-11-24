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
	cl := auth.ClaimsFromContext(c)
	var rules []db.GeoIPRule
	q := h.DB
	if cl != nil && cl.RoleName != "superadmin" && cl.TenantID != nil {
		q = q.Where("tenant_id = ?", *cl.TenantID)
	}
	if siteID := c.Query("site_id"); siteID != "" {
		q = q.Where("site_id = ?", siteID)
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
	rule := db.GeoIPRule{
		TenantID:    tid,
		CountryCode: req.CountryCode,
		Action:      req.Action,
		Priority:    req.Priority,
	}
	if req.SiteID != "" {
		sid, err := uuid.Parse(req.SiteID)
		if err == nil {
			rule.SiteID = &sid
		}
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
	if err := h.DB.Delete(&db.GeoIPRule{}, "id = ?", id).Error; err != nil {
		InternalError(c, err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}
