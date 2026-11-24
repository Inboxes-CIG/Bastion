package proxy

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/inboxes-cig/bastion/internal/accesslog"
	"github.com/inboxes-cig/bastion/internal/cert"
	"github.com/inboxes-cig/bastion/internal/cache"
	"github.com/inboxes-cig/bastion/internal/crowdsec"
	"github.com/inboxes-cig/bastion/internal/db"
	"github.com/inboxes-cig/bastion/internal/geoip"
	"github.com/quic-go/quic-go/http3"
	"github.com/inboxes-cig/bastion/internal/iplist"
	"github.com/inboxes-cig/bastion/internal/ratelimit"
	"github.com/inboxes-cig/bastion/internal/siteaccess"
	"github.com/inboxes-cig/bastion/internal/web"
	"gorm.io/gorm"
)

type Server struct {
	DB         *gorm.DB
	Certs      *cert.Manager
	GeoIP      *geoip.Lookup
	CrowdSec   *crowdsec.Client
	Log        *accesslog.Logger
	IPList     *iplist.List
	RateLimiter *ratelimit.Limiter
	Cache       *cache.Cache
	HTTP3Enabled bool
	Access       *siteaccess.Cache
	Lockout      *siteaccess.Lockout
	HTTPAddr   string
	HTTPSAddr  string
	AdminHosts map[string]bool
}

func NewServer(db *gorm.DB, cm *cert.Manager, g *geoip.Lookup, cs *crowdsec.Client, lg *accesslog.Logger, il *iplist.List, rl *ratelimit.Limiter, ca *cache.Cache, httpAddr, httpsAddr string, adminHosts []string) *Server {
	ah := map[string]bool{}
	for _, h := range adminHosts {
		ah[strings.ToLower(h)] = true
	}
	return &Server{DB: db, Certs: cm, GeoIP: g, CrowdSec: cs, Log: lg, IPList: il, RateLimiter: rl, Cache: ca, HTTPAddr: httpAddr, HTTPSAddr: httpsAddr, AdminHosts: ah}
}

// statusRecorder captures status code and bytes for access logging while
// preserving Flusher/Hijacker support for streaming and WebSocket upgrades.
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int64
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	n, err := r.ResponseWriter.Write(b)
	r.bytes += int64(n)
	return n, err
}

func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := r.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, fmt.Errorf("hijacking not supported")
}

// teeWriter streams the response to the client while buffering it (up to max)
// so a complete, cacheable response can be stored afterwards.
type teeWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
	buf         bytes.Buffer
	max         int
	overflow    bool
}

func (t *teeWriter) WriteHeader(code int) {
	if !t.wroteHeader {
		t.status = code
		t.wroteHeader = true
	}
	t.ResponseWriter.WriteHeader(code)
}

func (t *teeWriter) Write(b []byte) (int, error) {
	if !t.wroteHeader {
		t.status = http.StatusOK
		t.wroteHeader = true
	}
	if !t.overflow {
		if t.buf.Len()+len(b) > t.max {
			t.overflow = true
		} else {
			t.buf.Write(b)
		}
	}
	return t.ResponseWriter.Write(b)
}

