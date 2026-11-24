package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/inboxes-cig/bastion/internal/audit"
	"github.com/inboxes-cig/bastion/internal/auth"
	"github.com/inboxes-cig/bastion/internal/db"
	"gorm.io/gorm"
)

type AuthHandler struct {
	DB        *gorm.DB
	JWTSecret string
}

type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}
	var user db.User
	if err := h.DB.Where("email = ? AND active = true", req.Email).First(&user).Error; err != nil {
		audit.Login(h.DB, req.Email, c.ClientIP(), false, nil, nil)
		Unauthorized(c, "invalid credentials")
		return
	}
	if !auth.CheckPassword(user.PasswordHash, req.Password) {
		audit.Login(h.DB, req.Email, c.ClientIP(), false, nil, nil)
		Unauthorized(c, "invalid credentials")
		return
	}
	audit.Login(h.DB, user.Email, c.ClientIP(), true, &user.ID, user.TenantID)

	var role db.Role
	if err := h.DB.First(&role, "id = ?", user.RoleID).Error; err != nil {
		InternalError(c, "role not found")
		return
	}

	tokens, err := auth.GenerateTokens(h.JWTSecret, user.ID, user.TenantID, user.RoleID, role.Name)
	if err != nil {
		InternalError(c, "token generation failed")
		return
	}

	h.DB.Model(&user).Update("last_login", gorm.Expr("NOW()"))

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"tokens": tokens,
			"user": gin.H{
				"id":       user.ID,
				"email":    user.Email,
				"role":     role.Name,
				"tenant_id": user.TenantID,
			},
		},
	})
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}
	claims, err := auth.ValidateToken(h.JWTSecret, req.RefreshToken)
	if err != nil {
		Unauthorized(c, "invalid refresh token")
		return
	}
	var user db.User
	if err := h.DB.First(&user, "id = ?", claims.UserID).Error; err != nil {
		Unauthorized(c, "user not found")
		return
	}
	var role db.Role
	if err := h.DB.First(&role, "id = ?", user.RoleID).Error; err != nil {
		InternalError(c, "role not found")
		return
	}
	tokens, err := auth.GenerateTokens(h.JWTSecret, user.ID, user.TenantID, user.RoleID, role.Name)
	if err != nil {
		InternalError(c, "token generation failed")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": tokens})
}

func (h *AuthHandler) Me(c *gin.Context) {
	cl := auth.ClaimsFromContext(c)
	if cl == nil {
		Unauthorized(c, "unauthorized")
		return
	}
	var user db.User
	if err := h.DB.First(&user, "id = ?", cl.UserID).Error; err != nil {
		NotFound(c, "user not found")
		return
	}
	OK(c, gin.H{
		"id":        user.ID,
		"email":     user.Email,
		"role":      cl.RoleName,
		"tenant_id": user.TenantID,
		"active":    user.Active,
	})
}
