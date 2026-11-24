package geoip

import (
	"fmt"
	"net"
	"sync"

	"github.com/oschwald/geoip2-golang"
	"gorm.io/gorm"
	"github.com/inboxes-cig/bastion/internal/db"
)

type Lookup struct {
	mu   sync.RWMutex
	db   *geoip2.Reader
	path string
}

func NewLookup(path string) *Lookup {
	return &Lookup{path: path}
}

func (l *Lookup) Load() error {
	reader, err := geoip2.Open(l.path)
	if err != nil {
		return err
	}
	l.mu.Lock()
	if l.db != nil {
		l.db.Close()
	}
	l.db = reader
	l.mu.Unlock()
	return nil
}

func (l *Lookup) CountryCode(ipStr string) (string, error) {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return "", fmt.Errorf("invalid ip")
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	if l.db == nil {
		return "", fmt.Errorf("geoip db not loaded")
	}
	record, err := l.db.Country(ip)
	if err != nil {
		return "", err
	}
	return record.Country.IsoCode, nil
}

func (l *Lookup) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.db != nil {
		l.db.Close()
		l.db = nil
	}
}

// EvaluateRules checks geoip rules for a given IP and site/tenant.
// Returns true if request should be blocked.
func EvaluateRules(d *gorm.DB, lookup *Lookup, ipStr string, tenantID, siteID string) bool {
	cc, err := lookup.CountryCode(ipStr)
	if err != nil || cc == "" {
		return false // fail open
	}
	// find applicable rules: site-specific first, then tenant-wide
	var rules []db.GeoIPRule
	q := d.Where("tenant_id = ?", tenantID)
	if siteID != "" {
		q = q.Where("site_id = ? OR site_id IS NULL", siteID)
	} else {
		q = q.Where("site_id IS NULL")
	}
	if err := q.Order("priority DESC").Find(&rules).Error; err != nil {
		return false
	}
	for _, r := range rules {
		if r.CountryCode == cc || r.CountryCode == "*" {
			return r.Action == "block"
		}
	}
	return false // default allow
}
