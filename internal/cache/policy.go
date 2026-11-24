package cache

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// TTLFromHeaders derives the freshness lifetime in seconds from an upstream
// response and reports whether a shared cache may store it.
//
// Supported directives: no-store, no-cache, private, s-maxage, max-age and
// Expires. s-maxage wins over max-age (shared cache semantics). An upstream
// Age header is subtracted so we do not extend a cached response's lifetime.
func TTLFromHeaders(h http.Header, defaultTTL int) (int, bool) {
	cc := strings.ToLower(strings.Join(h.Values("Cache-Control"), ","))
	directive := func(name string) (string, bool) {
		for _, part := range strings.Split(cc, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			k, v, _ := strings.Cut(part, "=")
			if strings.TrimSpace(k) == name {
				return strings.Trim(strings.TrimSpace(v), `"`), true
			}
		}
		return "", false
	}
	if _, ok := directive("no-store"); ok {
		return 0, false
	}
	if _, ok := directive("no-cache"); ok {
		return 0, false
	}
	if _, ok := directive("private"); ok {
		return 0, false
	}

	ttl := defaultTTL
	if v, ok := directive("s-maxage"); ok {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return 0, false
		}
		ttl = n
	} else if v, ok := directive("max-age"); ok {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return 0, false
		}
		ttl = n
	} else if exp := strings.TrimSpace(h.Get("Expires")); exp != "" {
		t, err := http.ParseTime(exp)
		if err != nil {
			return 0, false
		}
		ttl = int(time.Until(t).Seconds())
		if ttl <= 0 {
			return 0, false
		}
	}
	if ttl <= 0 {
		return 0, false
	}
	if age, err := strconv.Atoi(strings.TrimSpace(h.Get("Age"))); err == nil && age > 0 {
		ttl -= age
		if ttl <= 0 {
			return 0, false
		}
	}
	return ttl, true
}

// VaryAllowsCaching reports whether the response's Vary header is covered by
// our cache key (host, request URI, Accept-Encoding). Any other variation
// would need to be part of the key, so such responses are not stored.
func VaryAllowsCaching(h http.Header) bool {
	for _, v := range h.Values("Vary") {
		for _, f := range strings.Split(v, ",") {
			switch strings.ToLower(strings.TrimSpace(f)) {
			case "", "accept-encoding":
				continue
			default:
				return false
			}
		}
	}
	return true
}
