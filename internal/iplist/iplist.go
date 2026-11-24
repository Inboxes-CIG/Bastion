package iplist

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/inboxes-cig/bastion/internal/db"
	"gorm.io/gorm"
)

type List struct {
	db  *gorm.DB
	mu  sync.RWMutex
	allowExact map[string]bool
	allowCIDR  []*net.IPNet
	denyExact  map[string]bool
	denyCIDR   []*net.IPNet
}

func New(db *gorm.DB) *List {
	return &List{
		db:         db,
		allowExact: map[string]bool{},
		denyExact:  map[string]bool{},
	}
}

func (l *List) Reload() error {
	var entries []db.IPListEntry
	if err := l.db.Find(&entries).Error; err != nil {
		return err
	}
	allowExact := map[string]bool{}
	denyExact := map[string]bool{}
	var allowCIDR, denyCIDR []*net.IPNet
	for _, e := range entries {
		v := strings.TrimSpace(e.Value)
		if strings.Contains(v, "/") {
			if _, cidr, err := net.ParseCIDR(v); err == nil {
				if e.Kind == "allow" {
					allowCIDR = append(allowCIDR, cidr)
				} else {
					denyCIDR = append(denyCIDR, cidr)
				}
			}
			continue
		}
		ip := net.ParseIP(v)
		if ip == nil {
			continue
		}
		if e.Kind == "allow" {
			allowExact[ip.String()] = true
		} else {
			denyExact[ip.String()] = true
		}
	}
	l.mu.Lock()
	l.allowExact = allowExact
	l.allowCIDR = allowCIDR
	l.denyExact = denyExact
	l.denyCIDR = denyCIDR
	l.mu.Unlock()
	return nil
}

func (l *List) Run(interval time.Duration) {
	if interval <= 0 {
		interval = 60 * time.Second
	}
	for {
		time.Sleep(interval)
		_ = l.Reload()
	}
}

func matchIP(ip net.IP, exact map[string]bool, cidrs []*net.IPNet) bool {
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

func (l *List) IsAllowed(ipStr string) bool {
	ip := net.ParseIP(strings.TrimSpace(ipStr))
	if ip == nil {
		return false
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	return matchIP(ip, l.allowExact, l.allowCIDR)
}

func (l *List) IsDenied(ipStr string) bool {
	ip := net.ParseIP(strings.TrimSpace(ipStr))
	if ip == nil {
		return false
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	return matchIP(ip, l.denyExact, l.denyCIDR)
}

func Validate(value string) error {
	v := strings.TrimSpace(value)
	if v == "" {
		return fmt.Errorf("empty value")
	}
	if strings.Contains(v, "/") {
		if _, _, err := net.ParseCIDR(v); err != nil {
			return fmt.Errorf("invalid CIDR")
		}
		return nil
	}
	if net.ParseIP(v) == nil {
		return fmt.Errorf("invalid IP or CIDR")
	}
	return nil
}
