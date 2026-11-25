package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/inboxes-cig/bastion/internal/audit"
	"github.com/inboxes-cig/bastion/internal/auth"
	"github.com/inboxes-cig/bastion/internal/cert"
	"github.com/inboxes-cig/bastion/internal/db"
	"gorm.io/gorm"
)

type CertHandler struct {
	DB    *gorm.DB
	Certs *cert.Manager
}

type certRequest struct {
	Domain   string `json:"domain" binding:"required"`
	CertPEM  string `json:"cert_pem" binding:"required"`
	KeyPEM   string `json:"key_pem" binding:"required"`
	TenantID string `json:"tenant_id"`
}

func (h *CertHandler) List(c *gin.Context) {
	var certs []db.Certificate
	q := tenantScoped(c, h.DB.Select("id, tenant_id, domain, issuer, not_before, not_after, auto_renew, source, created_at"), "tenant_id")
	if err := q.Find(&certs).Error; err != nil {
		InternalError(c, err.Error())
		return
	}
	OK(c, certs)
}

func (h *CertHandler) Upload(c *gin.Context) {
	var req certRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}
	cl := auth.ClaimsFromContext(c)
	if cl == nil {
		Unauthorized(c, "unauthorized")
		return
	}
	if cl.RoleName != "superadmin" {
		if cl.TenantID == nil {
			Forbidden(c, "forbidden")
			return
		}
		req.TenantID = cl.TenantID.String()
	}
	if err := h.Certs.AddManual(req.Domain, req.CertPEM, req.KeyPEM, req.TenantID); err != nil {
		BadRequest(c, err.Error())
		return
	}
	Created(c, gin.H{"domain": req.Domain, "status": "uploaded"})
}

type issueRequest struct {
	SiteID   string   `json:"site_id"`
	Domain   string   `json:"domain"`
	Domains  []string `json:"domains"`
	Email    string   `json:"email" binding:"required,email"`
	TenantID string   `json:"tenant_id"`
}

func (h *CertHandler) Issue(c *gin.Context) {
	var req issueRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}
	cl := auth.ClaimsFromContext(c)
	if cl == nil {
		Unauthorized(c, "unauthorized")
		return
	}
	tenantID := req.TenantID
	if cl.RoleName != "superadmin" {
		if cl.TenantID == nil {
			Forbidden(c, "forbidden")
			return
		}
		tenantID = cl.TenantID.String()
	}

	var domains []string
	siteID := req.SiteID
	if siteID != "" {
		var site db.Site
		if err := h.DB.First(&site, "id = ?", siteID).Error; err != nil {
			NotFound(c, "site not found")
			return
		}
		if !canAccessTenant(c, &site.TenantID) {
			NotFound(c, "site not found")
			return
		}
		domains = append(domains, site.Domain)
		for _, d := range site.ExtraDomains {
			domains = append(domains, d)
		}
		if tenantID == "" {
			tenantID = site.TenantID.String()
		}
	} else if len(req.Domains) > 0 {
		domains = req.Domains
	} else if req.Domain != "" {
		domains = []string{req.Domain}
	}
	if len(domains) == 0 {
		BadRequest(c, "no domains provided")
		return
	}

	valid := cert.FilterPublicDomains(domains)
	excluded := []string{}
	validSet := map[string]bool{}
	for _, d := range valid {
		validSet[d] = true
	}
	for _, d := range domains {
		if !validSet[d] {
			excluded = append(excluded, d)
		}
	}
	if len(valid) == 0 {
		BadRequest(c, "keine öffentlich gültige Domain (nur .local/.test/IP?)")
		return
	}

	if err := h.Certs.IssueLetsEncrypt(valid, req.Email, tenantID, siteID); err != nil {
		InternalError(c, err.Error())
		return
	}
	OK(c, gin.H{"domains": valid, "excluded": excluded, "status": "issued"})
}

func (h *CertHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		BadRequest(c, "invalid id")
		return
	}
	var cert db.Certificate
	if err := h.DB.First(&cert, "id = ?", id).Error; err != nil {
		NotFound(c, "cert not found")
		return
	}
	if !canAccessTenant(c, &cert.TenantID) {
		NotFound(c, "cert not found")
		return
	}
	if err := h.Certs.DeleteByID(cert.ID); err != nil {
		InternalError(c, err.Error())
		return
	}
	audit.SetDetail(c, "Zertifikat \""+cert.Domain+"\" gelöscht")
	c.Status(http.StatusNoContent)
}

func (h *CertHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		BadRequest(c, "invalid id")
		return
	}
	var cert db.Certificate
	if err := h.DB.Select("id, tenant_id, domain, issuer, not_before, not_after, auto_renew, source, created_at").First(&cert, "id = ?", id).Error; err != nil {
		NotFound(c, "cert not found")
		return
	}
	if !canAccessTenant(c, &cert.TenantID) {
		NotFound(c, "cert not found")
		return
	}
	OK(c, cert)
}
