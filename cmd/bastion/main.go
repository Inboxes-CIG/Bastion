package main

import (
	"log"
	"net"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/inboxes-cig/bastion/internal/accesslog"
	"github.com/inboxes-cig/bastion/internal/api"
	"github.com/inboxes-cig/bastion/internal/autoban"
	"github.com/inboxes-cig/bastion/internal/cache"
	"github.com/inboxes-cig/bastion/internal/cert"
	"github.com/inboxes-cig/bastion/internal/config"
	"github.com/inboxes-cig/bastion/internal/crowdsec"
	"github.com/inboxes-cig/bastion/internal/db"
	"github.com/inboxes-cig/bastion/internal/geoip"
	"github.com/inboxes-cig/bastion/internal/iplist"
	"github.com/inboxes-cig/bastion/internal/proxy"
	"github.com/inboxes-cig/bastion/internal/ratelimit"
	"github.com/inboxes-cig/bastion/internal/siteaccess"
	"github.com/inboxes-cig/bastion/internal/system"
	"os"
	"os/signal"
	"syscall"
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

	autoBanEnabled := true
	if cfg.Security.AutoBan404.Enabled != nil {
		autoBanEnabled = *cfg.Security.AutoBan404.Enabled
	}
	ps.AutoBan = autoban.New(autoban.Config{
		Enabled:       autoBanEnabled,
		Threshold:     cfg.Security.AutoBan404.Threshold,
		WindowSeconds: cfg.Security.AutoBan404.WindowSeconds,
	}, func(ip string) {
		var existing db.IPListEntry
		if err := database.Where("kind = ? AND value = ?", "deny", ip).First(&existing).Error; err == nil {
			return
		}
		if err := database.Create(&db.IPListEntry{Kind: "deny", Value: ip, Note: "404"}).Error; err != nil {
			log.Printf("auto-ban %s: %v", ip, err)
			return
		}
		_ = database.Create(&db.AuditLog{
			Action:     "auto_ban",
			Resource:   "ip",
			ResourceID: ip,
			Method:     "AUTO",
			Path:       "/",
			Status:     403,
			IP:         ip,
			Details:    "blocked after repeated 404 responses",
		}).Error
		if err := ipL.Reload(); err != nil {
			log.Printf("auto-ban reload ip list: %v", err)
		}
		log.Printf("auto-ban: blocked %s after repeated 404 responses", ip)
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
		DB:         database,
		JWTSecret:  cfg.Auth.JWTSecret,
		Certs:      cm,
		CrowdSec:   cs,
		Log:        accessLog,
		AdminHosts: adminHosts,
		IPList:     ipL,
		Cache:      respCache,
		Access:     access,
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
