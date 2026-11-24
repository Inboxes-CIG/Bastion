package api

import (
	"encoding/base64"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp/totp"
	"github.com/skip2/go-qrcode"
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
	if user.MFAEnabled && user.MFASecret != "" {
		mfaToken, err := auth.GenerateMFAToken(h.JWTSecret, user.ID)
		if err != nil {
			InternalError(c, "mfa token failed")
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": gin.H{"mfa_required": true, "mfa_token": mfaToken}})
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
		"tenant_id":   user.TenantID,
		"active":      user.Active,
		"mfa_enabled": user.MFAEnabled,
	})
}

type mfaLoginRequest struct {
	MFAToken string `json:"mfa_token" binding:"required"`
	Code     string `json:"code" binding:"required"`
}

func (h *AuthHandler) MFALogin(c *gin.Context) {
	var req mfaLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}
	userID, err := auth.ValidateMFAToken(h.JWTSecret, req.MFAToken)
	if err != nil {
		Unauthorized(c, "invalid or expired mfa token")
		return
	}
	var user db.User
	if err := h.DB.First(&user, "id = ? AND active = true", userID).Error; err != nil {
		Unauthorized(c, "user not found")
		return
	}
	if !user.MFAEnabled || user.MFASecret == "" {
		BadRequest(c, "mfa not enabled")
		return
	}
	secret, err := auth.DecryptSecret(h.JWTSecret, user.MFASecret)
	if err != nil {
		InternalError(c, "mfa secret error")
		return
	}
	if !totp.Validate(req.Code, secret) {
		audit.Login(h.DB, user.Email, c.ClientIP(), false, nil, nil)
		Unauthorized(c, "invalid code")
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
	audit.Login(h.DB, user.Email, c.ClientIP(), true, &user.ID, user.TenantID)
	h.DB.Model(&user).Update("last_login", gorm.Expr("NOW()"))
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"tokens": tokens,
			"user": gin.H{
				"id":        user.ID,
				"email":     user.Email,
				"role":      role.Name,
				"tenant_id": user.TenantID,
			},
		},
	})
}

type mfaCodeRequest struct {
	Code string `json:"code" binding:"required"`
}

func (h *AuthHandler) MFASetup(c *gin.Context) {
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
	if user.MFAEnabled {
		BadRequest(c, "MFA ist bereits aktiviert")
		return
	}
	key, err := totp.Generate(totp.GenerateOpts{Issuer: "Bastion", AccountName: user.Email})
	if err != nil {
		InternalError(c, err.Error())
		return
	}
	enc, err := auth.EncryptSecret(h.JWTSecret, key.Secret())
	if err != nil {
		InternalError(c, err.Error())
		return
	}
	if err := h.DB.Model(&user).Update("mfa_secret", enc).Error; err != nil {
		InternalError(c, err.Error())
		return
	}
	png, err := qrcode.Encode(key.URL(), qrcode.Medium, 256)
	if err != nil {
		InternalError(c, err.Error())
		return
	}
	OK(c, gin.H{
		"secret":      key.Secret(),
		"otpauth_url": key.URL(),
		"qr_data_url": "data:image/png;base64," + base64.StdEncoding.EncodeToString(png),
	})
}

func (h *AuthHandler) MFAEnable(c *gin.Context) {
	var req mfaCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}
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
	if user.MFAEnabled {
		BadRequest(c, "MFA ist bereits aktiviert")
		return
	}
	if user.MFASecret == "" {
		BadRequest(c, "bitte zuerst die MFA-Einrichtung starten")
		return
	}
	secret, err := auth.DecryptSecret(h.JWTSecret, user.MFASecret)
	if err != nil {
		InternalError(c, "mfa secret error")
		return
	}
	if !totp.Validate(req.Code, secret) {
		Unauthorized(c, "invalid code")
		return
	}
	if err := h.DB.Model(&user).Update("mfa_enabled", true).Error; err != nil {
		InternalError(c, err.Error())
		return
	}
	OK(c, gin.H{"mfa_enabled": true})
}

func (h *AuthHandler) MFADisable(c *gin.Context) {
	var req mfaCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}
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
	if !user.MFAEnabled {
		BadRequest(c, "MFA ist nicht aktiviert")
		return
	}
	secret, err := auth.DecryptSecret(h.JWTSecret, user.MFASecret)
	if err != nil {
		InternalError(c, "mfa secret error")
		return
	}
	if !totp.Validate(req.Code, secret) {
		Unauthorized(c, "invalid code")
		return
	}
	h.DB.Model(&user).Updates(map[string]interface{}{"mfa_enabled": false, "mfa_secret": ""})
	OK(c, gin.H{"mfa_enabled": false})
}
