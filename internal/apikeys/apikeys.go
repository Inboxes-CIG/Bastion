// Package apikeys handles API keys for scripted access: key generation and
// hashing, the global API IP allowlist (empty = loopback only) and per-key
// rate limiting.
package apikeys

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/inboxes-cig/bastion/internal/db"
	"gorm.io/gorm"
)

const keyPrefix = "bsk_"

// Hash returns the SHA-256 hex digest of a plaintext API key.
func Hash(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

// Generate creates a new key. The plaintext is only returned here and never
// stored; the display prefix is the leading "bsk_<id>" part.
func Generate() (plain, hash, displayPrefix string) {
	b := make([]byte, 20)
	_, _ = rand.Read(b)
	id := strings.ReplaceAll(uuid.NewString(), "-", "")[:8]
	plain = keyPrefix + id + "_" + hex.EncodeToString(b)
	return plain, Hash(plain), plain[:len(keyPrefix)+8]
}

type Service struct {
	DB *gorm.DB

	mu         sync.RWMutex
	allowExact map[string]bool
	allowCIDR  []*net.IPNet
	hasAllow   bool
	defaultRPM int

	rlMu    sync.Mutex
	windows map[uuid.UUID]*rlWindow

	usageMu sync.Mutex
	lastHit map[uuid.UUID]time.Time
}

type rlWindow struct {
	start time.Time
	count int
}

func New(db *gorm.DB) *Service {
	return &Service{
		DB:         db,
		allowExact: map[string]bool{},
		windows:    map[uuid.UUID]*rlWindow{},
		lastHit:    map[uuid.UUID]time.Time{},
	}
}

func (s *Service) Reload() error {
	var rules []db.APIIPRule
	if err := s.DB.Find(&rules).Error; err != nil {
		return err
	}
	exact := map[string]bool{}
	var cidrs []*net.IPNet
	for _, r := range rules {
		v := strings.TrimSpace(r.Value)
		if strings.Contains(v, "/") {
			if _, n, err := net.ParseCIDR(v); err == nil {
				cidrs = append(cidrs, n)
			}
			continue
		}
		if ip := net.ParseIP(v); ip != nil {
			exact[ip.String()] = true
		}
	}
	rpm := 0
	var st db.APISettings
	if err := s.DB.Order("created_at ASC").First(&st).Error; err == nil {
		if st.DefaultRateLimitRPM > 0 {
			rpm = st.DefaultRateLimitRPM
		}
	}

	s.mu.Lock()
	s.allowExact = exact
	s.allowCIDR = cidrs
	s.hasAllow = len(exact) > 0 || len(cidrs) > 0
	s.defaultRPM = rpm
	s.mu.Unlock()
	return nil
}

func (s *Service) Run(interval time.Duration) {
	if interval <= 0 {
		interval = 60 * time.Second
	}
	for {
		time.Sleep(interval)
		_ = s.Reload()
	}
}

// IPAllowed reports whether an API-key request from ipStr may proceed.
// Loopback is always allowed; an empty allowlist blocks every other source.
func (s *Service) IPAllowed(ipStr string) bool {
	ip := net.ParseIP(strings.TrimSpace(strings.Trim(ipStr, "[]")))
	if ip == nil {
		return false
	}
	if ip.IsLoopback() {
		return true
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.hasAllow {
		return false
	}
	if s.allowExact[ip.String()] {
		return true
	}
	for _, n := range s.allowCIDR {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// DefaultRPM returns the configured default rate limit (0 = disabled).
func (s *Service) DefaultRPM() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.defaultRPM
}

// AllowRequest applies the per-key rate limit. rpm <= 0 means "use default";
// if both are 0 the limit is disabled.
func (s *Service) AllowRequest(keyID uuid.UUID, rpm int) bool {
	if rpm <= 0 {
		rpm = s.DefaultRPM()
	}
	if rpm <= 0 {
		return true
	}
	now := time.Now()
	s.rlMu.Lock()
	defer s.rlMu.Unlock()
	w := s.windows[keyID]
	if w == nil || now.Sub(w.start) >= time.Minute {
		w = &rlWindow{start: now}
		s.windows[keyID] = w
	}
	if w.count >= rpm {
		return false
	}
	w.count++
	if len(s.windows) > 4096 {
		for id, ww := range s.windows {
			if now.Sub(ww.start) >= 2*time.Minute {
				delete(s.windows, id)
			}
		}
	}
	return true
}

// MarkUsed throttles last_used_at updates to at most once per minute.
func (s *Service) MarkUsed(keyID uuid.UUID) {
	s.usageMu.Lock()
	if time.Since(s.lastHit[keyID]) < time.Minute {
		s.usageMu.Unlock()
		return
	}
	s.lastHit[keyID] = time.Now()
	s.usageMu.Unlock()
	now := time.Now()
	s.DB.Model(&db.APIKey{}).Where("id = ?", keyID).Update("last_used_at", &now)
}
