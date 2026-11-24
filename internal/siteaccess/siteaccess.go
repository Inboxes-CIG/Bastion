package siteaccess

import (
	"net"
	"strings"
	"sync"
	"time"

	"github.com/inboxes-cig/bastion/internal/db"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// siteData holds one access scope: either the site itself (LocationID nil) or
// one of its path locations.
type siteData struct {
	loaded     time.Time
	users      map[string]string
	allowExact map[string]bool
	allowCIDR  []*net.IPNet
	denyExact  map[string]bool
	denyCIDR   []*net.IPNet
	locations  map[string]*siteData
}

func newData() *siteData {
	return &siteData{
		loaded:     time.Now(),
		users:      map[string]string{},
		allowExact: map[string]bool{},
		denyExact:  map[string]bool{},
		locations:  map[string]*siteData{},
	}
}

type Cache struct {
	db    *gorm.DB
	mu    sync.RWMutex
	sites map[string]*siteData
}

func New(db *gorm.DB) *Cache {
	return &Cache{db: db, sites: map[string]*siteData{}}
}

func (c *Cache) Invalidate(siteID string) {
	c.mu.Lock()
	delete(c.sites, siteID)
	c.mu.Unlock()
}

func (c *Cache) Get(siteID string) *siteData {
	c.mu.RLock()
	d := c.sites[siteID]
	c.mu.RUnlock()
	if d != nil && time.Since(d.loaded) < 30*time.Second {
		return d
	}
	d = c.load(siteID)
	c.mu.Lock()
	c.sites[siteID] = d
	c.mu.Unlock()
	return d
}

// GetLocation returns the access data for one location. Unknown locations
// yield an empty scope (no users, no rules), which is the correct behavior
// when a location uses "own" mode but has no entries yet.
func (c *Cache) GetLocation(siteID, locationID string) *siteData {
	d := c.Get(siteID)
	if d != nil {
		if l := d.locations[locationID]; l != nil {
			return l
		}
	}
	return newData()
}

func (c *Cache) load(siteID string) *siteData {
	d := newData()

	var users []db.SiteAuthUser
	c.db.Where("site_id = ? AND active = true", siteID).Find(&users)
	for _, u := range users {
		target := d
		if u.LocationID != nil {
			target = d.location(u.LocationID.String())
		}
		target.users[u.Username] = u.PasswordHash
	}

	var rules []db.SiteIPRule
	c.db.Where("site_id = ?", siteID).Find(&rules)
	for _, r := range rules {
		target := d
		if r.LocationID != nil {
			target = d.location(r.LocationID.String())
		}
		addRule(target, r)
	}
	return d
}

func (d *siteData) location(id string) *siteData {
	l := d.locations[id]
	if l == nil {
		l = newData()
		d.locations[id] = l
	}
	return l
}

func addRule(d *siteData, r db.SiteIPRule) {
	v := strings.TrimSpace(r.Value)
	if strings.Contains(v, "/") {
		if _, cidr, err := net.ParseCIDR(v); err == nil {
			if r.Kind == "allow" {
				d.allowCIDR = append(d.allowCIDR, cidr)
			} else {
				d.denyCIDR = append(d.denyCIDR, cidr)
			}
		}
		return
	}
	ip := net.ParseIP(v)
	if ip == nil {
		return
	}
	if r.Kind == "allow" {
		d.allowExact[ip.String()] = true
	} else {
		d.denyExact[ip.String()] = true
	}
}

func match(ip net.IP, exact map[string]bool, cidrs []*net.IPNet) bool {
	if ip == nil {
		return false
	}
	if exact[ip.String()] {
		return true
	}
	for _, c := range cidrs {
		if c.Contains(ip) {
			return true
		}
	}
	return false
}

func (d *siteData) Allowed(ipStr string) bool {
	return match(net.ParseIP(strings.TrimSpace(ipStr)), d.allowExact, d.allowCIDR)
}

func (d *siteData) Denied(ipStr string) bool {
	return match(net.ParseIP(strings.TrimSpace(ipStr)), d.denyExact, d.denyCIDR)
}

func (d *siteData) HasUsers() bool {
	return len(d.users) > 0
}

func (d *siteData) Validate(user, pass string) bool {
	hash, ok := d.users[user]
	if !ok {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pass)) == nil
}

// Lockout tracks failed auth attempts per site+location+IP and locks for 5
// minutes after 3 failures.
type Lockout struct {
	mu sync.Mutex
	m  map[string]*lockEntry
}

type lockEntry struct {
	fails int
	until time.Time
}

func NewLockout() *Lockout {
	return &Lockout{m: map[string]*lockEntry{}}
}

func (l *Lockout) Locked(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	e := l.m[key]
	if e == nil {
		return false
	}
	if e.until.IsZero() {
		return false
	}
	if time.Now().Before(e.until) {
		return true
	}
	delete(l.m, key)
	return false
}

func (l *Lockout) Fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	e := l.m[key]
	if e == nil {
		e = &lockEntry{}
		l.m[key] = e
	}
	if !e.until.IsZero() && time.Now().After(e.until) {
		e.fails = 0
		e.until = time.Time{}
	}
	e.fails++
	if e.fails >= 3 {
		e.until = time.Now().Add(5 * time.Minute)
	}
}

func (l *Lockout) Reset(key string) {
	l.mu.Lock()
	delete(l.m, key)
	l.mu.Unlock()
}