func (t *teeWriter) Flush() {
	if f, ok := t.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (t *teeWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := t.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, fmt.Errorf("hijacking not supported")
}

func (s *Server) isAdminHost(host string) bool {
	h := strings.ToLower(strings.SplitN(host, ":", 2)[0])
	if s.AdminHosts[h] {
		return true
	}
	if net.ParseIP(h) != nil {
		return true
	}
	return false
}

func (s *Server) findSite(host string) (*db.Site, error) {
	h := strings.ToLower(strings.SplitN(host, ":", 2)[0])
	var sites []db.Site
	if err := s.DB.Find(&sites).Error; err != nil {
		return nil, err
	}
	for i := range sites {
		if strings.ToLower(sites[i].Domain) == h {
			return &sites[i], nil
		}
		for _, d := range sites[i].ExtraDomains {
			if strings.ToLower(d) == h {
				return &sites[i], nil
			}
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func getClientIP(r *http.Request) string {
	ip := r.RemoteAddr
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		ip = strings.SplitN(fwd, ",", 2)[0]
	} else if xr := r.Header.Get("X-Real-IP"); xr != "" {
		ip = xr
	}
	ip = strings.TrimSpace(ip)
	if h, _, err := net.SplitHostPort(ip); err == nil {
		ip = h
	}
	return ip
}

func silentBlock(w http.ResponseWriter) {
	if sr, ok := w.(*statusRecorder); ok {
		sr.status = http.StatusForbidden
	}
	if hj, ok := w.(http.Hijacker); ok {
		if conn, _, err := hj.Hijack(); err == nil {
			_ = conn.Close()
			return
		}
	}
	w.WriteHeader(http.StatusForbidden)
}

func (s *Server) proxyHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	sw := &statusRecorder{ResponseWriter: w, status: 200}
	skipLog := false

	entry := accesslog.Entry{
		Time:      start,
		ClientIP:  getClientIP(r),
		Host:      r.Host,
		Method:    r.Method,
		Path:      r.URL.Path,
		UserAgent: r.UserAgent(),
		Referer:   r.Referer(),
		Proto:     r.Proto,
	}
	defer func() {
		entry.Status = sw.status
		entry.Bytes = sw.bytes
		entry.DurationMs = time.Since(start).Milliseconds()
		if !skipLog && s.Log != nil {
			s.Log.Record(entry)
		}
	}()

	host := r.Host

	// Admin host: GUI + API (logged, but excluded from site statistics)
	if s.isAdminHost(host) {
		entry.Domain = "(admin)"
		entry.Admin = true
		if strings.HasPrefix(r.URL.Path, "/assets/") || r.URL.Path == "/healthz" || r.URL.Path == "/favicon.svg" || strings.HasPrefix(r.URL.Path, "/icons") {
			skipLog = true
		}
		if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/healthz" {
			upstream, _ := url.Parse("http://127.0.0.1:8080")
			rp := httputil.NewSingleHostReverseProxy(upstream)
			rp.ServeHTTP(sw, r)
			return
		}
		web.Handler().ServeHTTP(sw, r)
		return
	}

	// resolve country for stats / ranking (GeoIP db lookup)
	if s.GeoIP != nil {
		if cc, err := s.GeoIP.CountryCode(entry.ClientIP); err == nil {
			entry.Country = cc
		}
	}

	allowed := false
	if s.IPList != nil {
		if s.IPList.IsDenied(entry.ClientIP) {
			entry.Blocked = true
			entry.BlockReason = "blacklist"
			silentBlock(sw)
			return
		}
		allowed = s.IPList.IsAllowed(entry.ClientIP)
	}

	// CrowdSec ban check (before any upstream contact)
	if !allowed && s.CrowdSec != nil && s.CrowdSec.IsBanned(entry.ClientIP) {
		entry.Blocked = true
		entry.BlockReason = "crowdsec"
		silentBlock(sw)
		return
	}

	site, err := s.findSite(host)
	if err != nil {
		entry.Domain = "(unknown)"
		entry.BlockReason = "unknown_site"
		sw.WriteHeader(http.StatusBadGateway)
		return
	}
	entry.SiteID = site.ID.String()
	entry.Domain = site.Domain

	// Resolve template
	var tmpl *db.SiteTemplate
	if site.TemplateID != nil {
		var t db.SiteTemplate
		if err := s.DB.First(&t, "id = ?", *site.TemplateID).Error; err == nil {
			tmpl = &t
		}
	}

	// Per-site access control (IP allow/deny + Basic Auth)
	if s.Access != nil {
		data := s.Access.Get(site.ID.String())
		if data.Denied(entry.ClientIP) {
			entry.Blocked = true
			entry.BlockReason = "site_blacklist"
			silentBlock(sw)
			return
		}
		authEnabled := false
		if site.AuthEnabled != nil {
			authEnabled = *site.AuthEnabled
		} else if tmpl != nil && tmpl.AuthEnabled != nil {
			authEnabled = *tmpl.AuthEnabled
		}
		if authEnabled {
			// Only the per-site allow list bypasses the login; the global
			// whitelist does NOT.
			ipAllowed := data.Allowed(entry.ClientIP)
			if !ipAllowed {
				key := site.ID.String() + "|" + entry.ClientIP
				if s.Lockout != nil && s.Lockout.Locked(key) {
					entry.Blocked = true
					entry.BlockReason = "site_auth_locked"
					sw.Header().Set("Retry-After", "300")
					sw.WriteHeader(http.StatusTooManyRequests)
					return
				}
				user, pass, ok := r.BasicAuth()
				if !ok || !data.Validate(user, pass) {
					if s.Lockout != nil {
						s.Lockout.Fail(key)
					}
					entry.Blocked = true
					entry.BlockReason = "site_auth"
					sw.Header().Set("WWW-Authenticate", `Basic realm="Protected"`)
					sw.WriteHeader(http.StatusUnauthorized)
					return
				}
				if s.Lockout != nil {
					s.Lockout.Reset(key)
				}
			}
		}
	}

	// Effective upstream URL
	upstreamURL := ""
	if site.UpstreamURL != nil {
		upstreamURL = *site.UpstreamURL
	} else if tmpl != nil && tmpl.UpstreamURL != nil {
		upstreamURL = *tmpl.UpstreamURL
	}
	upstreamHost := ""
	if site.UpstreamHost != nil {
		upstreamHost = *site.UpstreamHost
	} else if tmpl != nil && tmpl.UpstreamHost != nil {
		upstreamHost = *tmpl.UpstreamHost
	}

	if upstreamURL == "" {
		entry.BlockReason = "no_upstream"
		sw.WriteHeader(http.StatusBadGateway)
		return
	}

	// GeoIP filter (effective)
	geoEnabled := false
	if site.GeoIPEnabled != nil {
		geoEnabled = *site.GeoIPEnabled
	} else if tmpl != nil && tmpl.GeoIPEnabled != nil {
		geoEnabled = *tmpl.GeoIPEnabled
	}
	if !allowed && geoEnabled && s.GeoIP != nil {
		blocked := false
		cc := entry.Country
		if cc != "" {
			mode := "block"
			if site.GeoIPMode != nil {
				mode = *site.GeoIPMode
			} else if tmpl != nil && tmpl.GeoIPMode != nil {
				mode = *tmpl.GeoIPMode
			}
			var countries []string
			if site.GeoIPCountries != nil {
				countries = site.GeoIPCountries
			} else if tmpl != nil {
				countries = tmpl.GeoIPCountries
			}
			inList := false
			for _, c := range countries {
				if strings.EqualFold(c, cc) {
					inList = true
					break
				}
			}
			if mode == "block" {
				blocked = inList
			} else if mode == "allow" && len(countries) > 0 && !inList {
				blocked = true
			}
		}
		if !blocked {
			blocked = geoip.EvaluateRules(s.DB, s.GeoIP, entry.ClientIP, site.TenantID.String(), site.ID.String())
		}
		if blocked {
			entry.Blocked = true
			entry.BlockReason = "geoip"
			silentBlock(sw)
			return
		}
	}

	// Rate limiting (effective from site/template)
	if !allowed && s.RateLimiter != nil {
		rps := 0
		if site.RateLimitRPS != nil {
			rps = *site.RateLimitRPS
		} else if tmpl != nil && tmpl.RateLimitRPS != nil {
			rps = *tmpl.RateLimitRPS
		}
		burst := 0
		if site.RateLimitBurst != nil {
			burst = *site.RateLimitBurst
		} else if tmpl != nil && tmpl.RateLimitBurst != nil {
			burst = *tmpl.RateLimitBurst
		}
		if rps > 0 && !s.RateLimiter.Allow(site.ID.String()+"|"+entry.ClientIP, rps, burst) {
			entry.Blocked = true
			entry.BlockReason = "ratelimit"
			sw.Header().Set("Retry-After", "1")
			sw.WriteHeader(http.StatusTooManyRequests)
			return
		}
	}

	// Response cache (anonymous GET only, per-site settings with template inheritance)
	cacheEnabled := false
	cacheTTL := 0
	cacheMaxObj := 0
	if s.Cache != nil {
		cfg := s.Cache.Config()
		cacheEnabled = cfg.Enabled
		cacheTTL = cfg.TTLSeconds
		cacheMaxObj = cfg.MaxObjectKB
	}
	if site.CacheEnabled != nil {
		cacheEnabled = *site.CacheEnabled
	} else if tmpl != nil && tmpl.CacheEnabled != nil {
		cacheEnabled = *tmpl.CacheEnabled
	}
	if site.CacheTTL != nil {
		cacheTTL = *site.CacheTTL
	} else if tmpl != nil && tmpl.CacheTTL != nil {
		cacheTTL = *tmpl.CacheTTL
	}
	if site.CacheMaxObjectKB != nil {
		cacheMaxObj = *site.CacheMaxObjectKB
	} else if tmpl != nil && tmpl.CacheMaxObjectKB != nil {
		cacheMaxObj = *tmpl.CacheMaxObjectKB
	}

	cacheKey := ""
	reqNoCache := strings.Contains(strings.ToLower(r.Header.Get("Cache-Control")), "no-cache") || strings.EqualFold(r.Header.Get("Pragma"), "no-cache")
	if s.Cache != nil && cacheEnabled && r.Method == http.MethodGet && r.Header.Get("Authorization") == "" && !reqNoCache {
		cacheKey = r.Host + " " + r.URL.RequestURI() + " " + r.Header.Get("Accept-Encoding")
		if e, ok := s.Cache.Get(cacheKey); ok {
			for k, vals := range e.Header {
				for _, v := range vals {
					sw.Header().Add(k, v)
				}
			}
			sw.Header().Set("X-Cache", "HIT")
			sw.Header().Set("Content-Length", strconv.Itoa(len(e.Body)))
			sw.WriteHeader(e.Status)
			_, _ = sw.Write(e.Body)
			return
		}
		sw.Header().Set("X-Cache", "MISS")
	}

	// Advertise HTTP/3 for this site (Alt-Svc); clients upgrade on next request.
	if s.HTTP3Enabled {
		h3 := false
		if site.HTTP3Enabled != nil {
			h3 = *site.HTTP3Enabled
		} else if tmpl != nil && tmpl.HTTP3Enabled != nil {
			h3 = *tmpl.HTTP3Enabled
		}
		if h3 {
			sw.Header().Set("Alt-Svc", `h3=":443"; ma=86400`)
		}
	}

	upstream, err := url.Parse(upstreamURL)
	if err != nil {
		entry.BlockReason = "bad_upstream"
		sw.WriteHeader(http.StatusBadGateway)
		return
	}

	rp := httputil.NewSingleHostReverseProxy(upstream)
	origHost := r.Host
	origDirector := rp.Director
	rp.Director = func(req *http.Request) {
		origDirector(req)
		// Tell the upstream it is behind HTTPS so it generates https URLs.
		req.Header.Set("X-Forwarded-Proto", "https")
		req.Header.Set("X-Forwarded-Host", origHost)
		if upstreamHost != "" {
			req.Host = upstreamHost
		}
	}
	rp.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, e error) {
		log.Printf("proxy error for %s: %v", host, e)
		rw.WriteHeader(http.StatusBadGateway)
	}
	if cacheKey != "" && s.Cache != nil {
		tw := &teeWriter{ResponseWriter: sw, status: http.StatusOK, max: max(cacheMaxObj*1024, 4096)}
		rp.ServeHTTP(tw, r)
		if !tw.overflow {
			cacheableStatus := (tw.status >= 200 && tw.status < 300 && tw.status != 206) ||
				(tw.status >= 300 && tw.status <= 308) || tw.status == 404 || tw.status == 410
			cc := strings.ToLower(tw.Header().Get("Cache-Control"))
			vary := strings.ToLower(tw.Header().Get("Vary"))
			if cacheableStatus && !strings.Contains(cc, "no-cache") && !strings.Contains(cc, "no-store") &&
				tw.Header().Get("Set-Cookie") == "" && !strings.Contains(vary, "cookie") {
				s.Cache.Set(cacheKey, site.ID.String(), tw.status, tw.Header(), tw.buf.Bytes(), cacheTTL, cacheMaxObj)
			}
		}
	} else {
		rp.ServeHTTP(sw, r)
	}
}

