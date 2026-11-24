package db

import (
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/inboxes-cig/bastion/internal/auth"
	"gorm.io/gorm"
)

// Seed ensures base roles, a default tenant, and a superadmin user exist.
// Returns the superadmin password if newly created (empty otherwise).
func Seed(d *gorm.DB) (string, error) {
	// Roles
	roles := []struct {
		Name        string
		Scope       string
		Permissions string
	}{
		{"superadmin", "system", `["*"]`},
		{"tenant_admin", "tenant", `["tenant:*","site:*","cert:*","user:*","geoip:*","role:read"]`},
		{"editor", "tenant", `["site:*","cert:*","geoip:*"]`},
		{"viewer", "tenant", `["site:read","cert:read","geoip:read"]`},
	}
	roleIDs := map[string]uuid.UUID{}
	for _, r := range roles {
		var existing Role
		err := d.Where("name = ? AND scope = ?", r.Name, r.Scope).First(&existing).Error
		if err == nil {
			roleIDs[r.Name] = existing.ID
			continue
		}
		if err != nil && err != gorm.ErrRecordNotFound {
			return "", err
		}
		rl := Role{Name: r.Name, Scope: r.Scope, Permissions: []byte(r.Permissions)}
		if err := d.Create(&rl).Error; err != nil {
			return "", fmt.Errorf("create role %s: %w", r.Name, err)
		}
		roleIDs[r.Name] = rl.ID
		log.Printf("seeded role: %s", r.Name)
	}

	// Default tenant
	var tenant Tenant
	err := d.Where("slug = ?", "default").First(&tenant).Error
	if err == gorm.ErrRecordNotFound {
		tenant = Tenant{Name: "Default", Slug: "default"}
		if err := d.Create(&tenant).Error; err != nil {
			return "", fmt.Errorf("create default tenant: %w", err)
		}
		log.Printf("seeded default tenant: %s", tenant.ID)
	} else if err != nil {
		return "", err
	}

	// Superadmin user
	var superadmin User
	err = d.Where("email = ?", "admin@bastion.local").First(&superadmin).Error
	if err == gorm.ErrRecordNotFound {
		pw := generateInitialPassword()
		hash, err := auth.HashPassword(pw)
		if err != nil {
			return "", err
		}
		u := User{
			TenantID:     nil,
			Email:        "admin@bastion.local",
			PasswordHash: hash,
			RoleID:       roleIDs["superadmin"],
			Active:       true,
		}
		if err := d.Create(&u).Error; err != nil {
			return "", fmt.Errorf("create superadmin: %w", err)
		}
		log.Printf("seeded superadmin: %s", u.Email)
		return pw, nil
	} else if err != nil {
		return "", err
	}
	return "", nil
}
