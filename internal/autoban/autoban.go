package autoban

import (
	"sync"
	"time"
)

// Config controls the 404 flood detector.
type Config struct {
	Enabled       bool
	Threshold     int
	WindowSeconds int
}

// Detector counts 404 responses per client IP inside a fixed window and calls
// OnBan once when the threshold is reached. It is safe for concurrent use.
type Detector struct {
	cfg   Config
	onBan func(ip string)

	mu          sync.Mutex
	counts      map[string]*counter
	banned      map[string]time.Time
	lastCleanup time.Time
}

type counter struct {
	count int
	first time.Time
}

func New(cfg Config, onBan func(ip string)) *Detector {
	if cfg.Threshold <= 0 {
		cfg.Threshold = 50
	}
	if cfg.WindowSeconds <= 0 {
		cfg.WindowSeconds = 600
	}
	return &Detector{
		cfg:         cfg,
		onBan:       onBan,
		counts:      map[string]*counter{},
		banned:      map[string]time.Time{},
		lastCleanup: time.Now(),
	}
}

// Observe records one response. Admin requests, globally allow-listed clients
// and anything that is not a 404 are ignored.
func (d *Detector) Observe(ip string, status int, admin, allowed bool) {
	if d == nil || !d.cfg.Enabled || ip == "" || admin || allowed || status != 404 {
		return
	}
	now := time.Now()
	window := time.Duration(d.cfg.WindowSeconds) * time.Second

	d.mu.Lock()
	if _, ok := d.banned[ip]; ok {
		d.mu.Unlock()
		return
	}
	c := d.counts[ip]
	if c == nil || now.Sub(c.first) > window {
		c = &counter{first: now}
		d.counts[ip] = c
	}
	c.count++

	reached := c.count >= d.cfg.Threshold
	if reached {
		d.banned[ip] = now
		delete(d.counts, ip)
	}
	if now.Sub(d.lastCleanup) > window {
		d.cleanupLocked(now, window)
	}
	d.mu.Unlock()

	if reached && d.onBan != nil {
		go d.onBan(ip)
	}
}

// cleanupLocked drops expired counters and forgets bans after 24h so the map
// does not grow without bound. The blocklist entry itself remains until an
// administrator removes it.
func (d *Detector) cleanupLocked(now time.Time, window time.Duration) {
	for ip, c := range d.counts {
		if now.Sub(c.first) > window {
			delete(d.counts, ip)
		}
	}
	for ip, at := range d.banned {
		if now.Sub(at) > 24*time.Hour {
			delete(d.banned, ip)
		}
	}
	d.lastCleanup = now
}
