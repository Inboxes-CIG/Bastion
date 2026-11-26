package api

import (
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/inboxes-cig/bastion/internal/apikeys"
	"github.com/inboxes-cig/bastion/internal/auth"
	"github.com/inboxes-cig/bastion/internal/db"
)

// clientIPFromProxy returns the real client IP for API-key requests. Only
// X-Real-IP (set by our own reverse proxy for admin hosts) and the direct peer
// address are trusted; client-supplied X-Forwarded-For is deliberately
// ignored so the API IP allowlist cannot be bypassed by header spoofing.
func clientIPFromProxy(c *gin.Context) string {
	if v := strings.TrimSpace(c.GetHeader("X-Real-IP")); v != "" {
		return v
	}
	if host, _, err := net.SplitHostPort(c.Request.RemoteAddr); err == nil {
		return host
	}
	return c.Request.RemoteAddr
}

// apiKeyAuthMiddleware accepts either a JWT access token (three dot-separated
// segments) or an API key. API keys are checked against the global API IP
// allowlist (empty = loopback only), their own expiry and the rate limit.
func apiKeyAuthMiddleware(jwtSecret string, keys *apikeys.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			return
		}
		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header"})
			return
		}
		token := parts[1]

		if strings.Count(token, ".") == 2 {
			claims, err := auth.ValidateAccessToken(jwtSecret, token)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
				return
			}
			c.Set("claims", claims)
			c.Next()
			return
		}

		if keys == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		ip := clientIPFromProxy(c)
		if !keys.IPAllowed(ip) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "api access denied from this ip"})
			return
		}
		var k db.APIKey
		if err := keys.DB.Where("key_hash = ? AND revoked_at IS NULL", apikeys.Hash(token)).First(&k).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid api key"})
			return
		}
		if k.ExpiresAt != nil && k.ExpiresAt.Before(time.Now()) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "api key expired"})
			return
		}
		var user db.User
		if err := keys.DB.First(&user, "id = ? AND active = true", k.UserID).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "user not found or inactive"})
			return
		}
		var role db.Role
		if err := keys.DB.First(&role, "id = ?", user.RoleID).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "role not found"})
			return
		}
		rpm := 0
		if k.RateLimitRPM != nil {
			rpm = *k.RateLimitRPM
		}
		if !keys.AllowRequest(k.ID, rpm) {
			c.Header("Retry-After", "60")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "api key rate limit exceeded"})
			return
		}
		keys.MarkUsed(k.ID)

		c.Set("claims", &auth.Claims{
			UserID:    user.ID,
			TenantID:  user.TenantID,
			RoleID:    role.ID,
			RoleName:  role.Name,
			TokenType: "api_key",
		})
		c.Set("auth_method", "api_key")
		c.Next()
	}
}
