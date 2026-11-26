package crowdsec

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"unicode"

	"github.com/inboxes-cig/bastion/internal/db"
	"gorm.io/gorm"
	"time"
)

type Decision struct {
	ID        int64  `json:"id"`
	Origin    string `json:"origin"`
	Type      string `json:"type"`
	Scope     string `json:"scope"`
	Value     string `json:"value"`
	Duration  string `json:"duration"`
	Scenario  string `json:"scenario"`
	Simulated bool   `json:"simulated"`
}

type Client struct {
	URL     string
	APIKey  string
	http    *http.Client
	mu      sync.RWMutex
	decisions []Decision
	lastSync  time.Time
	lastErr   string
}

func New(url, key string) *Client {
	return &Client{
		URL:    strings.TrimRight(url, "/"),
		APIKey: key,
		http:   &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) Refresh() error {
	req, err := http.NewRequest("GET", c.URL+"/v1/decisions", nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Api-Key", c.APIKey)
	resp, err := c.http.Do(req)
	if err != nil {
		c.mu.Lock()
		c.lastErr = err.Error()
		c.lastSync = time.Now()
		c.mu.Unlock()
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		err := fmt.Errorf("lapi status %d", resp.StatusCode)
		c.mu.Lock()
		c.lastErr = err.Error()
		c.lastSync = time.Now()
		c.mu.Unlock()
		return err
	}
	var ds []Decision
	if err := json.NewDecoder(resp.Body).Decode(&ds); err != nil && err.Error() != "EOF" {
		return err
	}
	c.mu.Lock()
	c.decisions = ds
	c.lastSync = time.Now()
	c.lastErr = ""
	c.mu.Unlock()
	return nil
}

func (c *Client) Run(interval time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	for {
		_ = c.Refresh()
		time.Sleep(interval)
	}
}

func (c *Client) List() []Decision {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]Decision, len(c.decisions))
	copy(out, c.decisions)
	return out
}

func (c *Client) Status() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return map[string]interface{}{
		"enabled":   true,
		"connected": c.lastErr == "",
		"last_sync": c.lastSync,
		"error":     c.lastErr,
		"count":     len(c.decisions),
	}
}

func (c *Client) IsBanned(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, d := range c.decisions {
		if d.Simulated {
			continue
		}
		if d.Type != "" && !strings.EqualFold(d.Type, "ban") {
			continue
		}
		if strings.Contains(d.Value, "/") {
			if _, cidr, err := net.ParseCIDR(d.Value); err == nil && cidr.Contains(ip) {
				return true
			}
			continue
		}
		if v := net.ParseIP(d.Value); v != nil && v.Equal(ip) {
			return true
		}
	}
	return false
}

var durationRe = regexp.MustCompile(`^[0-9]+(s|m|h|d)$`)

