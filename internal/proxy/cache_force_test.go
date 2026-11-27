package proxy

import (
	"net/http"
	"testing"
	"time"
)

func TestCacheTTLDecision(t *testing.T) {
	noCache := http.Header{}
	noCache.Set("Cache-Control", "no-cache, must-revalidate")
	noCache.Set("Expires", time.Now().Format(http.TimeFormat))

	if _, ok := cacheTTLDecision(false, noCache, 60); ok {
		t.Fatal("respect mode must not cache no-cache")
	}
	if ttl, ok := cacheTTLDecision(true, noCache, 60); !ok || ttl != 60 {
		t.Fatalf("force mode: ttl=%d ok=%v", ttl, ok)
	}

	maxAge := http.Header{}
	maxAge.Set("Cache-Control", "max-age=120")
	if ttl, ok := cacheTTLDecision(false, maxAge, 60); !ok || ttl != 120 {
		t.Fatalf("respect max-age: ttl=%d ok=%v", ttl, ok)
	}
	if ttl, ok := cacheTTLDecision(true, maxAge, 60); !ok || ttl != 60 {
		t.Fatalf("force must use site ttl: ttl=%d ok=%v", ttl, ok)
	}

	if _, ok := cacheTTLDecision(true, noCache, 0); ok {
		t.Fatal("force without a TTL must not cache")
	}
}
