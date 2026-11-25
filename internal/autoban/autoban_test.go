package autoban

import (
	"sync"
	"testing"
	"time"
)

func TestDetectorThreshold(t *testing.T) {
	var mu sync.Mutex
	var banned []string
	d := New(Config{Enabled: true, Threshold: 3, WindowSeconds: 60}, func(ip string) {
		mu.Lock()
		banned = append(banned, ip)
		mu.Unlock()
	})

	d.Observe("203.0.113.10", 404, false, false)
	d.Observe("203.0.113.10", 404, false, false)
	if len(banned) != 0 {
		t.Fatalf("banned too early: %v", banned)
	}
	d.Observe("203.0.113.10", 404, false, false)
	// onBan runs in a goroutine; give it a moment.
	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	got := append([]string(nil), banned...)
	mu.Unlock()
	if len(got) != 1 || got[0] != "203.0.113.10" {
		t.Fatalf("banned = %v, want [203.0.113.10]", got)
	}

	// Further 404s must not trigger a second ban.
	d.Observe("203.0.113.10", 404, false, false)
	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if len(banned) != 1 {
		t.Fatalf("duplicate ban: %v", banned)
	}
}

func TestDetectorIgnoresNon404AndExemptClients(t *testing.T) {
	var mu sync.Mutex
	var banned []string
	d := New(Config{Enabled: true, Threshold: 1, WindowSeconds: 60}, func(ip string) {
		mu.Lock()
		banned = append(banned, ip)
		mu.Unlock()
	})

	d.Observe("198.51.100.1", 200, false, false)
	d.Observe("198.51.100.2", 404, true, false)  // admin
	d.Observe("198.51.100.3", 404, false, true)  // allow-listed
	d.Observe("", 404, false, false)             // no IP
	d.Observe("198.51.100.4", 403, false, false) // blocked
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(banned) != 0 {
		t.Fatalf("unexpected bans: %v", banned)
	}
}

func TestDetectorWindowExpires(t *testing.T) {
	var mu sync.Mutex
	var banned []string
	d := New(Config{Enabled: true, Threshold: 2, WindowSeconds: 1}, func(ip string) {
		mu.Lock()
		banned = append(banned, ip)
		mu.Unlock()
	})

	d.Observe("192.0.2.50", 404, false, false)
	time.Sleep(1100 * time.Millisecond)
	d.Observe("192.0.2.50", 404, false, false)
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(banned) != 0 {
		t.Fatalf("window did not reset: %v", banned)
	}
}

func TestDetectorDisabled(t *testing.T) {
	d := New(Config{Enabled: false, Threshold: 1, WindowSeconds: 60}, func(ip string) {
		t.Fatalf("unexpected ban for %s", ip)
	})
	d.Observe("192.0.2.99", 404, false, false)
}

func TestForgetAllowsNextTier(t *testing.T) {
	var mu sync.Mutex
	hits := 0
	d := New(Config{Enabled: true, Threshold: 1, WindowSeconds: 60}, func(ip string) {
		mu.Lock()
		hits++
		mu.Unlock()
	})
	d.Observe("192.0.2.10", 404, false, false)
	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	first := hits
	mu.Unlock()
	if first != 1 {
		t.Fatalf("expected first trigger, got %d", hits)
	}
	// While suppressed, further floods must not re-trigger.
	d.Observe("192.0.2.10", 404, false, false)
	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	suppressed := hits
	mu.Unlock()
	if suppressed != 1 {
		t.Fatalf("suppression lost: %d", suppressed)
	}
	// After Forget (timed ban expired) the next flood triggers the next tier.
	d.Forget("192.0.2.10")
	d.Observe("192.0.2.10", 404, false, false)
	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if hits != 2 {
		t.Fatalf("Forget did not re-arm detector: %d", hits)
	}
}
