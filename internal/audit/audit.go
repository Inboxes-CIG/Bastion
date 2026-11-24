package audit

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/inboxes-cig/bastion/internal/auth"
	"github.com/inboxes-cig/bastion/internal/db"
	"gorm.io/gorm"
)

func actionFor(method string) string {
	switch method {
	case "POST":
		return "create"
	case "PUT", "PATCH":
		return "update"
	case "DELETE":
		return "delete"
	}
	return strings.ToLower(method)
}

func resourceFor(path string) string {
	p := strings.TrimPrefix(path, "/api/")
	p = strings.TrimPrefix(p, "v1/")
	parts := strings.Split(p, "/")
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

// Middleware records mutating API requests into the audit log.
func Middleware(d *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		m := c.Request.Method
		if m == "GET" || m == "HEAD" || m == "OPTIONS" {
			return
		}
		path := c.Request.URL.Path
		if !strings.HasPrefix(path, "/api/") {
			return
		}
		if strings.HasPrefix(path, "/api/v1/auth/login") {
			return
		}
		e := db.AuditLog{
			Action:    actionFor(m),
			Resource:  resourceFor(path),
			Method:    m,
			Path:      path,
			Status:    c.Writer.Status(),
			IP:        c.ClientIP(),
			CreatedAt: time.Now(),
		}
		if cl := auth.ClaimsFromContext(c); cl != nil {
			uid := cl.UserID
			e.UserID = &uid
			if cl.TenantID != nil {
				tid := *cl.TenantID
				e.TenantID = &tid
			}
			var u db.User
			if err := d.Select("email").First(&u, "id = ?", cl.UserID).Error; err == nil {
				e.UserEmail = u.Email
			}
		}
		_ = d.Create(&e).Error
	}
}

// Login records a successful or failed login attempt.
func Login(d *gorm.DB, email, ip string, success bool, userID, tenantID *uuid.UUID) {
	action := "login"
	status := 200
	if !success {
		action = "login_failed"
		status = 401
	}
	_ = d.Create(&db.AuditLog{
		Action:    action,
		Resource:  "auth",
		Method:    "POST",
		Path:      "/api/v1/auth/login",
		Status:    status,
		IP:        ip,
		UserEmail: email,
		UserID:    userID,
		TenantID:  tenantID,
		CreatedAt: time.Now(),
	}).Error
}