func (c *Client) AddBan(value, duration, reason string) error {
	isCIDR := false
	if _, _, err := net.ParseCIDR(value); err == nil {
		isCIDR = true
	} else if net.ParseIP(value) == nil {
		return fmt.Errorf("invalid IP or CIDR")
	}
	args := []string{"decisions", "add"}
	if isCIDR {
		args = append(args, "--range", value)
	} else {
		args = append(args, "--ip", value)
	}
	if duration != "" {
		if !durationRe.MatchString(duration) {
			return fmt.Errorf("invalid duration (use e.g. 1h, 24h, 30m)")
		}
		args = append(args, "--duration", duration)
	}
	if reason != "" {
		reason = sanitizeReason(reason)
		if reason == "" {
			return fmt.Errorf("invalid reason")
		}
		args = append(args, "--reason", reason)
	}
	out, err := exec.Command("cscli", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("cscli: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// sanitizeReason keeps the reason a single, flag-free argument: no control
// characters, no leading dash (which cscli would parse as another flag) and
// capped length.
func sanitizeReason(s string) string {
	var b strings.Builder
	prevSpace := false
	for _, r := range s {
		if unicode.IsControl(r) {
			r = ' '
		}
		if r == ' ' {
			if !prevSpace {
				b.WriteByte(' ')
			}
			prevSpace = true
			continue
		}
		prevSpace = false
		b.WriteRune(r)
	}
	out := strings.TrimSpace(b.String())
	out = strings.TrimLeft(out, "-")
	runes := []rune(out)
	if len(runes) > 200 {
		out = string(runes[:200])
	}
	return out
}

func (c *Client) DeleteDecision(id string) error {
	if !regexp.MustCompile(`^[0-9]+$`).MatchString(id) {
		return fmt.Errorf("invalid decision id")
	}
	out, err := exec.Command("cscli", "decisions", "delete", "--id", id).CombinedOutput()
	if err != nil {
		return fmt.Errorf("cscli: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

type Alert struct {
	ID          int64     `json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	Scenario    string    `json:"scenario"`
	Message     string    `json:"message"`
	Kind        string    `json:"kind"`
	EventsCount int       `json:"events_count"`
	Source      struct {
		IP    string `json:"ip"`
		Value string `json:"value"`
		Scope string `json:"scope"`
	} `json:"source"`
	Decisions []struct {
		Type     string `json:"type"`
		Value    string `json:"value"`
		Duration string `json:"duration"`
	} `json:"decisions"`
}

func (c *Client) Alerts(limit int) ([]Alert, error) {
	if limit <= 0 {
		limit = 50
	}
	out, err := exec.Command("cscli", "alerts", "list", "-o", "json", "--limit", fmt.Sprintf("%d", limit)).Output()
	if err != nil {
		// Surface cscli's stderr: the usual cause is a permission problem
		// (the bastion service user cannot read /etc/crowdsec/config.yaml).
		msg := ""
		if ee, ok := err.(*exec.ExitError); ok {
			msg = strings.TrimSpace(string(ee.Stderr))
		}
		if msg != "" {
			return nil, fmt.Errorf("cscli alerts: %w: %s", err, msg)
		}
		return nil, fmt.Errorf("cscli alerts: %w", err)
	}
	if len(out) == 0 {
		return []Alert{}, nil
	}
	var alerts []Alert
	if err := json.Unmarshal(out, &alerts); err != nil {
		return nil, err
	}
	return alerts, nil
}

func (c *Client) DeleteDecisionsByIP(value string) error {
	value = strings.TrimSpace(value)
	flag := "--ip"
	if strings.Contains(value, "/") {
		if _, _, err := net.ParseCIDR(value); err != nil {
			return fmt.Errorf("invalid CIDR")
		}
		flag = "--range"
	} else if net.ParseIP(value) == nil {
		return fmt.Errorf("invalid IP")
	}
	out, err := exec.Command("cscli", "decisions", "delete", flag, value).CombinedOutput()
	if err != nil {
		return fmt.Errorf("cscli: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// WriteGoodActors writes a CrowdSec whitelist file from the good-actor list.
func WriteGoodActors(d *gorm.DB, path string) error {
	var actors []db.GoodActor
	if err := d.Find(&actors).Error; err != nil {
		return err
	}
	var ips, cidrs []string
	for _, a := range actors {
		v := strings.TrimSpace(a.Value)
		if v == "" {
			continue
		}
		// Only canonical, valid IP/CIDR strings reach the YAML file. Anything
		// else is skipped, which removes the quoting/injection surface
		// entirely (no quotes, newlines or YAML specials can appear).
		if strings.Contains(v, "/") {
			if _, n, err := net.ParseCIDR(v); err == nil {
				cidrs = append(cidrs, n.String())
			}
			continue
		}
		if ip := net.ParseIP(v); ip != nil {
			ips = append(ips, ip.String())
		}
	}
	var b strings.Builder
	b.WriteString("name: waf/good-actors\n")
	b.WriteString("description: \"Bastion good actors (managed via GUI)\"\n")
	b.WriteString("whitelist:\n")
	b.WriteString("  reason: \"waf good actor\"\n")
	if len(ips) > 0 {
		b.WriteString("  ip:\n")
		for _, ip := range ips {
			b.WriteString("    - \"" + ip + "\"\n")
		}
	}
	if len(cidrs) > 0 {
		b.WriteString("  cidr:\n")
		for _, c := range cidrs {
			b.WriteString("    - \"" + c + "\"\n")
		}
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}
