package httpheaders

import (
	"net/http"
	"testing"
)

func TestSanitize(t *testing.T) {
	in := map[string]string{
		" x-frame-options ": " SAMEORIGIN ",
		"X-Robots-Tag":      "noindex",
	}
	out, err := Sanitize(in)
	if err != nil {
		t.Fatal(err)
	}
	if out["X-Frame-Options"] != "SAMEORIGIN" {
		t.Fatalf("canonicalization failed: %#v", out)
	}
	if out["X-Robots-Tag"] != "noindex" {
		t.Fatalf("missing value: %#v", out)
	}
}

func TestSanitizeRejects(t *testing.T) {
	for _, tc := range []map[string]string{
		{"Bad Header": "x"},
		{"X-Test": "line\r\ninjection"},
		{"Content-Length": "10"},
		{"Transfer-Encoding": "chunked"},
		{"Connection": "close"},
		{"Content-Encoding": "br"},
	} {
		if _, err := Sanitize(tc); err == nil {
			t.Fatalf("expected error for %#v", tc)
		}
	}
}

func TestApply(t *testing.T) {
	h := http.Header{}
	h.Set("X-Frame-Options", "DENY")
	Apply(h, map[string]string{"X-Frame-Options": "SAMEORIGIN", "X-Test": "1"})
	if got := h.Get("X-Frame-Options"); got != "SAMEORIGIN" {
		t.Fatalf("got %q", got)
	}
	if got := h.Get("X-Test"); got != "1" {
		t.Fatalf("got %q", got)
	}
}
