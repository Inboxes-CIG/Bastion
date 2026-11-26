package db

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Tenant struct {
	ID                   uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Name                 string         `gorm:"not null" json:"name"`
	Slug                 string         `gorm:"uniqueIndex;not null" json:"slug"`
	AllowedUpstreamHosts StringArray    `gorm:"type:jsonb" json:"allowed_upstream_hosts"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
	DeletedAt            gorm.DeletedAt `gorm:"index" json:"-"`
}

type Role struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Name        string    `gorm:"not null" json:"name"`
	Scope       string    `gorm:"not null" json:"scope"`
	Permissions []byte    `gorm:"type:jsonb" json:"permissions"`
	CreatedAt   time.Time `json:"created_at"`
}

type User struct {
	ID                 uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID           *uuid.UUID     `gorm:"type:uuid;index" json:"tenant_id"`
	Email              string         `gorm:"uniqueIndex;not null" json:"email"`
	PasswordHash       string         `gorm:"not null" json:"-"`
	RoleID             uuid.UUID      `gorm:"type:uuid;not null" json:"role_id"`
	Active             bool           `gorm:"default:true" json:"active"`
	MustChangePassword bool           `gorm:"default:false" json:"must_change_password"`
	MFAEnabled         bool           `gorm:"default:false" json:"mfa_enabled"`
	MFASecret          string         `json:"-"`
	LastLogin          *time.Time     `json:"last_login"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	DeletedAt          gorm.DeletedAt `gorm:"index" json:"-"`
}

// SiteTemplate defines reusable site configuration defaults.
type SiteTemplate struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID    uuid.UUID `gorm:"type:uuid;index;not null" json:"tenant_id"`
	Name        string    `gorm:"not null" json:"name"`
	Description string    `json:"description"`

	// Inheritable config fields
	UpstreamURL                *string           `json:"upstream_url"`
	UpstreamHost               *string           `json:"upstream_host"`
	UpstreamInsecureSkipVerify *bool             `json:"upstream_insecure_skip_verify"`
	UpstreamAllowPrivate       bool              `json:"-"`
	GeoIPEnabled               *bool             `json:"geoip_enabled"`
	GeoIPMode                  *string           `json:"geoip_mode"`
	GeoIPCountries             StringArray       `gorm:"type:jsonb" json:"geoip_countries"`
	HTTP2Enabled               *bool             `json:"http2_enabled"`
	HTTP3Enabled               *bool             `json:"http3_enabled"`
	ForceHTTPS                 *bool             `json:"force_https"`
	HSTSEnabled                *bool             `json:"hsts_enabled"`
	HSTSMaxAge                 *int              `json:"hsts_max_age"`
	WebSocketEnabled           *bool             `json:"websocket_enabled"`
	CompressionEnabled         *bool             `json:"compression_enabled"`
	RateLimitRPS               *int              `json:"rate_limit_rps"`
	RateLimitBurst             *int              `json:"rate_limit_burst"`
	MaxBodySizeMB              *int              `json:"max_body_size_mb"`
	CacheEnabled               *bool             `json:"cache_enabled"`
	CacheTTL                   *int              `json:"cache_ttl"`
	CacheMaxObjectKB           *int              `json:"cache_max_object_kb"`
	AuthEnabled                *bool             `json:"auth_enabled"`
	CustomHeaders              map[string]string `gorm:"serializer:json;type:jsonb" json:"custom_headers"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Site represents a proxied site/domain.
type Site struct {
	ID         uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID   uuid.UUID  `gorm:"type:uuid;index;not null" json:"tenant_id"`
	TemplateID *uuid.UUID `gorm:"type:uuid;index" json:"template_id"`

	// Primary domain + extra domains (aliases)
	Domain       string      `gorm:"not null" json:"domain"`
	ExtraDomains StringArray `gorm:"type:jsonb" json:"extra_domains"`

	// Override fields (nil = inherit from template)
	UpstreamURL                *string           `json:"upstream_url"`
	UpstreamHost               *string           `json:"upstream_host"`
	UpstreamInsecureSkipVerify *bool             `json:"upstream_insecure_skip_verify"`
	UpstreamAllowPrivate       bool              `json:"-"`
	GeoIPEnabled               *bool             `json:"geoip_enabled"`
	GeoIPMode                  *string           `json:"geoip_mode"`
	GeoIPCountries             StringArray       `gorm:"type:jsonb" json:"geoip_countries"`
	HTTP2Enabled               *bool             `json:"http2_enabled"`
	HTTP3Enabled               *bool             `json:"http3_enabled"`
	ForceHTTPS                 *bool             `json:"force_https"`
	HSTSEnabled                *bool             `json:"hsts_enabled"`
	HSTSMaxAge                 *int              `json:"hsts_max_age"`
	WebSocketEnabled           *bool             `json:"websocket_enabled"`
	CompressionEnabled         *bool             `json:"compression_enabled"`
	RateLimitRPS               *int              `json:"rate_limit_rps"`
	RateLimitBurst             *int              `json:"rate_limit_burst"`
	MaxBodySizeMB              *int              `json:"max_body_size_mb"`
	CacheEnabled               *bool             `json:"cache_enabled"`
	CacheTTL                   *int              `json:"cache_ttl"`
	CacheMaxObjectKB           *int              `json:"cache_max_object_kb"`
	AuthEnabled                *bool             `json:"auth_enabled"`
	CustomHeaders              map[string]string `gorm:"serializer:json;type:jsonb" json:"custom_headers"`

	// Effective (resolved) fields - computed, not stored
	// Cert reference
	CertID *uuid.UUID `gorm:"type:uuid" json:"cert_id"`

	// Status
	Enabled bool `gorm:"not null" json:"enabled"`

	// Path-based overrides (loaded on demand)
	Locations []SiteLocation `gorm:"foreignKey:SiteID" json:"locations,omitempty"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// SiteLocation overrides a site's configuration for a path prefix. Nil
