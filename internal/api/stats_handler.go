package api

import (
	"math"
	"sort"
	"strconv"
	"strings"
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
	super := cl != nil && cl.RoleName == "superadmin"

	scope := func(q *gorm.DB) *gorm.DB {
		if super {
			return q
		}
		if cl != nil && cl.TenantID != nil {
			return q.Where("tenant_id = ?", *cl.TenantID)
		}
		return q.Where("1 = 0")
	}

	var sites, certs, users, tenants, rules, templates int64
	scope(h.DB.Model(&db.Site{})).Count(&sites)
	scope(h.DB.Model(&db.Certificate{})).Count(&certs)
	scope(h.DB.Model(&db.User{})).Count(&users)
	if super {
		h.DB.Model(&db.Tenant{}).Count(&tenants)
	}
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
		if super {
			resp["metrics"] = h.Log.Stats()
			resp["traffic_24h"] = h.Log.StatsFor(time.Now().Add(-24 * time.Hour))
		} else {
			domains, _ := tenantDomains(c, h.DB)
			empty := accesslog.Stats{ByStatus: map[string]int64{}, ByReason: map[string]int64{}}
			if len(domains) > 0 {
				resp["traffic_24h"] = h.Log.StatsForDomains(time.Now().Add(-24*time.Hour), domains)
			} else {
				resp["traffic_24h"] = empty
			}
			resp["metrics"] = empty
		}
	}
	if super {
		resp["system"] = system.Snapshot()
	}
	OK(c, resp)
}

func (h *StatsHandler) SiteStats(c *gin.Context) {
	rng := c.DefaultQuery("range", "24h")
	now := time.Now()
	var since time.Time
	switch rng {
	case "7d":
		since = now.AddDate(0, 0, -7)
	case "30d":
		since = now.AddDate(0, 0, -30)
	case "all":
		// zero value = everything that is retained
	default:
		rng = "24h"
		since = now.Add(-24 * time.Hour)
	}

	q := h.DB.Model(&db.Site{}).Select("id, domain, tenant_id, extra_domains")
	if !isSuperadmin(c) {
		cl := auth.ClaimsFromContext(c)
		if cl == nil || cl.TenantID == nil {
			OK(c, gin.H{"range": rng, "sites": []interface{}{}})
			return
		}
		q = q.Where("tenant_id = ?", *cl.TenantID)
	}
	var sites []db.Site
	if err := q.Find(&sites).Error; err != nil {
		InternalError(c, err.Error())
		return
	}
	hostToDomain := map[string]string{}
	for _, s := range sites {
		hostToDomain[strings.ToLower(s.Domain)] = s.Domain
		for _, d := range s.ExtraDomains {
			hostToDomain[strings.ToLower(d)] = s.Domain
		}
	}
	stats := map[string]accesslog.SiteRangeStats{}
	if h.Log != nil && len(hostToDomain) > 0 {
		stats = h.Log.SiteStatsSince(since, hostToDomain)
	}

	type siteResp struct {
		ID           string           `json:"id"`
		Domain       string           `json:"domain"`
		TenantID     string           `json:"tenant_id"`
		ExtraDomains []string         `json:"extra_domains"`
		Total        int64            `json:"total"`
		Blocked      int64            `json:"blocked"`
		Allowed      int64            `json:"allowed"`
		BlockRate    float64          `json:"block_rate"`
		ByStatus     map[string]int64 `json:"by_status"`
		ByReason     map[string]int64 `json:"by_reason"`
	}
	out := make([]siteResp, 0, len(sites))
	for _, s := range sites {
		st := stats[s.Domain]
		if st.ByStatus == nil {
			st.ByStatus = map[string]int64{}
		}
		if st.ByReason == nil {
			st.ByReason = map[string]int64{}
		}
		rate := 0.0
		if st.Total > 0 {
			rate = float64(st.Blocked) / float64(st.Total) * 100
		}
		out = append(out, siteResp{
			ID:           s.ID.String(),
			Domain:       s.Domain,
			TenantID:     s.TenantID.String(),
			ExtraDomains: []string(s.ExtraDomains),
			Total:        st.Total,
			Blocked:      st.Blocked,
			Allowed:      st.Allowed,
			BlockRate:    math.Round(rate*10) / 10,
			ByStatus:     st.ByStatus,
			ByReason:     st.ByReason,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Total == out[j].Total {
			return out[i].Domain < out[j].Domain
		}
		return out[i].Total > out[j].Total
	})
	resp := gin.H{"range": rng, "sites": out}
	if since.IsZero() {
		resp["since"] = nil
	} else {
		resp["since"] = since
	}
	OK(c, resp)
}

func (h *StatsHandler) LogFiles(c *gin.Context) {
	if h.Log == nil || !isSuperadmin(c) {
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
	if !isSuperadmin(c) {
		domains, ok := tenantDomains(c, h.DB)
		if !ok || len(domains) == 0 {
			OK(c, []accesslog.Entry{})
			return
		}
		q.Domains = domains
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
		if !isSuperadmin(c) {
			Forbidden(c, "forbidden")
			return
		}
		OK(c, h.Log.QueryFile(file, q))
		return
	}
	OK(c, h.Log.Query(q))
}
