package api

import (
	"github.com/gin-gonic/gin"
	"github.com/inboxes-cig/bastion/internal/auth"
	"github.com/inboxes-cig/bastion/internal/db"
	"gorm.io/gorm"
)

type AuditHandler struct {
	DB *gorm.DB
}

func (h *AuditHandler) List(c *gin.Context) {
	q := h.DB.Order("created_at DESC").Limit(200)
	if a := c.Query("action"); a != "" {
		q = q.Where("action = ?", a)
	}
	if s := c.Query("q"); s != "" {
		like := "%" + s + "%"
		q = q.Where("user_email ILIKE ? OR path ILIKE ? OR resource ILIKE ? OR details ILIKE ? OR resource_id ILIKE ?", like, like, like, like, like)
	}
	if cl := auth.ClaimsFromContext(c); cl != nil && cl.RoleName != "superadmin" && cl.TenantID != nil {
		q = q.Where("tenant_id = ?", *cl.TenantID)
	}
	var logs []db.AuditLog
	if err := q.Find(&logs).Error; err != nil {
		InternalError(c, err.Error())
		return
	}
	OK(c, logs)
}
