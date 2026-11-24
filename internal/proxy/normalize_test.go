package proxy

import "testing"

func TestNormalizeLocationPrefix(t *testing.T) {
	tests := map[string]string{
		"/api":      "/api",
		"api/":      "/api",
		"/static/":  "/static",
		"/":         "/",
		"  /x/y/  ": "/x/y",
	}
	for in, want := range tests {
		got, err := normalizeLocationPrefix(in)
		if err != nil {
			t.Fatalf("normalizeLocationPrefix(%q): %v", in, err)
		}
		if got != want {
			t.Fatalf("normalizeLocationPrefix(%q) = %q, want %q", in, got, want)
		}
	}
}
