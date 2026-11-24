package api

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/inboxes-cig/bastion/internal/accesslog"
	"github.com/inboxes-cig/bastion/internal/auth"
	"github.com/inboxes-cig/bastion/internal/db"
	"github.com/inboxes-cig/bastion/internal/system"
	"gorm.io/gorm"
)

type StatsHandler struct {
	DB  *gorm.DB
	Log *accesslog.Logger
}

type certExpiry struct {
	ID       string     `json:"id"`
	Domain   string     `json:"domain"`
	NotAfter *time.Time `json:"not_after"`
	Source   string     `json:"source"`
}

func (h *StatsHandler) Get(c *gin.Context) {
	cl := auth.ClaimsFromContext(c)

	scope := func(q *gorm.DB) *gorm.DB {
		if cl != nil && cl.RoleName != "superadmin" && cl.TenantID != nil {
			return q.Where("tenant_id = ?", *cl.TenantID)
		}
		return q
	}

	var sites, certs, users, tenants, rules, templates int64
	scope(h.DB.Model(&db.Site{})).Count(&sites)
	scope(h.DB.Model(&db.Certificate{})).Count(&certs)
	scope(h.DB.Model(&db.User{})).Count(&users)
	h.DB.Model(&db.Tenant{}).Count(&tenants)
	scope(h.DB.Model(&db.GeoIPRule{})).Count(&rules)
	scope(h.DB.Model(&db.SiteTemplate{})).Count(&templates)

	var expiring []db.Certificate
	q := scope(h.DB.Model(&db.Certificate{}))
	q.Where("not_after IS NOT NULL AND not_after < ?", time.Now().Add(30*24*time.Hour)).
		Order("not_after ASC").Limit(10).Find(&expiring)

	list := []certExpiry{}
	for _, e := range expiring {
		list = append(list, certExpiry{ID: e.ID.String(), Domain: e.Domain, NotAfter: e.NotAfter, Source: e.Source})
	}

	resp := gin.H{
		"sites":          sites,
		"certs":          certs,
		"users":          users,
		"tenants":        tenants,
		"rules":          rules,
		"templates":      templates,
		"expiring_certs": list,
	}
	if h.Log != nil {
		resp["metrics"] = h.Log.Stats()
		resp["traffic_24h"] = h.Log.StatsFor(time.Now().Add(-24 * time.Hour))
	}
	resp["system"] = system.Snapshot()
	OK(c, resp)
}

func (h *StatsHandler) LogFiles(c *gin.Context) {
	if h.Log == nil {
		OK(c, []accesslog.LogFile{})
		return
	}
	files, err := h.Log.Files()
	if err != nil {
		InternalError(c, err.Error())
		return
	}
	OK(c, files)
}

func (h *StatsHandler) Logs(c *gin.Context) {
	if h.Log == nil {
		OK(c, []accesslog.Entry{})
		return
	}
	q := accesslog.Query{
		Site:         c.Query("site"),
		Search:       c.Query("q"),
		ExcludeAdmin: c.Query("hide_admin") == "true",
	}
	if b := c.Query("blocked"); b == "true" || b == "false" {
		v := b == "true"
		q.Blocked = &v
	}
	if s := c.Query("status"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			q.Status = n
		}
	}
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil {
			q.Limit = n
		}
	}
	if file := c.Query("file"); file != "" {
		OK(c, h.Log.QueryFile(file, q))
		return
	}
	OK(c, h.Log.Query(q))
}
