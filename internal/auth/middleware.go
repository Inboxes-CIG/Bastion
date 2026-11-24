package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const claimsKey = "claims"

func AuthMiddleware(jwtSecret string) gin.HandlerFunc {
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
		claims, err := ValidateToken(jwtSecret, parts[1])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}
		c.Set(claimsKey, claims)
		c.Next()
	}
}

func ClaimsFromContext(c *gin.Context) *Claims {
	v, ok := c.Get(claimsKey)
	if !ok {
		return nil
	}
	cl, _ := v.(*Claims)
	return cl
}

// RequireRole checks that the user has one of the given role names.
func RequireRole(roles ...string) gin.HandlerFunc {
	allowed := map[string]bool{}
	for _, r := range roles {
		allowed[r] = true
	}
	return func(c *gin.Context) {
		cl := ClaimsFromContext(c)
		if cl == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		if !allowed[cl.RoleName] {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		c.Next()
	}
}

// TenantScope ensures a non-superadmin user is scoped to their tenant.
// Sets tenant_id in context for handlers.
func TenantScope() gin.HandlerFunc {
	return func(c *gin.Context) {
		cl := ClaimsFromContext(c)
		if cl == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		if cl.TenantID != nil {
			c.Set("tenant_id", *cl.TenantID)
		}
		c.Next()
	}
}
