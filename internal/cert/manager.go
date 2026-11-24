package cert

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/caddyserver/certmagic"
	"github.com/google/uuid"
	"github.com/inboxes-cig/bastion/internal/db"
	"gorm.io/gorm"
)

type Manager struct {
	DB      *gorm.DB
	mu       sync.RWMutex
	cache    map[string]*tls.Certificate
	Default  *tls.Certificate
	leCfg    *certmagic.Config
	leIssuer *certmagic.ACMEIssuer
}

func NewManager(d *gorm.DB) *Manager {
	return &Manager{DB: d, cache: map[string]*tls.Certificate{}}
}

func (m *Manager) LoadAll() error {
	var certs []db.Certificate
	if err := m.DB.Order("source DESC").Find(&certs).Error; err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range certs {
		c, err := tls.X509KeyPair([]byte(certs[i].CertPEM), []byte(certs[i].KeyPEM))
		if err != nil {
			continue
		}
		names := []string{certs[i].Domain}
		if len(c.Certificate) > 0 {
			if leaf, err := x509.ParseCertificate(c.Certificate[0]); err == nil {
				names = append(names, leaf.DNSNames...)
				for _, ip := range leaf.IPAddresses {
					names = append(names, ip.String())
				}
			}
		}
		for _, n := range names {
			cc := c
			m.cache[strings.ToLower(n)] = &cc
		}
	}
	return nil
}

func (m *Manager) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if hello.ServerName != "" {
		name := strings.ToLower(hello.ServerName)
		if c, ok := m.cache[name]; ok {
			return c, nil
		}
		// Wildcard fallback: *.example.com matches foo.example.com
		if i := strings.IndexByte(name, '.'); i > 0 {
			if c, ok := m.cache["*"+name[i:]]; ok {
				return c, nil
			}
		}
	}
	if m.Default != nil {
		return m.Default, nil
	}
	return nil, fmt.Errorf("no certificate for %s", hello.ServerName)
}

func (m *Manager) AddManual(domain, certPEM, keyPEM string, tenantID string) error {
	if _, err := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM)); err != nil {
		return fmt.Errorf("invalid cert/key: %w", err)
	}
	block, _ := pem.Decode([]byte(certPEM))
	var notBefore, notAfter *time.Time
	var issuer string
	var leaf *x509.Certificate
	if block != nil {
		if c, err := x509.ParseCertificate(block.Bytes); err == nil {
			leaf = c
			nb := c.NotBefore
			na := c.NotAfter
			notBefore = &nb
			notAfter = &na
			issuer = c.Issuer.CommonName
		}
	}
	cert := db.Certificate{
		Domain:    domain,
		CertPEM:   certPEM,
		KeyPEM:    keyPEM,
		Issuer:    issuer,
		NotBefore: notBefore,
		NotAfter:  notAfter,
		Source:    "manual",
	}
	if tenantID != "" {
		if tid, err := uuid.Parse(tenantID); err == nil {
			cert.TenantID = tid
		}
	}
	if err := m.DB.Create(&cert).Error; err != nil {
		return err
	}
	c, _ := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM))
	names := []string{domain}
	if leaf != nil {
		names = append(names, leaf.DNSNames...)
		for _, ip := range leaf.IPAddresses {
			names = append(names, ip.String())
		}
	}
	m.mu.Lock()
	for _, n := range names {
		cc := c
		m.cache[strings.ToLower(n)] = &cc
	}
	m.mu.Unlock()
	return nil
}

func (m *Manager) Delete(domain string) error {
	var certs []db.Certificate
	m.DB.Where("domain = ?", domain).Find(&certs)
	for _, c := range certs {
		_ = m.DeleteByID(c.ID)
	}
	return nil
}

// DeleteByID removes a certificate, all of its SANs from the cache, and clears
// any site references so the site does not point to a deleted cert.
func (m *Manager) DeleteByID(id uuid.UUID) error {
	var cert db.Certificate
	if err := m.DB.First(&cert, "id = ?", id).Error; err != nil {
		return err
	}
	names := m.SANsFromPEM(cert.CertPEM)
	if len(names) == 0 {
		names = []string{cert.Domain}
	}
	names = append(names, cert.Domain)

	m.mu.Lock()
	for _, n := range names {
		delete(m.cache, strings.ToLower(n))
	}
	m.mu.Unlock()

	m.DB.Model(&db.Site{}).Where("cert_id = ?", cert.ID).Update("cert_id", nil)
	return m.DB.Delete(&db.Certificate{}, "id = ?", id).Error
}

func (m *Manager) GenerateSelfSigned(hosts []string) (*tls.Certificate, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: hosts[0]},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
		} else {
			tmpl.DNSNames = append(tmpl.DNSNames, h)
		}
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		return nil, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyBytes, _ := x509.MarshalECPrivateKey(priv)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}

	dbCert := db.Certificate{
		Domain:    hosts[0],
		CertPEM:   string(certPEM),
		KeyPEM:    string(keyPEM),
		Issuer:    "self-signed",
		NotBefore: &tmpl.NotBefore,
		NotAfter:  &tmpl.NotAfter,
		Source:    "manual",
	}
	m.DB.Where("domain = ?", hosts[0]).Delete(&db.Certificate{})
	m.DB.Create(&dbCert)

	m.mu.Lock()
	for _, h := range hosts {
		key := strings.ToLower(h)
		if _, ok := m.cache[key]; !ok {
			cc := cert
			m.cache[key] = &cc
		}
	}
	m.Default = &cert
	m.mu.Unlock()
	return &cert, nil
}

func (m *Manager) BootstrapAdmin(hosts []string) error {
	var existing db.Certificate
	if err := m.DB.Where("domain = ? AND issuer = ?", hosts[0], "self-signed").First(&existing).Error; err == nil {
		c, err := tls.X509KeyPair([]byte(existing.CertPEM), []byte(existing.KeyPEM))
		if err == nil {
			m.mu.Lock()
			for _, h := range hosts {
				key := strings.ToLower(h)
				if _, ok := m.cache[key]; !ok {
					cc := c
					m.cache[key] = &cc
				}
			}
			m.Default = &c
			m.mu.Unlock()
			return nil
		}
	}
	_, err := m.GenerateSelfSigned(hosts)
	return err
}
