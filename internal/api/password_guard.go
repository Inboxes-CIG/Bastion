package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/inboxes-cig/bastion/internal/auth"
	"github.com/inboxes-cig/bastion/internal/db"
	"gorm.io/gorm"
)

// PasswordChangeGuard blocks all protected endpoints until a user with the
// must_change_password flag has set a new password.
func PasswordChangeGuard(d *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		cl := auth.ClaimsFromContext(c)
		if cl == nil {
			c.Next()
			return
		}
		// API keys are an independent credential; the password gate does not
		// apply to scripted access.
		if c.GetString("auth_method") == "api_key" {
			c.Next()
			return
		}
		path := c.Request.URL.Path
		if strings.HasSuffix(path, "/auth/me") ||
			strings.HasSuffix(path, "/auth/change-password") ||
			strings.HasSuffix(path, "/auth/logout") {
			c.Next()
			return
		}
		var user db.User
		if err := d.Select("must_change_password").First(&user, "id = ?", cl.UserID).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
			return
		}
		if user.MustChangePassword {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "password change required",
				"code":  "password_change_required",
			})
			return
		}
		c.Next()
	}
}
