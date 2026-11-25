package db

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/inboxes-cig/bastion/internal/auth"
	"gorm.io/gorm"
)

// InitialAdminPath is the one-time file written by the installer. It contains
// `email=...` and `password=...` lines. The file is deleted after the initial
// superadmin has been created or found.
var InitialAdminPath = "/etc/bastion/initial-admin"

type SeedResult struct {
	Email    string
	Password string
}

type initialAdmin struct {
	Email    string
	Password string
}

func readInitialAdmin(path string) (initialAdmin, bool) {
	f, err := os.Open(path)
	if err != nil {
		return initialAdmin{}, false
	}
	defer f.Close()

	var out initialAdmin
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(k) {
		case "email":
			out.Email = strings.TrimSpace(v)
		case "password":
			out.Password = strings.TrimRight(v, "\r")
		}
	}
	return out, out.Email != ""
}

// Seed ensures base roles, a default tenant, and a superadmin user exist.
// If the installer wrote an initial-admin file, that account is created (or
// restored) instead of the legacy admin@bastion.local fallback.
func Seed(d *gorm.DB) (SeedResult, error) {
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
			return SeedResult{}, err
		}
		rl := Role{Name: r.Name, Scope: r.Scope, Permissions: []byte(r.Permissions)}
		if err := d.Create(&rl).Error; err != nil {
			return SeedResult{}, fmt.Errorf("create role %s: %w", r.Name, err)
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
			return SeedResult{}, fmt.Errorf("create default tenant: %w", err)
		}
		log.Printf("seeded default tenant: %s", tenant.ID)
	} else if err != nil {
		return SeedResult{}, err
	}

	// Installer-provided initial admin
	if init, ok := readInitialAdmin(InitialAdminPath); ok {
		email := strings.TrimSpace(init.Email)
		password := init.Password
		if password == "" {
			password = generateInitialPassword()
		}

		var existing User
		err := d.Unscoped().Where("email = ?", email).First(&existing).Error
		switch {
		case err == gorm.ErrRecordNotFound:
			hash, herr := auth.HashPassword(password)
			if herr != nil {
				return SeedResult{}, herr
			}
			u := User{
				TenantID:          nil,
				Email:             email,
				PasswordHash:      hash,
				RoleID:            roleIDs["superadmin"],
				Active:            true,
				MustChangePassword: true,
			}
			if cerr := d.Create(&u).Error; cerr != nil {
				return SeedResult{}, fmt.Errorf("create superadmin %s: %w", email, cerr)
			}
			log.Printf("seeded superadmin: %s", email)
			_ = os.Remove(InitialAdminPath)
			return SeedResult{Email: email, Password: password}, nil

		case err != nil:
			return SeedResult{}, err

		default:
			if existing.DeletedAt.Valid {
				hash, herr := auth.HashPassword(password)
				if herr != nil {
					return SeedResult{}, herr
				}
				if uerr := d.Unscoped().Model(&existing).Updates(map[string]interface{}{
					"deleted_at":           nil,
					"password_hash":        hash,
					"role_id":              roleIDs["superadmin"],
					"active":               true,
					"must_change_password": true,
				}).Error; uerr != nil {
					return SeedResult{}, fmt.Errorf("restore superadmin %s: %w", email, uerr)
				}
				log.Printf("restored superadmin: %s", email)
				_ = os.Remove(InitialAdminPath)
				return SeedResult{Email: email, Password: password}, nil
			}
			// Existing active account: never reset its password from a one-time file.
			_ = os.Remove(InitialAdminPath)
			log.Printf("initial admin %s already exists; ignoring initial-admin file", email)
			return SeedResult{}, nil
		}
	}

	// Legacy fallback for installs that do not use an initial-admin file.
	// Only create the default account when the database has no users at all;
	// otherwise every restart after a custom initial admin would create a
	// second superadmin.
	var userCount int64
	if err := d.Model(&User{}).Count(&userCount).Error; err != nil {
		return SeedResult{}, err
	}
	if userCount > 0 {
		return SeedResult{}, nil
	}

	var superadmin User
	err = d.Where("email = ?", "admin@bastion.local").First(&superadmin).Error
	if err == gorm.ErrRecordNotFound {
		pw := generateInitialPassword()
		hash, herr := auth.HashPassword(pw)
		if herr != nil {
			return SeedResult{}, herr
		}
			u := User{
				TenantID:          nil,
				Email:             "admin@bastion.local",
				PasswordHash:      hash,
				RoleID:            roleIDs["superadmin"],
				Active:            true,
				MustChangePassword: true,
			}
		if cerr := d.Create(&u).Error; cerr != nil {
			return SeedResult{}, fmt.Errorf("create superadmin: %w", cerr)
		}
		log.Printf("seeded superadmin: %s", u.Email)
		return SeedResult{Email: u.Email, Password: pw}, nil
	} else if err != nil {
		return SeedResult{}, err
	}
	return SeedResult{}, nil
}
