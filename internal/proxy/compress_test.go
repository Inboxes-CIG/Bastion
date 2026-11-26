package proxy

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAcceptsGzip(t *testing.T) {
	for header, want := range map[string]bool{
		"":              false,
		"gzip":          true,
		"deflate, gzip": true,
		"gzip;q=0":      false,
		"gzip;q=0.5":    true,
		"br, *":         true,
		"br, *;q=0":     false,
		"gzip;q=0, *":   false,
		"identity":      false,
		"GZIP":          true,
	} {
		r := httptest.NewRequest("GET", "/", nil)
		if header != "" {
			r.Header.Set("Accept-Encoding", header)
		}
		if got := acceptsGzip(r); got != want {
			t.Fatalf("acceptsGzip(%q) = %v, want %v", header, got, want)
		}
	}
}

func TestGzipResponseWriter(t *testing.T) {
	rec := httptest.NewRecorder()
	g := newGzipResponseWriter(rec)
	g.Header().Set("Content-Type", "text/html; charset=utf-8")
	g.Header().Set("ETag", `"abc"`)
	g.WriteHeader(http.StatusOK)
	body := strings.Repeat("hello world ", 200)
	if _, err := g.Write([]byte(body)); err != nil {
		t.Fatal(err)
	}
	if err := g.Close(); err != nil {
		t.Fatal(err)
	}
	if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q", got)
	}
	if got := rec.Header().Get("Content-Length"); got != "" {
		t.Fatalf("Content-Length should be removed, got %q", got)
	}
	if got := rec.Header().Get("ETag"); got != `W/"abc"` {
		t.Fatalf("ETag = %q", got)
	}
	zr, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != body {
		t.Fatalf("roundtrip mismatch: %d bytes", len(out))
	}
}

func TestGzipSkipsIneligible(t *testing.T) {
	for name, setup := range map[string]func(*gzipResponseWriter){
		"image": func(g *gzipResponseWriter) { g.Header().Set("Content-Type", "image/png") },
		"already-encoded": func(g *gzipResponseWriter) {
			g.Header().Set("Content-Type", "text/html")
			g.Header().Set("Content-Encoding", "br")
		},
		"small": func(g *gzipResponseWriter) {
			g.Header().Set("Content-Type", "text/html")
			g.Header().Set("Content-Length", "100")
		},
		"no-content-type": func(g *gzipResponseWriter) {},
	} {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			g := newGzipResponseWriter(rec)
			setup(g)
			g.WriteHeader(http.StatusOK)
			_, _ = g.Write([]byte("plain"))
			_ = g.Close()
			if got := rec.Header().Get("Content-Encoding"); got == "gzip" {
				t.Fatalf("unexpected compression: %q", got)
			}
			if rec.Body.String() != "plain" {
				t.Fatalf("body = %q", rec.Body.String())
			}
		})
	}
}

func TestCompressibleContentType(t *testing.T) {
	for ct, want := range map[string]bool{
		"text/html; charset=utf-8": true,
		"application/json":         true,
		"application/problem+json": true,
		"image/svg+xml":            true,
		"application/javascript":   true,
		"text/event-stream":        false,
		"image/png":                false,
		"application/octet-stream": false,
		"":                         false,
	} {
		if got := compressibleContentType(ct); got != want {
			t.Fatalf("compressibleContentType(%q) = %v, want %v", ct, got, want)
		}
	}
}
