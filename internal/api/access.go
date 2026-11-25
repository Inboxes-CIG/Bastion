package api

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/inboxes-cig/bastion/internal/auth"
	"github.com/inboxes-cig/bastion/internal/db"
	"gorm.io/gorm"
)

// isSuperadmin reports whether the current user may access all tenants.
func isSuperadmin(c *gin.Context) bool {
	cl := auth.ClaimsFromContext(c)
	return cl != nil && cl.RoleName == "superadmin"
}

// canAccessTenant reports whether the current user may access an object with
// the given tenant. Nil tenant IDs are system objects (superadmin only).
func canAccessTenant(c *gin.Context, tenantID *uuid.UUID) bool {
	cl := auth.ClaimsFromContext(c)
	if cl == nil {
		return false
	}
	if cl.RoleName == "superadmin" {
		return true
	}
	if cl.TenantID == nil || tenantID == nil {
		return false
	}
	return *cl.TenantID == *tenantID
}

// tenantScoped applies the tenant filter to a query. Non-superadmins without
// a tenant get an empty result set.
func tenantScoped(c *gin.Context, q *gorm.DB, column string) *gorm.DB {
	cl := auth.ClaimsFromContext(c)
	if cl != nil && cl.RoleName == "superadmin" {
		return q
	}
	if cl == nil || cl.TenantID == nil {
		return q.Where("1 = 0")
	}
	return q.Where(column+" = ?", *cl.TenantID)
}

// canAccessSite loads the site's tenant and checks access.
func canAccessSite(c *gin.Context, d *gorm.DB, siteID uuid.UUID) bool {
	var site db.Site
	if err := d.Select("tenant_id").First(&site, "id = ?", siteID).Error; err != nil {
		return false
	}
	return canAccessTenant(c, &site.TenantID)
}

// requireSuperadmin aborts with 403 unless the current user is a superadmin.
func requireSuperadmin(c *gin.Context) bool {
	if !isSuperadmin(c) {
		Forbidden(c, "forbidden")
		return false
	}
	return true
}

// requireAnyRole aborts with 403 unless the current user has one of the roles.
func requireAnyRole(c *gin.Context, roles ...string) bool {
	cl := auth.ClaimsFromContext(c)
	if cl == nil {
		Unauthorized(c, "unauthorized")
		return false
	}
	for _, r := range roles {
		if cl.RoleName == r {
			return true
		}
	}
	Forbidden(c, "forbidden")
	return false
}

// reauth checks the acting user's password for sensitive operations.
func reauth(c *gin.Context, d *gorm.DB, password string) bool {
	cl := auth.ClaimsFromContext(c)
	if cl == nil || password == "" {
		return false
	}
	var u db.User
	if err := d.Select("password_hash").First(&u, "id = ?", cl.UserID).Error; err != nil {
		return false
	}
	return auth.CheckPassword(u.PasswordHash, password)
}

// tenantDomains returns the primary domains of all sites in the current
// tenant. The second return value is false when the caller is not allowed to
// see any logs. A nil map means "no filter" (superadmin).
func tenantDomains(c *gin.Context, d *gorm.DB) (map[string]bool, bool) {
	cl := auth.ClaimsFromContext(c)
	if cl == nil {
		return nil, false
	}
	if cl.RoleName == "superadmin" {
		return nil, true
	}
	if cl.TenantID == nil {
		return map[string]bool{}, true
	}
	var sites []db.Site
	d.Select("domain").Where("tenant_id = ?", *cl.TenantID).Find(&sites)
	m := make(map[string]bool, len(sites))
	for _, s := range sites {
		m[s.Domain] = true
	}
	return m, true
}
