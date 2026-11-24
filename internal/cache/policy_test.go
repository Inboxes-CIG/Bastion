package cache

import (
	"net/http"
	"testing"
	"time"
)

func TestTTLFromHeaders(t *testing.T) {
	future := time.Now().Add(90 * time.Second).UTC().Format(http.TimeFormat)
	past := time.Now().Add(-90 * time.Second).UTC().Format(http.TimeFormat)

	tests := []struct {
		name   string
		header http.Header
		def    int
		want   int
		ok     bool
	}{
		{"no headers uses default", http.Header{}, 60, 60, true},
		{"max-age wins", http.Header{"Cache-Control": {"max-age=120"}}, 60, 120, true},
		{"s-maxage wins over max-age", http.Header{"Cache-Control": {"max-age=10, s-maxage=300"}}, 60, 300, true},
		{"no-store", http.Header{"Cache-Control": {"no-store"}}, 60, 0, false},
		{"no-cache", http.Header{"Cache-Control": {"no-cache"}}, 60, 0, false},
		{"private", http.Header{"Cache-Control": {"private, max-age=60"}}, 60, 0, false},
		{"max-age zero", http.Header{"Cache-Control": {"max-age=0"}}, 60, 0, false},
		{"max-age invalid", http.Header{"Cache-Control": {"max-age=abc"}}, 60, 0, false},
		{"expires future", http.Header{"Expires": {future}}, 60, 0, true},
		{"expires past", http.Header{"Expires": {past}}, 60, 0, false},
		{"expires invalid", http.Header{"Expires": {"0"}}, 60, 0, false},
		{"age subtracts", http.Header{"Cache-Control": {"max-age=60"}, "Age": {"55"}}, 60, 5, true},
		{"age exceeds ttl", http.Header{"Cache-Control": {"max-age=60"}, "Age": {"60"}}, 60, 0, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := TTLFromHeaders(tc.header, tc.def)
			if ok != tc.ok {
				t.Fatalf("cacheable = %v, want %v", ok, tc.ok)
			}
			if !ok {
				return
			}
			if tc.want == 0 {
				if got <= 0 {
					t.Fatalf("ttl = %d, want > 0", got)
				}
				return
			}
			if got != tc.want {
				t.Fatalf("ttl = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestVaryAllowsCaching(t *testing.T) {
	tests := []struct {
		name  string
		value []string
		want  bool
	}{
		{"no vary", nil, true},
		{"accept-encoding", []string{"Accept-Encoding"}, true},
		{"accept-encoding plus spaces", []string{"Accept-Encoding, Accept-Encoding"}, true},
		{"cookie", []string{"Cookie"}, false},
		{"accept-language", []string{"Accept-Language"}, false},
		{"star", []string{"*"}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := http.Header{}
			for _, v := range tc.value {
				h.Add("Vary", v)
			}
			if got := VaryAllowsCaching(h); got != tc.want {
				t.Fatalf("VaryAllowsCaching = %v, want %v", got, tc.want)
			}
		})
	}
}