func (s *Server) Start() error {
	httpHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target := "https://" + r.Host + r.URL.RequestURI()
		http.Redirect(w, r, target, http.StatusMovedPermanently)
	})
	httpSrv := &http.Server{
		Addr:              s.HTTPAddr,
		Handler:           s.Certs.HTTPChallengeHandler(httpHandler),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		log.Printf("HTTP listener on %s", s.HTTPAddr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("http listen: %v", err)
		}
	}()

	tlsCfg := &tls.Config{
		GetCertificate: s.Certs.GetCertificate,
		GetConfigForClient: func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
			// Do not risk locking out the admin UI (IP access / admin hosts).
			if hello.ServerName == "" || s.isAdminHost(hello.ServerName) {
				return nil, nil
			}
			ip := ""
			if hello.Conn != nil {
				if host, _, err := net.SplitHostPort(hello.Conn.RemoteAddr().String()); err == nil {
					ip = host
				}
			}
			blocked := false
			allowed := false
			reason := ""
			if ip != "" && s.IPList != nil {
				if s.IPList.IsDenied(ip) {
					blocked, reason = true, "blacklist"
				} else if s.IPList.IsAllowed(ip) {
					allowed = true
				}
			}
			if !blocked && !allowed && ip != "" && s.CrowdSec != nil && s.CrowdSec.IsBanned(ip) {
				blocked, reason = true, "crowdsec"
			}
			if blocked {
				if s.Log != nil {
					s.Log.Record(accesslog.Entry{
						Time:        time.Now(),
						ClientIP:    ip,
						Host:        hello.ServerName,
						Method:      "-",
						Path:        "-",
						Status:      http.StatusForbidden,
						Blocked:     true,
						BlockReason: reason,
					})
				}
				return nil, fmt.Errorf("connection rejected")
			}
			return nil, nil
		},
		MinVersion: tls.VersionTLS12,
	}
	httpsSrv := &http.Server{
		Addr:              s.HTTPSAddr,
		Handler:           http.HandlerFunc(s.proxyHandler),
		TLSConfig:         tlsCfg,
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("HTTPS listener on %s", s.HTTPSAddr)

	if s.HTTP3Enabled {
		h3TLS := tlsCfg.Clone()
		h3TLS.NextProtos = []string{"h3"}
		h3Srv := &http3.Server{
			Addr:      s.HTTPSAddr,
			TLSConfig: h3TLS,
			Handler:   http.HandlerFunc(s.proxyHandler),
		}
		go func() {
			log.Printf("HTTP/3 (QUIC) listener on udp %s", s.HTTPSAddr)
			if err := h3Srv.ListenAndServe(); err != nil {
				log.Printf("http3 listen: %v", err)
			}
		}()
	}

	return httpsSrv.ListenAndServeTLS("", "")
}
