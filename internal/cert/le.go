package cert

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/caddyserver/certmagic"
	"github.com/google/uuid"
	"github.com/inboxes-cig/bastion/internal/db"
)

// InitLetsEncrypt builds a single certmagic config + ACME issuer shared by both
// certificate issuance and the HTTP-01 challenge handler.
func (m *Manager) InitLetsEncrypt() {
	if m.leCfg != nil {
		return
	}
	certmagic.DefaultACME.Agreed = true

	cfg := certmagic.NewDefault()
	cfg.Storage = &certmagic.FileStorage{Path: "/var/lib/bastion/certs/acme"}

	issuer := certmagic.NewACMEIssuer(cfg, certmagic.DefaultACME)
	issuer.Agreed = true
	issuer.DisableTLSALPNChallenge = true
	cfg.Issuers = []certmagic.Issuer{issuer}

	m.leCfg = cfg
	m.leIssuer = issuer
}

func normalizeDomains(domains []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(domains))
	for _, d := range domains {
		d = strings.TrimSpace(strings.ToLower(d))
		if d == "" || seen[d] {
			continue
		}
		seen[d] = true
		out = append(out, d)
	}
	return out
}

// FilterPublicDomains drops names that can never receive a public certificate
// (e.g. .local, .test, localhost, private IPs).
func FilterPublicDomains(domains []string) []string {
	out := make([]string, 0, len(domains))
	for _, d := range normalizeDomains(domains) {
		if certmagic.SubjectQualifiesForPublicCert(d) {
			out = append(out, d)
		}
	}
	return out
}

// IssueLetsEncrypt obtains a multi-SAN certificate for all domains and stores
// the full chain. If siteID is set, the certificate is associated with the site
// and the site's cert_id is updated (keeping a stable certificate row on renew).
func (m *Manager) IssueLetsEncrypt(domains []string, email, tenantID, siteID string) error {
	m.InitLetsEncrypt()
	hosts := normalizeDomains(domains)
	if len(hosts) == 0 {
		return fmt.Errorf("no domains provided")
	}
	if email != "" {
		m.leIssuer.Email = email
		certmagic.DefaultACME.Email = email
	}

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate key: %w", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return fmt.Errorf("marshal key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	tmpl := &x509.CertificateRequest{}
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
		} else {
			tmpl.DNSNames = append(tmpl.DNSNames, h)
		}
	}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, tmpl, priv)
	if err != nil {
		return fmt.Errorf("create csr: %w", err)
	}
	csr, err := x509.ParseCertificateRequest(csrDER)
	if err != nil {
		return fmt.Errorf("parse csr: %w", err)
	}

	ctx := context.Background()
	if err := m.leIssuer.PreCheck(ctx, hosts, true); err != nil {
		return fmt.Errorf("precheck: %w", err)
	}
	issued, err := m.leIssuer.Issue(ctx, csr)
	if err != nil {
		return fmt.Errorf("obtain: %w", err)
	}
	certPEM := issued.Certificate
	if len(certPEM) == 0 {
		return fmt.Errorf("empty certificate from issuer")
	}

	leafBlock, _ := pem.Decode(certPEM)
	if leafBlock == nil {
		return fmt.Errorf("invalid certificate PEM")
	}
	leaf, err := x509.ParseCertificate(leafBlock.Bytes)
	if err != nil {
		return fmt.Errorf("parse leaf: %w", err)
	}

	dbCert := db.Certificate{
		Domain:    hosts[0],
		CertPEM:   string(certPEM),
		KeyPEM:    string(keyPEM),
		Issuer:    "Let's Encrypt",
		Source:    "letsencrypt",
		AutoRenew: true,
		Email:     email,
	}
	nb, na := leaf.NotBefore, leaf.NotAfter
	dbCert.NotBefore = &nb
	dbCert.NotAfter = &na
	if tenantID != "" {
		if tid, err := uuid.Parse(tenantID); err == nil {
			dbCert.TenantID = tid
		}
	}

	var existing db.Certificate
	if err := m.DB.Where("domain = ? AND source = ?", hosts[0], "letsencrypt").First(&existing).Error; err == nil {
		existing.CertPEM = dbCert.CertPEM
		existing.KeyPEM = dbCert.KeyPEM
		existing.Issuer = dbCert.Issuer
		existing.NotBefore = dbCert.NotBefore
		existing.NotAfter = dbCert.NotAfter
		existing.Email = dbCert.Email
		existing.AutoRenew = true
		if dbCert.TenantID != uuid.Nil {
			existing.TenantID = dbCert.TenantID
		}
		if err := m.DB.Save(&existing).Error; err != nil {
			return fmt.Errorf("db save: %w", err)
		}
		dbCert = existing
	} else {
		if err := m.DB.Create(&dbCert).Error; err != nil {
			return fmt.Errorf("db create: %w", err)
		}
	}

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return fmt.Errorf("build tls cert: %w", err)
	}
	m.mu.Lock()
	for _, name := range leaf.DNSNames {
		c := tlsCert
		m.cache[strings.ToLower(name)] = &c
	}
	for _, ip := range leaf.IPAddresses {
		c := tlsCert
		m.cache[ip.String()] = &c
	}
	if len(leaf.DNSNames) == 0 && len(leaf.IPAddresses) == 0 {
		m.cache[hosts[0]] = &tlsCert
	}
	m.mu.Unlock()

	if siteID != "" {
		m.DB.Model(&db.Site{}).Where("id = ?", siteID).Update("cert_id", dbCert.ID)
	}
	return nil
}

// SANsFromPEM extracts DNS names and IPs from a stored certificate PEM.
func (m *Manager) SANsFromPEM(certPEM string) []string {
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		return nil
	}
	c, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil
	}
	out := append([]string{}, c.DNSNames...)
	for _, ip := range c.IPAddresses {
		out = append(out, ip.String())
	}
	return out
}

// HTTPChallengeHandler serves ACME HTTP-01 challenges from the same issuer
// used for issuance.
func (m *Manager) HTTPChallengeHandler(h http.Handler) http.Handler {
	m.InitLetsEncrypt()
	if h == nil {
		h = http.DefaultServeMux
	}
	if m.leIssuer == nil {
		return h
	}
	return m.leIssuer.HTTPChallengeHandler(h)
}
