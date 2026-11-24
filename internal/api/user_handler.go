package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/inboxes-cig/bastion/internal/auth"
	"github.com/inboxes-cig/bastion/internal/db"
	"gorm.io/gorm"
)

type UserHandler struct {
	DB *gorm.DB
}

func (h *UserHandler) List(c *gin.Context) {
	cl := auth.ClaimsFromContext(c)
	var users []db.User
	q := h.DB.Select("id, tenant_id, email, role_id, active, last_login, created_at")
	if cl != nil && cl.RoleName != "superadmin" && cl.TenantID != nil {
		q = q.Where("tenant_id = ?", *cl.TenantID)
	}
	if err := q.Find(&users).Error; err != nil {
		InternalError(c, err.Error())
		return
	}
	OK(c, users)
}

type userRequest struct {
	Email    string     `json:"email" binding:"required,email"`
	Password string     `json:"password"`
	RoleID   uuid.UUID  `json:"role_id" binding:"required"`
	TenantID *uuid.UUID `json:"tenant_id"`
	ClearTenant bool     `json:"clear_tenant"`
	Active   *bool      `json:"active"`
}

func (h *UserHandler) Create(c *gin.Context) {
	var req userRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		InternalError(c, err.Error())
		return
	}
	if req.Password == "" {
		hash, _ = auth.HashPassword(generateTempPassword())
	}
	u := db.User{
		TenantID:     req.TenantID,
		Email:        req.Email,
		PasswordHash: hash,
		RoleID:       req.RoleID,
		Active:       true,
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
	OK(c, u)
}

func (h *UserHandler) Update(c *gin.Context) {
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
	var req userRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
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
		if err == nil {
			u.PasswordHash = hh
		}
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
	if err := h.DB.Delete(&db.User{}, "id = ?", id).Error; err != nil {
		InternalError(c, err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

func generateTempPassword() string {
	return "TempPass123!"
}
