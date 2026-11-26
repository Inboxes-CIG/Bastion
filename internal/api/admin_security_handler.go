package api

import (
	"fmt"
	"net"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/inboxes-cig/bastion/internal/adminaccess"
	"github.com/inboxes-cig/bastion/internal/audit"
	"github.com/inboxes-cig/bastion/internal/db"
	"github.com/inboxes-cig/bastion/internal/geoip"
	"github.com/inboxes-cig/bastion/internal/iplist"
	"gorm.io/gorm"
)

type AdminSecurityHandler struct {
	DB     *gorm.DB
	Access *adminaccess.Cache
	GeoIP  *geoip.Lookup
}

type adminIPRequest struct {
	Kind  string `json:"kind" binding:"required"`
	Value string `json:"value" binding:"required"`
	Note  string `json:"note"`
}

func (h *AdminSecurityHandler) ListIPRules(c *gin.Context) {
	if !requireSuperadmin(c) {
		return
	}
	var rules []db.AdminIPRule
	if err := h.DB.Order("created_at DESC").Find(&rules).Error; err != nil {
		InternalError(c, err.Error())
		return
	}
	OK(c, rules)
}

func (h *AdminSecurityHandler) CreateIPRule(c *gin.Context) {
	if !requireSuperadmin(c) {
		return
	}
	var req adminIPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}
	if req.Kind != "allow" && req.Kind != "deny" {
		BadRequest(c, "kind must be allow or deny")
		return
	}
	if err := iplist.Validate(req.Value); err != nil {
		BadRequest(c, err.Error())
		return
	}
	r := db.AdminIPRule{Kind: req.Kind, Value: strings.TrimSpace(req.Value), Note: req.Note}
	if err := h.DB.Create(&r).Error; err != nil {
		BadRequest(c, err.Error())
		return
	}
	if !h.guard(c, func() { h.DB.Unscoped().Delete(&db.AdminIPRule{}, "id = ?", r.ID) }) {
		return
	}
	audit.SetDetail(c, "GUI-Regel "+r.Kind+" "+r.Value+" angelegt")
	Created(c, r)
}

func (h *AdminSecurityHandler) UpdateIPRule(c *gin.Context) {
	if !requireSuperadmin(c) {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		BadRequest(c, "invalid id")
		return
	}
	var r db.AdminIPRule
	if err := h.DB.First(&r, "id = ?", id).Error; err != nil {
		NotFound(c, "entry not found")
		return
	}
	var req adminIPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}
	if req.Kind != "allow" && req.Kind != "deny" {
		BadRequest(c, "kind must be allow or deny")
		return
	}
	if err := iplist.Validate(req.Value); err != nil {
		BadRequest(c, err.Error())
		return
	}
	old := r
	r.Kind = req.Kind
	r.Value = strings.TrimSpace(req.Value)
	r.Note = req.Note
	if err := h.DB.Save(&r).Error; err != nil {
		InternalError(c, err.Error())
		return
	}
	if !h.guard(c, func() {
		h.DB.Model(&db.AdminIPRule{}).Where("id = ?", old.ID).
			Updates(map[string]interface{}{"kind": old.Kind, "value": old.Value, "note": old.Note})
	}) {
		return
	}
	audit.SetDetail(c, "GUI-Regel "+r.Kind+" "+r.Value+" geändert")
	OK(c, r)
}

func (h *AdminSecurityHandler) DeleteIPRule(c *gin.Context) {
	if !requireSuperadmin(c) {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		BadRequest(c, "invalid id")
		return
	}
	var r db.AdminIPRule
	if err := h.DB.First(&r, "id = ?", id).Error; err != nil {
		NotFound(c, "entry not found")
		return
	}
	if err := h.DB.Delete(&db.AdminIPRule{}, "id = ?", id).Error; err != nil {
		InternalError(c, err.Error())
		return
	}
	if !h.guard(c, func() { h.DB.Create(&r) }) {
		return
	}
	audit.SetDetail(c, "GUI-Regel "+r.Kind+" "+r.Value+" gelöscht")
	c.Status(204)
}

type adminGeoRequest struct {
	Enabled   bool     `json:"enabled"`
	Mode      string   `json:"mode"`
	Countries []string `json:"countries"`
}

func (h *AdminSecurityHandler) GetGeo(c *gin.Context) {
	if !requireSuperadmin(c) {
		return
	}
	var p db.AdminGeoPolicy
	if err := h.DB.Order("created_at ASC").First(&p).Error; err != nil {
		OK(c, gin.H{"enabled": false, "mode": "block", "countries": []string{}})
		return
	}
	OK(c, p)
}

func (h *AdminSecurityHandler) SetGeo(c *gin.Context) {
	if !requireSuperadmin(c) {
		return
	}
	var req adminGeoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}
	if req.Mode != "allow" && req.Mode != "block" {
		BadRequest(c, "mode must be allow or block")
		return
	}
	countries := db.StringArray{}
	seen := map[string]bool{}
	for _, cc := range req.Countries {
		cc = strings.ToUpper(strings.TrimSpace(cc))
		if len(cc) == 2 && !seen[cc] {
			seen[cc] = true
			countries = append(countries, cc)
		}
	}

	var p db.AdminGeoPolicy
	existing := h.DB.Order("created_at ASC").First(&p).Error == nil
	old := p
	p.Enabled = req.Enabled
	p.Mode = req.Mode
	p.Countries = countries
	if err := h.DB.Save(&p).Error; err != nil {
		InternalError(c, err.Error())
		return
	}
	revert := func() {
		if !existing {
			h.DB.Unscoped().Delete(&db.AdminGeoPolicy{}, "id = ?", p.ID)
			return
		}
		h.DB.Model(&db.AdminGeoPolicy{}).Where("id = ?", old.ID).
			Updates(map[string]interface{}{"enabled": old.Enabled, "mode": old.Mode, "countries": old.Countries})
	}
	if !h.guard(c, revert) {
		return
	}
	audit.SetDetail(c, fmt.Sprintf("GUI-GeoIP %s, Länder: %s", req.Mode, strings.Join(countries, ", ")))
	OK(c, p)
}

// guard reloads the live policy and verifies that the acting admin still has
// access; otherwise the change is reverted so nobody can lock themselves out.
func (h *AdminSecurityHandler) guard(c *gin.Context, revert func()) bool {
	if h.Access == nil {
		return true
	}
	if err := h.Access.Reload(); err != nil {
		revert()
		_ = h.Access.Reload()
		InternalError(c, err.Error())
		return false
	}
	if ok, _ := h.Access.Check(requestClientIP(c), h.GeoIP); !ok {
		revert()
		_ = h.Access.Reload()
		BadRequest(c, "diese Änderung würde deinen eigenen GUI-Zugriff sperren")
		return false
	}
	return true
}

func requestClientIP(c *gin.Context) string {
	if xff := c.GetHeader("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			xff = xff[:i]
		}
		if ip := strings.TrimSpace(xff); ip != "" {
			return ip
		}
	}
	if host, _, err := net.SplitHostPort(c.Request.RemoteAddr); err == nil {
		return host
	}
	return c.Request.RemoteAddr
}
