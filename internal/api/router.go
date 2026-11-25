package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/inboxes-cig/bastion/internal/accesslog"
	"github.com/inboxes-cig/bastion/internal/audit"
	"github.com/inboxes-cig/bastion/internal/auth"
	"github.com/inboxes-cig/bastion/internal/cache"
	"github.com/inboxes-cig/bastion/internal/cert"
	"github.com/inboxes-cig/bastion/internal/crowdsec"
	"github.com/inboxes-cig/bastion/internal/iplist"
	"github.com/inboxes-cig/bastion/internal/siteaccess"
	"github.com/inboxes-cig/bastion/internal/web"
	"gorm.io/gorm"
)

type Deps struct {
	DB         *gorm.DB
	JWTSecret  string
	SessionSecret string
	Certs      *cert.Manager
	CrowdSec   *crowdsec.Client
	Log        *accesslog.Logger
	AdminHosts []string
	IPList     *iplist.List
	Cache      *cache.Cache
	Access     *siteaccess.Cache
	CORSOrigins []string
	AllowPrivateUpstream bool
	AllowedUpstreamHosts []string
}

func securityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "no-referrer")
		c.Next()
	}
}

func cors(origins []string) gin.HandlerFunc {
	allowed := map[string]bool{}
	for _, o := range origins {
		if o = strings.TrimSpace(o); o != "" {
			allowed[o] = true
		}
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" && allowed[origin] {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			if c.Request.Method == http.MethodOptions {
				c.AbortWithStatus(http.StatusNoContent)
				return
			}
		}
		c.Next()
	}
}

