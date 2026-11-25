package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/inboxes-cig/bastion/internal/audit"
	"github.com/inboxes-cig/bastion/internal/auth"
	"github.com/inboxes-cig/bastion/internal/db"
	"gorm.io/gorm"
)

type UserHandler struct {
	DB *gorm.DB
}

func (h *UserHandler) List(c *gin.Context) {
	var users []db.User
	q := tenantScoped(c, h.DB.Select("id, tenant_id, email, role_id, active, must_change_password, last_login, created_at"), "tenant_id")
	if err := q.Find(&users).Error; err != nil {
		InternalError(c, err.Error())
		return
	}
	OK(c, users)
}

type userRequest struct {
	Email          string     `json:"email" binding:"required,email"`
	Password       string     `json:"password"`
	RoleID         uuid.UUID  `json:"role_id" binding:"required"`
	TenantID       *uuid.UUID `json:"tenant_id"`
	ClearTenant    bool       `json:"clear_tenant"`
	Active         *bool      `json:"active"`
	ReauthPassword string     `json:"reauth_password"`
}

func (h *UserHandler) Create(c *gin.Context) {
	var req userRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}
	cl := auth.ClaimsFromContext(c)
	if cl == nil {
		Unauthorized(c, "unauthorized")
		return
	}
	if req.Password == "" {
		BadRequest(c, "password required")
		return
	}
	if req.RoleID == uuid.Nil {
		BadRequest(c, "role_id required")
		return
	}
	var role db.Role
	if err := h.DB.First(&role, "id = ?", req.RoleID).Error; err != nil {
		BadRequest(c, "invalid role_id")
		return
	}
	if cl.RoleName != "superadmin" {
		if cl.TenantID == nil {
			Forbidden(c, "forbidden")
			return
		}
		if role.Scope == "system" {
			Forbidden(c, "cannot assign a system role")
			return
		}
		if req.TenantID != nil && *req.TenantID != *cl.TenantID {
			Forbidden(c, "forbidden")
			return
		}
		req.TenantID = cl.TenantID
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		InternalError(c, err.Error())
		return
	}
	u := db.User{
		TenantID:           req.TenantID,
		Email:              req.Email,
		PasswordHash:       hash,
		RoleID:             req.RoleID,
		Active:             true,
		MustChangePassword: true,
	}
	if req.Active != nil {
		u.Active = *req.Active
	}
	if err := h.DB.Create(&u).Error; err != nil {
		BadRequest(c, err.Error())
		return
	}
	Created(c, u)
}

func (h *UserHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		BadRequest(c, "invalid id")
		return
	}
	var u db.User
	if err := h.DB.First(&u, "id = ?", id).Error; err != nil {
		NotFound(c, "user not found")
		return
	}
	if !canAccessTenant(c, u.TenantID) {
		NotFound(c, "user not found")
		return
	}
	OK(c, u)
}

func (h *UserHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		BadRequest(c, "invalid id")
		return
	}
	cl := auth.ClaimsFromContext(c)
	if cl == nil {
		Unauthorized(c, "unauthorized")
		return
	}
	var u db.User
	if err := h.DB.First(&u, "id = ?", id).Error; err != nil {
		NotFound(c, "user not found")
		return
	}
	if !canAccessTenant(c, u.TenantID) {
		NotFound(c, "user not found")
		return
	}
	if cl.RoleName != "superadmin" && u.RoleID != uuid.Nil {
		var targetRole db.Role
		if err := h.DB.Select("scope").First(&targetRole, "id = ?", u.RoleID).Error; err == nil && targetRole.Scope == "system" {
			NotFound(c, "user not found")
			return
		}
	}

	var req userRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}

	tenantChanged := req.ClearTenant || (req.TenantID != nil && (u.TenantID == nil || *req.TenantID != *u.TenantID))
	sensitive := req.Password != "" ||
		(req.RoleID != uuid.Nil && req.RoleID != u.RoleID) ||
		(req.Email != "" && req.Email != u.Email) ||
		tenantChanged ||
		(req.Active != nil && *req.Active != u.Active)
	if sensitive && !reauth(c, h.DB, req.ReauthPassword) {
		Forbidden(c, "re-authentication required")
		return
	}

	if cl.RoleName != "superadmin" {
		if tenantChanged {
			Forbidden(c, "forbidden")
			return
		}
		if req.RoleID != uuid.Nil {
			var role db.Role
			if err := h.DB.Select("scope").First(&role, "id = ?", req.RoleID).Error; err != nil {
				BadRequest(c, "invalid role_id")
				return
			}
			if role.Scope == "system" {
				Forbidden(c, "cannot assign a system role")
				return
			}
		}
		if req.Active != nil && !*req.Active && u.ID == cl.UserID {
			BadRequest(c, "cannot deactivate yourself")
			return
		}
	}

	if req.Email != "" {
		u.Email = req.Email
	}
	if req.RoleID != uuid.Nil {
		u.RoleID = req.RoleID
	}
	if req.ClearTenant {
		u.TenantID = nil
	} else if req.TenantID != nil {
		u.TenantID = req.TenantID
	}
	if req.Active != nil {
		u.Active = *req.Active
	}
	if req.Password != "" {
		hh, err := auth.HashPassword(req.Password)
		if err != nil {
			InternalError(c, err.Error())
			return
		}
		u.PasswordHash = hh
		// Admins resetting another user's password force a change on next login.
		u.MustChangePassword = u.ID != cl.UserID
	}
	if err := h.DB.Save(&u).Error; err != nil {
		InternalError(c, err.Error())
		return
	}
	OK(c, u)
}

func (h *UserHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		BadRequest(c, "invalid id")
		return
	}
	var u db.User
	if err := h.DB.First(&u, "id = ?", id).Error; err != nil {
		NotFound(c, "user not found")
		return
	}
	if !canAccessTenant(c, u.TenantID) {
		NotFound(c, "user not found")
		return
	}
	if !isSuperadmin(c) {
		var role db.Role
		if err := h.DB.Select("scope").First(&role, "id = ?", u.RoleID).Error; err == nil && role.Scope == "system" {
			NotFound(c, "user not found")
			return
		}
	}
	audit.SetDetail(c, fmt.Sprintf("Benutzer %q gelöscht", u.Email))
	if err := h.DB.Delete(&db.User{}, "id = ?", id).Error; err != nil {
		InternalError(c, err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}