// pointers inherit the site/template value. The longest matching prefix wins.
type SiteLocation struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	SiteID     uuid.UUID `gorm:"type:uuid;index;not null" json:"site_id"`
	PathPrefix string    `gorm:"not null" json:"path_prefix"`
	Enabled    bool      `gorm:"not null" json:"enabled"`

	UpstreamURL                *string     `json:"upstream_url"`
	UpstreamHost               *string     `json:"upstream_host"`
	UpstreamInsecureSkipVerify *bool       `json:"upstream_insecure_skip_verify"`
	UpstreamAllowPrivate       bool        `json:"-"`
	GeoIPEnabled               *bool       `json:"geoip_enabled"`
	GeoIPMode                  *string     `json:"geoip_mode"`
	GeoIPCountries             StringArray `gorm:"type:jsonb" json:"geoip_countries"`
	HTTP3Enabled               *bool       `json:"http3_enabled"`
	RateLimitRPS               *int        `json:"rate_limit_rps"`
	RateLimitBurst             *int        `json:"rate_limit_burst"`
	CacheEnabled               *bool       `json:"cache_enabled"`
	CacheTTL                   *int        `json:"cache_ttl"`
	CacheMaxObjectKB           *int        `json:"cache_max_object_kb"`
	AuthEnabled                *bool       `json:"auth_enabled"`
	AuthUsersMode              string      `gorm:"not null;default:inherit" json:"auth_users_mode"`
	IPRulesMode                string      `gorm:"not null;default:inherit" json:"ip_rules_mode"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

type Certificate struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID  uuid.UUID  `gorm:"type:uuid;index;not null" json:"tenant_id"`
	Domain    string     `gorm:"not null" json:"domain"`
	CertPEM   string     `gorm:"type:text" json:"-"`
	KeyPEM    string     `gorm:"type:text" json:"-"`
	Issuer    string     `json:"issuer"`
	NotBefore *time.Time `json:"not_before"`
	NotAfter  *time.Time `json:"not_after"`
	AutoRenew bool       `gorm:"default:false" json:"auto_renew"`
	Source    string     `gorm:"default:manual" json:"source"`
	Email     string     `json:"email"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type IPListEntry struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Kind      string     `gorm:"not null" json:"kind"`
	Value     string     `gorm:"not null" json:"value"`
	Note      string     `json:"note"`
	ExpiresAt *time.Time `gorm:"index" json:"expires_at"`
	Source    string     `gorm:"default:manual" json:"source"`
	CreatedAt time.Time  `json:"created_at"`
}

// BanStrike counts how often the 404 auto-ban fired for an IP. It drives the
// escalation tiers (short ban -> long ban -> permanent) and survives restarts.
type BanStrike struct {
	ID      uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	IP      string    `gorm:"uniqueIndex;not null" json:"ip"`
	Strikes int       `json:"strikes"`
	LastAt  time.Time `json:"last_at"`
}

// AdminIPRule restricts access to the admin GUI itself (independent of the
// per-site IP lists). Loopback is always allowed by the enforcement layer.
type AdminIPRule struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Kind      string    `gorm:"not null" json:"kind"` // allow | deny
	Value     string    `gorm:"not null" json:"value"`
	Note      string    `json:"note"`
	CreatedAt time.Time `json:"created_at"`
}