func NewRouter(d Deps) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(gin.Logger())
	r.Use(securityHeaders())
	r.Use(cors(d.CORSOrigins))
	r.Use(audit.Middleware(d.DB))

	mfaKey := d.SessionSecret
	if mfaKey == "" {
		mfaKey = d.JWTSecret
	}
	authH := &AuthHandler{
		DB:        d.DB,
		JWTSecret: d.JWTSecret,
		MFAKey:    mfaKey,
		Limiter:   auth.NewLoginLimiter(3, 10*time.Minute, 10*time.Minute),
	}
	tenantH := &TenantHandler{DB: d.DB}
	userH := &UserHandler{DB: d.DB}
	roleH := &RoleHandler{DB: d.DB}
	certH := &CertHandler{DB: d.DB, Certs: d.Certs}
	tmplH := &TemplateHandler{DB: d.DB, AllowPrivateUpstream: d.AllowPrivateUpstream, AllowedUpstreamHosts: d.AllowedUpstreamHosts}
	siteH := &SiteHandler{DB: d.DB, AllowPrivateUpstream: d.AllowPrivateUpstream, AllowedUpstreamHosts: d.AllowedUpstreamHosts}
	locH := &LocationHandler{DB: d.DB, Cache: d.Cache, AllowPrivateUpstream: d.AllowPrivateUpstream, AllowedUpstreamHosts: d.AllowedUpstreamHosts}
	geoipH := &GeoIPHandler{DB: d.DB}
	statsH := &StatsHandler{DB: d.DB, Log: d.Log}
	csH := &CrowdSecHandler{CS: d.CrowdSec}
	auditH := &AuditHandler{DB: d.DB}
	gaH := &GoodActorHandler{DB: d.DB, CS: d.CrowdSec}
	versionH := &VersionHandler{CachePath: "/var/lib/bastion/version_cache.json"}
	accessH := &SiteAccessHandler{DB: d.DB, Access: d.Access}
	adminH := &AdminHandler{DB: d.DB, AdminHosts: d.AdminHosts}
	iplH := &IPListHandler{DB: d.DB, IPs: d.IPList}
	cacheH := &CacheHandler{DB: d.DB, Cache: d.Cache}

	r.GET("/healthz", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })

	v1 := r.Group("/api/v1")
	{
		v1.POST("/auth/login", authH.Login)
		v1.POST("/auth/refresh", authH.Refresh)
		v1.POST("/auth/login/mfa", authH.MFALogin)

		protected := v1.Group("")
		protected.Use(auth.AuthMiddleware(d.JWTSecret))
		protected.Use(PasswordChangeGuard(d.DB))
		{
			protected.GET("/auth/me", authH.Me)
			protected.POST("/auth/change-password", authH.ChangePassword)
			protected.POST("/auth/logout", authH.Logout)
			protected.POST("/auth/mfa/setup", authH.MFASetup)
			protected.POST("/auth/mfa/enable", authH.MFAEnable)
			protected.POST("/auth/mfa/disable", authH.MFADisable)
			protected.GET("/stats", statsH.Get)
			protected.GET("/admin/info", adminH.Info)

			// Local IP allow/deny lists
			protected.GET("/cache", cacheH.Get)
			protected.PUT("/cache/config", auth.RequireRole("superadmin"), cacheH.Update)
			protected.POST("/cache/clear", auth.RequireRole("superadmin", "tenant_admin"), cacheH.Clear)

			protected.GET("/iplist", auth.RequireRole("superadmin"), iplH.List)
			protected.POST("/iplist", auth.RequireRole("superadmin"), iplH.Create)
			protected.PUT("/iplist/:id", auth.RequireRole("superadmin"), iplH.Update)
			protected.DELETE("/iplist/:id", auth.RequireRole("superadmin"), iplH.Delete)
			protected.GET("/logs", statsH.Logs)
			protected.GET("/logs/files", statsH.LogFiles)

			protected.GET("/roles", roleH.List)

			protected.GET("/tenants", tenantH.List)
			protected.GET("/tenants/:id", tenantH.Get)
			protected.POST("/tenants", auth.RequireRole("superadmin"), tenantH.Create)
			protected.PUT("/tenants/:id", auth.RequireRole("superadmin"), tenantH.Update)
			protected.DELETE("/tenants/:id", auth.RequireRole("superadmin"), tenantH.Delete)

			protected.GET("/users", userH.List)
			protected.GET("/users/:id", userH.Get)
			protected.POST("/users", auth.RequireRole("superadmin", "tenant_admin"), userH.Create)
			protected.PUT("/users/:id", auth.RequireRole("superadmin", "tenant_admin"), userH.Update)
			protected.DELETE("/users/:id", auth.RequireRole("superadmin", "tenant_admin"), userH.Delete)

			protected.GET("/certificates", certH.List)
			protected.GET("/certificates/:id", certH.Get)
			protected.POST("/certificates", auth.RequireRole("superadmin", "tenant_admin"), certH.Upload)
			protected.POST("/certificates/issue", auth.RequireRole("superadmin", "tenant_admin"), certH.Issue)
			protected.DELETE("/certificates/:id", auth.RequireRole("superadmin", "tenant_admin"), certH.Delete)

			protected.GET("/sites/:id/locations", locH.List)
			protected.POST("/sites/:id/locations", auth.RequireRole("superadmin", "tenant_admin"), locH.Create)
			protected.PUT("/sites/:id/locations/:lid", auth.RequireRole("superadmin", "tenant_admin"), locH.Update)
			protected.DELETE("/sites/:id/locations/:lid", auth.RequireRole("superadmin", "tenant_admin"), locH.Delete)

			protected.GET("/sites/:id/locations/:lid/users", accessH.ListUsers)
			protected.POST("/sites/:id/locations/:lid/users", auth.RequireRole("superadmin", "tenant_admin"), accessH.CreateUser)
			protected.PUT("/sites/:id/locations/:lid/users/:uid", auth.RequireRole("superadmin", "tenant_admin"), accessH.UpdateUser)
			protected.DELETE("/sites/:id/locations/:lid/users/:uid", auth.RequireRole("superadmin", "tenant_admin"), accessH.DeleteUser)
			protected.GET("/sites/:id/locations/:lid/ip-rules", accessH.ListIPRules)
			protected.POST("/sites/:id/locations/:lid/ip-rules", auth.RequireRole("superadmin", "tenant_admin"), accessH.CreateIPRule)
			protected.PUT("/sites/:id/locations/:lid/ip-rules/:rid", auth.RequireRole("superadmin", "tenant_admin"), accessH.UpdateIPRule)
			protected.DELETE("/sites/:id/locations/:lid/ip-rules/:rid", auth.RequireRole("superadmin", "tenant_admin"), accessH.DeleteIPRule)

			protected.GET("/sites/:id/users", accessH.ListUsers)
			protected.POST("/sites/:id/users", auth.RequireRole("superadmin", "tenant_admin"), accessH.CreateUser)
			protected.PUT("/sites/:id/users/:uid", auth.RequireRole("superadmin", "tenant_admin"), accessH.UpdateUser)
			protected.DELETE("/sites/:id/users/:uid", auth.RequireRole("superadmin", "tenant_admin"), accessH.DeleteUser)
			protected.GET("/sites/:id/ip-rules", accessH.ListIPRules)
			protected.POST("/sites/:id/ip-rules", auth.RequireRole("superadmin", "tenant_admin"), accessH.CreateIPRule)
			protected.PUT("/sites/:id/ip-rules/:rid", auth.RequireRole("superadmin", "tenant_admin"), accessH.UpdateIPRule)
			protected.DELETE("/sites/:id/ip-rules/:rid", auth.RequireRole("superadmin", "tenant_admin"), accessH.DeleteIPRule)

			protected.GET("/sites", siteH.List)
			protected.GET("/sites/:id", siteH.Get)
			protected.POST("/sites", auth.RequireRole("superadmin", "tenant_admin"), siteH.Create)
			protected.PUT("/sites/:id", auth.RequireRole("superadmin", "tenant_admin"), siteH.Update)
			protected.DELETE("/sites/:id", auth.RequireRole("superadmin", "tenant_admin"), siteH.Delete)

			// Templates
			protected.GET("/templates", tmplH.List)
			protected.GET("/templates/:id", tmplH.Get)
			protected.POST("/templates", auth.RequireRole("superadmin", "tenant_admin"), tmplH.Create)
			protected.PUT("/templates/:id", auth.RequireRole("superadmin", "tenant_admin"), tmplH.Update)
			protected.DELETE("/templates/:id", auth.RequireRole("superadmin", "tenant_admin"), tmplH.Delete)

			protected.GET("/geoip/countries", geoipH.Countries)

			// CrowdSec
			protected.GET("/crowdsec/status", csH.Status)
			protected.GET("/crowdsec/decisions", csH.Decisions)
			protected.POST("/crowdsec/decisions", auth.RequireRole("superadmin"), csH.Add)
			protected.DELETE("/crowdsec/decisions/:id", auth.RequireRole("superadmin"), csH.Delete)
			protected.GET("/crowdsec/alerts", csH.Alerts)
			protected.GET("/crowdsec/good-actors", gaH.List)
			protected.POST("/crowdsec/good-actors", auth.RequireRole("superadmin"), gaH.Create)
			protected.PUT("/crowdsec/good-actors/:id", auth.RequireRole("superadmin"), gaH.Update)
			protected.DELETE("/crowdsec/good-actors/:id", auth.RequireRole("superadmin"), gaH.Delete)

			protected.GET("/audit", auditH.List)
			protected.GET("/version", versionH.Get)
			protected.GET("/geoip/rules", geoipH.ListRules)
			protected.POST("/geoip/rules", auth.RequireRole("superadmin", "tenant_admin"), geoipH.CreateRule)
			protected.DELETE("/geoip/rules/:id", auth.RequireRole("superadmin", "tenant_admin"), geoipH.DeleteRule)
		}
	}

	// SPA fallback for frontend
	r.NoRoute(func(c *gin.Context) {
		web.Handler().ServeHTTP(c.Writer, c.Request)
	})

	return r
}
