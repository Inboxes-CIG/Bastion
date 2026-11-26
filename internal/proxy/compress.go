package proxy

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

var gzipPool = sync.Pool{
	New: func() any {
		w, _ := gzip.NewWriterLevel(io.Discard, gzip.DefaultCompression)
		return w
	},
}

// gzipResponseWriter compresses eligible responses when the client accepts
// gzip. The decision is made in WriteHeader, so it can honour the upstream
// Content-Type, Content-Encoding, status and Content-Length. It is lazy: no
// gzip stream is produced for ineligible responses.
type gzipResponseWriter struct {
	http.ResponseWriter
	gz          *gzip.Writer
	wroteHeader bool
	enabled     bool
	closed      bool
}

func newGzipResponseWriter(w http.ResponseWriter) *gzipResponseWriter {
	return &gzipResponseWriter{ResponseWriter: w, gz: gzipPool.Get().(*gzip.Writer)}
}

func (g *gzipResponseWriter) WriteHeader(code int) {
	if g.wroteHeader {
		g.ResponseWriter.WriteHeader(code)
		return
	}
	// Informational responses (except 101) are forwarded and the final status
	// is decided afterwards.
	if code >= 100 && code < 200 && code != http.StatusSwitchingProtocols {
		g.ResponseWriter.WriteHeader(code)
		return
	}
	g.wroteHeader = true
	h := g.Header()
	if code >= 200 && code != http.StatusNoContent && code != http.StatusNotModified &&
		code != http.StatusSwitchingProtocols && h.Get("Content-Encoding") == "" &&
		compressibleContentType(h.Get("Content-Type")) {
		if cl := h.Get("Content-Length"); cl == "" || contentLengthAtLeast(cl, 860) {
			h.Del("Content-Length")
			h.Set("Content-Encoding", "gzip")
			h.Del("Content-MD5")
			h.Del("Accept-Ranges")
			weakenETag(h)
			g.enabled = true
		}
	}
	g.ResponseWriter.WriteHeader(code)
	if g.enabled {
		g.gz.Reset(g.ResponseWriter)
	}
}

func (g *gzipResponseWriter) Write(b []byte) (int, error) {
	if !g.wroteHeader {
		g.WriteHeader(http.StatusOK)
	}
	if g.enabled {
		return g.gz.Write(b)
	}
	return g.ResponseWriter.Write(b)
}

// Close finishes the gzip stream (if any) and returns the writer to the pool.
// It must be called after the upstream handler returned and before a cache
// layer inspects the buffered body.
func (g *gzipResponseWriter) Close() error {
	if g.closed {
		return nil
	}
	g.closed = true
	var err error
	if g.enabled {
		err = g.gz.Close()
	}
	gzipPool.Put(g.gz)
	return err
}

func (g *gzipResponseWriter) Flush() {
	if g.enabled {
		_ = g.gz.Flush()
	}
	if f, ok := g.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (g *gzipResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := g.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, fmt.Errorf("hijacking not supported")
}

func contentLengthAtLeast(v string, min int64) bool {
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return true
	}
	return n >= min
}

// acceptsGzip parses Accept-Encoding including q-values and the * wildcard.
func acceptsGzip(r *http.Request) bool {
	qGzip, foundGzip := -1.0, false
	qAny, foundAny := -1.0, false
	for _, part := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name, params, _ := strings.Cut(part, ";")
		name = strings.ToLower(strings.TrimSpace(name))
		q := 1.0
		for _, param := range strings.Split(params, ";") {
			param = strings.TrimSpace(param)
			if param == "" {
				continue
			}
			k, v, ok := strings.Cut(param, "=")
			if !ok || !strings.EqualFold(strings.TrimSpace(k), "q") {
				continue
			}
			if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
				q = f
			}
		}
		switch name {
		case "gzip":
			qGzip, foundGzip = q, true
		case "*":
			qAny, foundAny = q, true
		}
	}
	if foundGzip {
		return qGzip > 0
	}
	if foundAny {
		return qAny > 0
	}
	return false
}

func compressibleContentType(v string) bool {
	ct := strings.ToLower(strings.TrimSpace(strings.SplitN(v, ";", 2)[0]))
	if ct == "" {
		return false
	}
	if strings.HasPrefix(ct, "text/") {
		return ct != "text/event-stream"
	}
	switch ct {
	case "application/json", "application/ld+json", "application/javascript",
		"application/x-javascript", "application/xml", "application/xhtml+xml",
		"application/rss+xml", "application/atom+xml", "application/wasm",
		"image/svg+xml":
		return true
	}
	return strings.HasSuffix(ct, "+json") || strings.HasSuffix(ct, "+xml")
}

func addVary(h http.Header, value string) {
	for _, existing := range h.Values("Vary") {
		for _, item := range strings.Split(existing, ",") {
			item = strings.TrimSpace(item)
			if item == "*" || strings.EqualFold(item, value) {
				return
			}
		}
	}
	h.Add("Vary", value)
}

func weakenETag(h http.Header) {
	etag := h.Get("ETag")
	if etag == "" || strings.HasPrefix(etag, "W/") {
		return
	}
	h.Set("ETag", "W/"+etag)
}