// AdminGeoPolicy is the singleton GeoIP policy for the admin GUI.
type AdminGeoPolicy struct {
	ID        uuid.UUID   `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Enabled   bool        `json:"enabled"`
	Mode      string      `json:"mode"` // allow | block
	Countries StringArray `gorm:"type:jsonb" json:"countries"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

// APIKey authenticates scripts against /api/v1. Only the SHA-256 hash of the
// key is stored; the plaintext is shown once at creation.
type APIKey struct {
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID       uuid.UUID  `gorm:"type:uuid;index;not null" json:"user_id"`
	Name         string     `gorm:"not null" json:"name"`
	KeyHash      string     `gorm:"uniqueIndex;not null" json:"-"`
	Prefix       string     `json:"prefix"`
	RateLimitRPM *int       `json:"rate_limit_rpm"`
	ExpiresAt    *time.Time `json:"expires_at"`
	LastUsedAt   *time.Time `json:"last_used_at"`
	CreatedAt    time.Time  `json:"created_at"`
	RevokedAt    *time.Time `json:"revoked_at"`
}

// APIIPRule is an entry of the global API IP allowlist. An empty list means
// only loopback may use API keys.
type APIIPRule struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Value     string    `gorm:"not null" json:"value"`
	Note      string    `json:"note"`
	CreatedAt time.Time `json:"created_at"`
}

// APISettings is the singleton with API-wide defaults.
type APISettings struct {
	ID                  uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	DefaultRateLimitRPM int       `json:"default_rate_limit_rpm"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type GeoIPRule struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID    uuid.UUID  `gorm:"type:uuid;index;not null" json:"tenant_id"`
	SiteID      *uuid.UUID `gorm:"type:uuid;index" json:"site_id"`
	CountryCode string     `gorm:"not null" json:"country_code"`
	Action      string     `gorm:"not null" json:"action"`
	Priority    int        `gorm:"default:0" json:"priority"`
	CreatedAt   time.Time  `json:"created_at"`
}

type SiteAuthUser struct {
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	SiteID       uuid.UUID  `gorm:"type:uuid;index;not null" json:"site_id"`
	LocationID   *uuid.UUID `gorm:"type:uuid;index" json:"location_id"`
	Username     string     `gorm:"not null" json:"username"`
	PasswordHash string     `gorm:"not null" json:"-"`
	Active       bool       `gorm:"default:true" json:"active"`
	CreatedAt    time.Time  `json:"created_at"`
}

type SiteIPRule struct {
	ID         uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	SiteID     uuid.UUID  `gorm:"type:uuid;index;not null" json:"site_id"`
	LocationID *uuid.UUID `gorm:"type:uuid;index" json:"location_id"`
	Kind       string     `gorm:"not null" json:"kind"`
	Value      string     `gorm:"not null" json:"value"`
	Note       string     `json:"note"`
	CreatedAt  time.Time  `json:"created_at"`
}

type AuditLog struct {
	ID         uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID   *uuid.UUID `gorm:"type:uuid;index" json:"tenant_id"`
	UserID     *uuid.UUID `gorm:"type:uuid;index" json:"user_id"`
	UserEmail  string     `json:"user_email"`
	Action     string     `gorm:"index" json:"action"`
	Resource   string     `json:"resource"`
	ResourceID string     `json:"resource_id"`
	Method     string     `json:"method"`
	Path       string     `json:"path"`
	Status     int        `json:"status"`
	IP         string     `json:"ip"`
	Details    string     `json:"details"`
	CreatedAt  time.Time  `gorm:"index" json:"created_at"`
}

type GoodActor struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Value     string    `gorm:"not null" json:"value"`
	Note      string    `json:"note"`
	CreatedAt time.Time `json:"created_at"`
}

// RefreshToken tracks issued refresh tokens for rotation and revocation.
type RefreshToken struct {
	ID         uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID     uuid.UUID  `gorm:"type:uuid;index;not null" json:"user_id"`
	JTI        string     `gorm:"uniqueIndex;not null" json:"jti"`
	ExpiresAt  time.Time  `gorm:"index;not null" json:"expires_at"`
	RevokedAt  *time.Time `json:"revoked_at"`
	ReplacedBy string     `json:"replaced_by"`
	CreatedAt  time.Time  `json:"created_at"`
}

func AutoMigrate(d *gorm.DB) error {
	return d.AutoMigrate(&Tenant{}, &Role{}, &User{}, &SiteTemplate{}, &Site{}, &SiteLocation{}, &Certificate{}, &GeoIPRule{}, &IPListEntry{}, &AuditLog{}, &GoodActor{}, &SiteAuthUser{}, &SiteIPRule{}, &RefreshToken{}, &BanStrike{}, &AdminIPRule{}, &AdminGeoPolicy{}, &APIKey{}, &APIIPRule{}, &APISettings{})
}
