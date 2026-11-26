// Package adminaccess enforces the separate access policy for the admin GUI:
// its own allow/deny IP list and an optional GeoIP policy. It is independent
// of the per-site IP lists and the per-site GeoIP rules.
package adminaccess

import (
	"net"
	"strings"
	"sync"
	"time"

	"github.com/inboxes-cig/bastion/internal/db"
	"github.com/inboxes-cig/bastion/internal/geoip"
	"gorm.io/gorm"
)

type Cache struct {
	db *gorm.DB

	mu           sync.RWMutex
	allowExact   map[string]bool
	denyExact    map[string]bool
	allowCIDR    []*net.IPNet
	denyCIDR     []*net.IPNet
	hasAllow     bool
	geoEnabled   bool
	geoMode      string
	geoCountries map[string]bool
}

func New(db *gorm.DB) *Cache {
	return &Cache{
		db:           db,
		allowExact:   map[string]bool{},
		denyExact:    map[string]bool{},
		geoCountries: map[string]bool{},
		geoMode:      "block",
	}
}

func (c *Cache) Reload() error {
	var rules []db.AdminIPRule
	if err := c.db.Find(&rules).Error; err != nil {
		return err
	}
	allowExact := map[string]bool{}
	denyExact := map[string]bool{}
	var allowCIDR, denyCIDR []*net.IPNet
	for _, r := range rules {
		v := strings.TrimSpace(r.Value)
		if strings.Contains(v, "/") {
			if _, n, err := net.ParseCIDR(v); err == nil {
				if r.Kind == "allow" {
					allowCIDR = append(allowCIDR, n)
				} else {
					denyCIDR = append(denyCIDR, n)
				}
			}
			continue
		}
		ip := net.ParseIP(v)
		if ip == nil {
			continue
		}
		if r.Kind == "allow" {
			allowExact[ip.String()] = true
		} else {
			denyExact[ip.String()] = true
		}
	}
	hasAllow := len(allowExact) > 0 || len(allowCIDR) > 0

	geoEnabled := false
	geoMode := "block"
	countries := map[string]bool{}
	var policy db.AdminGeoPolicy
	if err := c.db.Order("created_at ASC").Limit(1).Find(&policy).Error; err == nil {
		geoEnabled = policy.Enabled
		if policy.Mode == "allow" || policy.Mode == "block" {
			geoMode = policy.Mode
		}
		for _, cc := range policy.Countries {
			cc = strings.ToUpper(strings.TrimSpace(cc))
			if cc != "" {
				countries[cc] = true
			}
		}
	}

	c.mu.Lock()
	c.allowExact = allowExact
	c.denyExact = denyExact
	c.allowCIDR = allowCIDR
	c.denyCIDR = denyCIDR
	c.hasAllow = hasAllow
	c.geoEnabled = geoEnabled
	c.geoMode = geoMode
	c.geoCountries = countries
	c.mu.Unlock()
	return nil
}

func (c *Cache) Run(interval time.Duration) {
	if interval <= 0 {
		interval = 60 * time.Second
	}
	for {
		time.Sleep(interval)
		_ = c.Reload()
	}
}

func match(ip net.IP, exact map[string]bool, cidrs []*net.IPNet) bool {
	if exact[ip.String()] {
		return true
	}
	for _, n := range cidrs {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// Check reports whether the admin GUI may be served to ipStr. The second
// return value is the block reason for the access log (admin_ip/admin_geo).
// Loopback is always allowed so server-local health checks and tooling work
// even if an allowlist is misconfigured.
func (c *Cache) Check(ipStr string, geo *geoip.Lookup) (bool, string) {
	ip := net.ParseIP(strings.TrimSpace(strings.Trim(ipStr, "[]")))
	if ip == nil {
		return false, "admin_ip"
	}
	if ip.IsLoopback() {
		return true, ""
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if match(ip, c.denyExact, c.denyCIDR) {
		return false, "admin_ip"
	}
	if c.hasAllow && !match(ip, c.allowExact, c.allowCIDR) {
		return false, "admin_ip"
	}
	if c.geoEnabled && len(c.geoCountries) > 0 && geo != nil {
		cc, err := geo.CountryCode(ipStr)
		if err == nil && cc != "" {
			inList := c.geoCountries[strings.ToUpper(cc)]
			if c.geoMode == "block" && inList {
				return false, "admin_geo"
			}
			if c.geoMode == "allow" && !inList {
				return false, "admin_geo"
			}
		}
	}
	return true, ""
}
