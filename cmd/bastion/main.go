package main

import (
	"log"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/inboxes-cig/bastion/internal/accesslog"
	"github.com/inboxes-cig/bastion/internal/adminaccess"
	"github.com/inboxes-cig/bastion/internal/apikeys"
	"github.com/inboxes-cig/bastion/internal/autoban"
	"github.com/inboxes-cig/bastion/internal/api"
	"os"
	"os/signal"
	"syscall"
	"github.com/inboxes-cig/bastion/internal/cert"
	"github.com/inboxes-cig/bastion/internal/cache"
	"github.com/inboxes-cig/bastion/internal/config"
	"github.com/inboxes-cig/bastion/internal/crowdsec"
	"github.com/google/uuid"
	"github.com/inboxes-cig/bastion/internal/db"
	"github.com/inboxes-cig/bastion/internal/geoip"
	"github.com/inboxes-cig/bastion/internal/iplist"
	"github.com/inboxes-cig/bastion/internal/ratelimit"
	"github.com/inboxes-cig/bastion/internal/siteaccess"
	"github.com/inboxes-cig/bastion/internal/system"
	"github.com/inboxes-cig/bastion/internal/proxy"
)

func main() {
	cfg, err := config.Load("/etc/bastion/config.yaml")
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		log.Fatalf("config validate: %v", err)
	}

	database, err := db.Connect(cfg.Database.URL)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	if err := db.AutoMigrate(database); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	_ = crowdsec.WriteGoodActors(database, "/var/lib/bastion/good-actors.yaml")

	if res, err := db.Seed(database); err != nil {
		log.Fatalf("seed: %v", err)
	} else if res.Password != "" {
		log.Printf("=== INITIAL SUPERADMIN CREATED ===")
		log.Printf("Email: %s", res.Email)
		log.Printf("Password: %s", res.Password)
		log.Printf("Please change this password after first login!")
		log.Printf("===================================")
	}

	cm := cert.NewManager(database)

	g := geoip.NewLookup(cfg.GeoIP.DBPath)
	if err := g.Load(); err != nil {
		log.Printf("geoip db not loaded (download it first): %v", err)
	}

	system.Prime()
	accessLog := accesslog.NewWithPersistence(cfg.Logging.Dir, cfg.Logging.MaxMemory, cfg.Logging.RetentionDays)
	defer accessLog.Close()

	cacheEnabled := true
	if cfg.Cache.Enabled != nil {
		cacheEnabled = *cfg.Cache.Enabled
	}
	respCache := cache.New(cache.Config{
		Enabled:     cacheEnabled,
		TTLSeconds:  cfg.Cache.TTLSeconds,
		MaxEntries:  cfg.Cache.MaxEntries,
		MaxObjectKB: cfg.Cache.MaxObjectKB,
		MaxTotalMB:  cfg.Cache.MaxTotalMB,
	}, "/var/lib/bastion/cache.json")

	rl := ratelimit.New()
	access := siteaccess.New(database)
	lockout := siteaccess.NewLockout()
	ipL := iplist.New(database)
	_ = ipL.Reload()
	go ipL.Run(60 * time.Second)

	adminAccess := adminaccess.New(database)
	if err := adminAccess.Reload(); err != nil {
		log.Printf("admin access policy: %v", err)
	}
	go adminAccess.Run(60 * time.Second)

	apiKeys := apikeys.New(database)
	if err := apiKeys.Reload(); err != nil {
		log.Printf("api key policy: %v", err)
	}
	go apiKeys.Run(60 * time.Second)

	var cs *crowdsec.Client
	if cfg.CrowdSec.Enabled {
		cs = crowdsec.New(cfg.CrowdSec.LAPIURL, cfg.CrowdSec.APIKey)
		interval := cfg.CrowdSec.RefreshSeconds
		go cs.Run(time.Duration(interval) * time.Second)
		log.Printf("CrowdSec bouncer enabled (LAPI %s)", cfg.CrowdSec.LAPIURL)
	}
	if err := cm.LoadAll(); err != nil {
		log.Printf("cert load: %v", err)
	}
	adminHosts := cfg.Server.AdminHosts
	if len(adminHosts) == 0 {
		adminHosts = []string{"localhost"}
	}
	bootstrapHosts := append([]string{}, adminHosts...)
	bootstrapHosts = append(bootstrapHosts, localIPs()...)
	if err := cm.BootstrapAdmin(bootstrapHosts); err != nil {
		log.Fatalf("bootstrap admin cert: %v", err)
	}

	// Proxy server
	ps := proxy.NewServer(database, cm, g, cs, accessLog, ipL, rl, respCache, cfg.Server.HTTPAddr, cfg.Server.HTTPSAddr, adminHosts)
	h3 := true
	if cfg.Server.HTTP3 != nil {
		h3 = *cfg.Server.HTTP3
	}
	ps.HTTP3Enabled = h3
	ps.Access = access
	ps.Lockout = lockout
	ps.AllowPrivateUpstream = cfg.Security.Upstream.AllowPrivate
	ps.AllowedUpstreamHosts = cfg.Security.Upstream.AllowedHosts
	ps.AdminAccess = adminAccess

	autoBanEnabled := true
	if cfg.Security.AutoBan404.Enabled != nil {
		autoBanEnabled = *cfg.Security.AutoBan404.Enabled
	}
	firstBanDur, fderr := time.ParseDuration(cfg.Security.AutoBan404.FirstBan)
	if fderr != nil || firstBanDur <= 0 {
		firstBanDur = 10 * time.Minute
	}
	secondBanDur, sderr := time.ParseDuration(cfg.Security.AutoBan404.SecondBan)
	if sderr != nil || secondBanDur <= 0 {
		secondBanDur = 24 * time.Hour
	}
	ps.AutoBan = autoban.New(autoban.Config{
		Enabled:       autoBanEnabled,
		Threshold:     cfg.Security.AutoBan404.Threshold,
		WindowSeconds: cfg.Security.AutoBan404.WindowSeconds,
	}, func(ip string) {
		now := time.Now()

		// Escalation tier from the persisted strike counter; strikes decay
		// after 30 days of good behaviour.
		var strike db.BanStrike
		fresh := false
		if err := database.Where("ip = ?", ip).First(&strike).Error; err != nil {
			strike = db.BanStrike{IP: ip}
			fresh = true
		} else if now.Sub(strike.LastAt) > 30*24*time.Hour {
			strike.Strikes = 0
		}
		strike.Strikes++
		strike.LastAt = now

		// A manual deny (no expiry or still active) already covers this IP.
		var active db.IPListEntry
		database.Where("kind = ? AND value = ? AND (expires_at IS NULL OR expires_at > ?)", "deny", ip, now).First(&active)
		if active.ID != uuid.Nil && active.Source != "auto_ban" {
			if fresh {
				database.Create(&strike)
			} else {
				database.Save(&strike)
			}
			return
		}

		var exp *time.Time
		var tier string
		switch {
		case strike.Strikes <= 1:
			t := now.Add(firstBanDur)
			exp = &t
			tier = "Stufe 1 (" + cfg.Security.AutoBan404.FirstBan + ")"
			time.AfterFunc(firstBanDur+2*time.Second, func() { ps.AutoBan.Forget(ip) })
		case strike.Strikes == 2:
			t := now.Add(secondBanDur)
			exp = &t
			tier = "Stufe 2 (" + cfg.Security.AutoBan404.SecondBan + ")"
			time.AfterFunc(secondBanDur+2*time.Second, func() { ps.AutoBan.Forget(ip) })
		default:
			tier = "Stufe " + strconv.Itoa(strike.Strikes) + " (dauerhaft)"
		}

		// Replace previous auto-ban rows for this IP, then insert the new tier.
		database.Where("kind = ? AND value = ? AND source = ?", "deny", ip, "auto_ban").Delete(&db.IPListEntry{})
		if err := database.Create(&db.IPListEntry{Kind: "deny", Value: ip, Note: "404 – auto (" + tier + ")", ExpiresAt: exp, Source: "auto_ban"}).Error; err != nil {
			log.Printf("auto-ban %s: %v", ip, err)
			return
		}
		if fresh {
			if err := database.Create(&strike).Error; err != nil {
				database.Model(&db.BanStrike{}).Where("ip = ?", ip).Updates(map[string]interface{}{"strikes": strike.Strikes, "last_at": strike.LastAt})
			}
		} else {
			database.Save(&strike)
		}
		_ = database.Create(&db.AuditLog{
			Action:     "auto_ban",
			Resource:   "ip",
			ResourceID: ip,
			Method:     "AUTO",
			Path:       "/",
			Status:     403,
			IP:         ip,
			Details:    "Auto-Ban " + tier + " nach wiederholten 404-Antworten",
		}).Error
		if err := ipL.Reload(); err != nil {
			log.Printf("auto-ban reload ip list: %v", err)
		}
		log.Printf("auto-ban: blocked %s – %s", ip, tier)
	})
	go func() {
		if err := ps.Start(); err != nil {
			log.Fatalf("proxy: %v", err)
		}
	}()

	// API server
	go func() {
		runRenewal := func() {
			var certs []db.Certificate
			cutoff := time.Now().Add(30 * 24 * time.Hour)
			if err := database.Where("source = ? AND not_after IS NOT NULL AND not_after < ?", "letsencrypt", cutoff).Find(&certs).Error; err != nil {
				return
			}
			for _, c := range certs {
				tenant := ""
				if c.TenantID != uuid.Nil {
					tenant = c.TenantID.String()
				}
				domains := cm.SANsFromPEM(c.CertPEM)
				if len(domains) == 0 {
					domains = []string{c.Domain}
				}
				if err := cm.IssueLetsEncrypt(domains, c.Email, tenant, ""); err != nil {
					log.Printf("renew %s: %v", c.Domain, err)
				} else {
					log.Printf("renewed Let's Encrypt cert for %s", c.Domain)
				}
			}
		}
		time.Sleep(2 * time.Minute)
		runRenewal()
		ticker := time.NewTicker(12 * time.Hour)
		for range ticker.C {
			runRenewal()
		}
	}()

	router := api.NewRouter(api.Deps{
		DB:            database,
		JWTSecret:     cfg.Auth.JWTSecret,
		SessionSecret: cfg.Auth.SessionSecret,
		Certs:         cm,
		CrowdSec:   cs,
		Log:        accessLog,
		AdminHosts: adminHosts,
		IPList:     ipL,
		Cache:      respCache,
		Access:     access,
		CORSOrigins: cfg.Server.CORSOrigins,
		AllowPrivateUpstream: cfg.Security.Upstream.AllowPrivate,
		AllowedUpstreamHosts: cfg.Security.Upstream.AllowedHosts,
		AdminAccess: adminAccess,
		GeoIP:       g,
		APIKeys:     apiKeys,
	})
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
		<-sig
		accessLog.Close()
		os.Exit(0)
	}()

	addr := cfg.Server.APIAddr
	log.Printf("Bastion API listening on %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("serve: %v", err)
	}
}

func localIPs() []string {
	var ips []string
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ips
	}
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ip4 := ipnet.IP.To4(); ip4 != nil {
				ips = append(ips, ip4.String())
			}
		}
	}
	return ips
}
