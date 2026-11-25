package api

import (
	"github.com/google/uuid"
	"github.com/inboxes-cig/bastion/internal/db"
	"github.com/inboxes-cig/bastion/internal/netguard"
	"gorm.io/gorm"
)

// validateUpstream checks a user-controlled upstream URL against the global
// and tenant-specific allowlists. The returned bool reports whether a private
// address was explicitly allowed, so the proxy can skip its literal check.
func validateUpstream(d *gorm.DB, tenantID uuid.UUID, raw string, allowPrivate bool, global []string) (bool, error) {
	if raw == "" {
		return false, nil
	}
	allowed := append([]string{}, global...)
	var tenant db.Tenant
	if err := d.Select("allowed_upstream_hosts").First(&tenant, "id = ?", tenantID).Error; err == nil {
		allowed = append(allowed, tenant.AllowedUpstreamHosts...)
	}
	return netguard.ValidateURL(raw, allowPrivate, allowed)
}

func templateBelongsToTenant(d *gorm.DB, templateID *uuid.UUID, tenantID uuid.UUID) bool {
	if templateID == nil {
		return true
	}
	var t db.SiteTemplate
	if err := d.Select("tenant_id").First(&t, "id = ?", *templateID).Error; err != nil {
		return false
	}
	return t.TenantID == tenantID
}

func certBelongsToTenant(d *gorm.DB, certID *uuid.UUID, tenantID uuid.UUID) bool {
	if certID == nil {
		return true
	}
	var cert db.Certificate
	if err := d.Select("tenant_id").First(&cert, "id = ?", *certID).Error; err != nil {
		return false
	}
	return cert.TenantID == tenantID
}
