package audit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/inboxes-cig/bastion/internal/auth"
	"github.com/inboxes-cig/bastion/internal/db"
	"gorm.io/gorm"
)

const maxAuditBody = 1 << 20 // 1 MiB

func actionFor(method string) string {
	switch method {
	case "POST":
		return "create"
	case "PUT", "PATCH":
		return "update"
	case "DELETE":
		return "delete"
	}
	return strings.ToLower(method)
}

func resourceFor(path string) string {
	p := strings.TrimPrefix(path, "/api/")
	p = strings.TrimPrefix(p, "v1/")
	parts := strings.Split(p, "/")
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

// SetDetail lets a handler override the auto-generated audit description with
// a precise, human-readable sentence.
func SetDetail(c *gin.Context, detail string) {
	c.Set("audit_detail", detail)
}

// Middleware records mutating API requests into the audit log.
func Middleware(d *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		method := c.Request.Method
		track := method != "GET" && method != "HEAD" && method != "OPTIONS" &&
			strings.HasPrefix(path, "/api/") &&
			!strings.HasPrefix(path, "/api/v1/auth/login") &&
			!strings.HasPrefix(path, "/api/v1/auth/refresh")

		var body []byte
		if track && c.Request.Body != nil {
			b, _ := io.ReadAll(io.LimitReader(c.Request.Body, maxAuditBody+1))
			if len(b) <= maxAuditBody {
				body = b
				c.Request.Body = io.NopCloser(bytes.NewReader(b))
			} else {
				// Too large to keep for inspection: restore the full stream.
				c.Request.Body = io.NopCloser(io.MultiReader(bytes.NewReader(b), c.Request.Body))
			}
		}

		c.Next()
		if !track {
			return
		}

		e := db.AuditLog{
			Action:    actionFor(method),
			Resource:  resourceFor(path),
			Method:    method,
			Path:      path,
			Status:    c.Writer.Status(),
			IP:        c.ClientIP(),
			CreatedAt: time.Now(),
		}
		if v, ok := c.Get("audit_detail"); ok {
			if s, ok := v.(string); ok {
				e.Details = s
			}
		}
		if e.Details == "" {
			e.Details = describe(c, body)
		}
		if cl := auth.ClaimsFromContext(c); cl != nil {
			uid := cl.UserID
			e.UserID = &uid
			if cl.TenantID != nil {
				tid := *cl.TenantID
				e.TenantID = &tid
			}
			var u db.User
			if err := d.Select("email").First(&u, "id = ?", cl.UserID).Error; err == nil {
				e.UserEmail = u.Email
			}
		}
		_ = d.Create(&e).Error
	}
}

// describe builds a human-readable summary from the request path and JSON body.
func describe(c *gin.Context, body []byte) string {
	path := c.Request.URL.Path
	failed := c.Writer.Status() >= 400
	action := actionFor(c.Request.Method)

	switch {
	case strings.HasSuffix(path, "/auth/mfa/enable"):
		return final("MFA aktiviert", failed)
	case strings.HasSuffix(path, "/auth/mfa/disable"):
		return final("MFA deaktiviert", failed)
	case strings.HasSuffix(path, "/auth/mfa/setup"):
		return final("MFA-Einrichtung gestartet", failed)
	case strings.HasSuffix(path, "/cache/clear"):
		var req struct {
			SiteID string `json:"site_id"`
		}
		_ = json.Unmarshal(body, &req)
		if req.SiteID != "" {
			return final("Cache für Site "+req.SiteID+" geleert", failed)
		}
		return final("Cache geleert", failed)
	case strings.Contains(path, "/certificates/issue"):
		if v := bodyString(body, "domain"); v != "" {
			return final(fmt.Sprintf("Zertifikat für %q angefordert", v), failed)
		}
		return final("Zertifikat angefordert", failed)
	case strings.HasSuffix(path, "/certificates") && c.Request.Method == "POST":
		if v := bodyString(body, "domain"); v != "" {
			return final(fmt.Sprintf("Zertifikat %q hochgeladen", v), failed)
		}
	}

	label := resourceLabel(path)
	name := bodyString(body, "name", "domain", "email", "username", "value", "path_prefix", "country_code", "note", "site_id")
	if name == "" {
		name = firstParam(c, "lid", "uid", "rid", "id")
	}
	verb := actionVerb(action)
	if name == "" {
		if failed {
			return label + ": " + verb + " fehlgeschlagen"
		}
		return label + " " + pastTense(action)
	}
	if failed {
		return fmt.Sprintf("%s %q: %s fehlgeschlagen", label, name, verb)
	}
	return fmt.Sprintf("%s %q %s", label, name, pastTense(action))
}

func final(text string, failed bool) string {
	if failed {
		return text + " – fehlgeschlagen"
	}
	return text
}

func pastTense(action string) string {
	switch action {
	case "create":
		return "angelegt"
	case "update":
		return "geändert"
	case "delete":
		return "gelöscht"
	}
	return action
}

func actionVerb(action string) string {
	switch action {
	case "create":
		return "Anlegen"
	case "update":
		return "Ändern"
	case "delete":
		return "Löschen"
	}
	return action
}

func resourceLabel(path string) string {
	switch {
	case strings.Contains(path, "/locations"):
		return "Location"
	case strings.Contains(path, "/ip-rules"):
		return "Site-IP-Regel"
	case strings.Contains(path, "/sites/") && strings.Contains(path, "/users"):
		return "Site-Benutzer"
	case strings.Contains(path, "/good-actors"):
		return "Good Actor"
	case strings.Contains(path, "/crowdsec/decisions"):
		return "CrowdSec-Entscheidung"
	case strings.Contains(path, "/geoip/rules"):
		return "GeoIP-Regel"
	case strings.Contains(path, "/certificates"):
		return "Zertifikat"
	case strings.Contains(path, "/templates"):
		return "Template"
	case strings.Contains(path, "/tenants"):
		return "Tenant"
	case strings.Contains(path, "/users"):
		return "Benutzer"
	case strings.Contains(path, "/sites"):
		return "Site"
	case strings.Contains(path, "/iplist"):
		return "IP-Eintrag"
	case strings.Contains(path, "/cache"):
		return "Cache"
	case strings.Contains(path, "/auth"):
		return "Konto"
	}
	return "Ressource"
}

func bodyString(body []byte, keys ...string) string {
	if len(body) == 0 {
		return ""
	}
	var m map[string]any
	if json.Unmarshal(body, &m) != nil {
		return ""
	}
	for _, k := range keys {
		v, ok := m[k]
		if !ok {
			continue
		}
		switch t := v.(type) {
		case string:
			if strings.TrimSpace(t) != "" {
				return t
			}
		case float64:
			return strconv.FormatFloat(t, 'f', -1, 64)
		}
	}
	return ""
}

func firstParam(c *gin.Context, names ...string) string {
	for _, n := range names {
		if v := c.Param(n); v != "" {
			return v
		}
	}
	return ""
}

// Login records a successful or failed login attempt.
func Login(d *gorm.DB, email, ip string, success bool, userID, tenantID *uuid.UUID) {
	action := "login"
	detail := "Login erfolgreich"
	status := 200
	if !success {
		action = "login_failed"
		detail = "Login fehlgeschlagen"
		status = 401
	}
	_ = d.Create(&db.AuditLog{
		Action:    action,
		Resource:  "auth",
		Method:    "POST",
		Path:      "/api/v1/auth/login",
		Status:    status,
		IP:        ip,
		UserEmail: email,
		UserID:    userID,
		TenantID:  tenantID,
		Details:   detail,
		CreatedAt: time.Now(),
	}).Error
}
