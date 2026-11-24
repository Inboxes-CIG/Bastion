package cache

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Config struct {
	Enabled     bool `json:"enabled" yaml:"enabled"`
	TTLSeconds  int  `json:"ttl_seconds" yaml:"ttl_seconds"`
	MaxEntries  int  `json:"max_entries" yaml:"max_entries"`
	MaxObjectKB int  `json:"max_object_kb" yaml:"max_object_kb"`
	MaxTotalMB  int  `json:"max_total_mb" yaml:"max_total_mb"`
}

func (c *Config) normalize() {
	if c.TTLSeconds <= 0 {
		c.TTLSeconds = 60
	}
	if c.MaxEntries <= 0 {
		c.MaxEntries = 500
	}
	if c.MaxObjectKB <= 0 {
		c.MaxObjectKB = 256
	}
	if c.MaxTotalMB <= 0 {
		c.MaxTotalMB = 64
	}
}

type entry struct {
	SiteID  string
	Status  int
	Header  http.Header
	Body    []byte
	Expires time.Time
	Created time.Time
}

type Stats struct {
	Enabled bool   `json:"enabled"`
	Entries int    `json:"entries"`
	SizeBytes int64 `json:"size_bytes"`
	Hits    int64  `json:"hits"`
	Misses  int64  `json:"misses"`
	Stores  int64  `json:"stores"`
	Config  Config `json:"config"`
}

type Cache struct {
	mu      sync.Mutex
	cfg     Config
	path    string
	entries map[string]*entry
	size    int64
	hits    int64
	misses  int64
	stores  int64
}

func New(cfg Config, persistPath string) *Cache {
	cfg.normalize()
	if persistPath != "" {
		if b, err := os.ReadFile(persistPath); err == nil {
			var pc Config
			if json.Unmarshal(b, &pc) == nil {
				pc.normalize()
				cfg = pc
			}
		}
	}
	return &Cache{cfg: cfg, path: persistPath, entries: map[string]*entry{}}
}

func (c *Cache) Config() Config {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cfg
}

func (c *Cache) UpdateConfig(cfg Config) error {
	cfg.normalize()
	c.mu.Lock()
	c.cfg = cfg
	c.entries = map[string]*entry{}
	c.size = 0
	c.mu.Unlock()
	return c.persist()
}

func (c *Cache) persist() error {
	if c.path == "" {
		return nil
	}
	c.mu.Lock()
	cfg := c.cfg
	c.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(c.path), 0o750); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp := c.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o640); err != nil {
		return err
	}
	return os.Rename(tmp, c.path)
}

func (c *Cache) Get(key string) (*entry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.cfg.Enabled {
		c.misses++
		return nil, false
	}
	e, ok := c.entries[key]
	if !ok {
		c.misses++
		return nil, false
	}
	if time.Now().After(e.Expires) {
		delete(c.entries, key)
		c.size -= int64(len(e.Body))
		c.misses++
		return nil, false
	}
	c.hits++
	return e, true
}

func (c *Cache) MaxObjectBytes() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cfg.MaxObjectKB * 1024
}

func (c *Cache) Set(key, siteID string, status int, h http.Header, body []byte, ttlSeconds, maxObjectKB int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.cfg.Enabled {
		return
	}
	maxObj := maxObjectKB
	if maxObj <= 0 {
		maxObj = c.cfg.MaxObjectKB
	}
	if len(body) > maxObj*1024 {
		return
	}
	ttl := ttlSeconds
	if ttl <= 0 {
		ttl = c.cfg.TTLSeconds
	}
	maxBytes := int64(c.cfg.MaxTotalMB) * 1024 * 1024
	// evict expired first
	now := time.Now()
	for k, e := range c.entries {
		if now.After(e.Expires) {
			delete(c.entries, k)
			c.size -= int64(len(e.Body))
		}
	}
	// evict oldest until it fits
	for (len(c.entries) >= c.cfg.MaxEntries || c.size+int64(len(body)) > maxBytes) && len(c.entries) > 0 {
		c.evictOldestLocked()
	}
	c.entries[key] = &entry{SiteID: siteID, Status: status, Header: h.Clone(), Body: body, Created: now, Expires: now.Add(time.Duration(ttl) * time.Second)}
	c.size += int64(len(body))
	c.stores++
}

func (c *Cache) evictOldestLocked() {
	var oldestKey string
	var oldest time.Time
	first := true
	for k, e := range c.entries {
		if first || e.Created.Before(oldest) {
			oldest = e.Created
			oldestKey = k
			first = false
		}
	}
	if oldestKey != "" {
		c.size -= int64(len(c.entries[oldestKey].Body))
		delete(c.entries, oldestKey)
	}
}

func (c *Cache) ClearSite(siteID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, e := range c.entries {
		if e.SiteID == siteID {
			c.size -= int64(len(e.Body))
			delete(c.entries, k)
		}
	}
}

func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = map[string]*entry{}
	c.size = 0
	c.hits = 0
	c.misses = 0
	c.stores = 0
}

func (c *Cache) Stats() Stats {
	c.mu.Lock()
	defer c.mu.Unlock()
	return Stats{
		Enabled:   c.cfg.Enabled,
		Entries:   len(c.entries),
		SizeBytes: c.size,
		Hits:      c.hits,
		Misses:    c.misses,
		Stores:    c.stores,
		Config:    c.cfg,
	}
}
