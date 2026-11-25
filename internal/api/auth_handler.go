package api

import (
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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
	MFAKey    string
	Limiter   *auth.LoginLimiter
}

type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

func (h *AuthHandler) loginKeys(c *gin.Context, email string) []string {
	return []string{
		"login:ip:" + c.ClientIP(),
		"login:email:" + strings.ToLower(strings.TrimSpace(email)),
	}
}

func (h *AuthHandler) mfaKeys(c *gin.Context, userID uuid.UUID) []string {
	return []string{
		"mfa:ip:" + c.ClientIP(),
		"mfa:uid:" + userID.String(),
	}
}

func (h *AuthHandler) tooMany(c *gin.Context, remaining time.Duration) {
	secs := int(remaining.Seconds())
	if secs < 1 {
		secs = 1
	}
	c.Header("Retry-After", strconv.Itoa(secs))
	c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many attempts"})
}

func (h *AuthHandler) storeRefresh(userID uuid.UUID, tokens *auth.TokenPair) error {
	return h.DB.Create(&db.RefreshToken{
		UserID:    userID,
		JTI:       tokens.RefreshJTI,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}).Error
}

func (h *AuthHandler) revokeAllRefresh(userID uuid.UUID) {
	now := time.Now()
	h.DB.Model(&db.RefreshToken{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", now)
}

// rotateRefresh revokes the presented token and reports whether it was valid.
// A reused/rotated token revokes the whole token family for that user.
func (h *AuthHandler) rotateRefresh(userID uuid.UUID, jti, replacedBy string) (bool, error) {
	var rt db.RefreshToken
	if err := h.DB.Where("jti = ? AND user_id = ?", jti, userID).First(&rt).Error; err != nil {
		return false, err
	}
	if rt.RevokedAt != nil || time.Now().After(rt.ExpiresAt) {
		h.revokeAllRefresh(userID)
		return false, nil
	}
	now := time.Now()
	if err := h.DB.Model(&rt).Updates(map[string]interface{}{
		"revoked_at":  now,
		"replaced_by": replacedBy,
	}).Error; err != nil {
		return false, err
	}
	return true, nil
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}
	keys := h.loginKeys(c, req.Email)
	if ok, remaining := h.Limiter.Allowed(keys...); !ok {
		h.tooMany(c, remaining)
		return
	}

	var user db.User
	if err := h.DB.Where("email = ? AND active = true", req.Email).First(&user).Error; err != nil {
		h.Limiter.Fail(keys...)
		audit.Login(h.DB, req.Email, c.ClientIP(), false, nil, nil)
		Unauthorized(c, "invalid credentials")
		return
	}
	if !auth.CheckPassword(user.PasswordHash, req.Password) {
		h.Limiter.Fail(keys...)
		audit.Login(h.DB, req.Email, c.ClientIP(), false, nil, nil)
		Unauthorized(c, "invalid credentials")
		return
	}
	h.Limiter.Reset(keys...)

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
	if err := h.storeRefresh(user.ID, tokens); err != nil {
		InternalError(c, "token storage failed")
		return
	}
	h.DB.Model(&user).Update("last_login", gorm.Expr("NOW()"))

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"tokens": tokens,
			"user": gin.H{
				"id":                   user.ID,
				"email":                user.Email,
				"role":                 role.Name,
				"tenant_id":            user.TenantID,
				"must_change_password": user.MustChangePassword,
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
	claims, err := auth.ValidateRefreshToken(h.JWTSecret, req.RefreshToken)
	if err != nil {
		Unauthorized(c, "invalid refresh token")
		return
	}
	var user db.User
	if err := h.DB.First(&user, "id = ? AND active = true", claims.UserID).Error; err != nil {
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
	ok, err := h.rotateRefresh(user.ID, claims.ID, tokens.RefreshJTI)
	if err != nil || !ok {
		Unauthorized(c, "refresh token reuse detected")
		return
	}
	if err := h.storeRefresh(user.ID, tokens); err != nil {
		InternalError(c, "token storage failed")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"tokens": tokens,
		"user": gin.H{
			"id":                   user.ID,
			"email":                user.Email,
			"role":                 role.Name,
			"tenant_id":            user.TenantID,
			"must_change_password": user.MustChangePassword,
		},
	}})
}

type logoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *AuthHandler) Logout(c *gin.Context) {
	var req logoutRequest
	_ = c.ShouldBindJSON(&req)
	if req.RefreshToken != "" {
		if claims, err := auth.ValidateRefreshToken(h.JWTSecret, req.RefreshToken); err == nil {
			h.rotateRefresh(claims.UserID, claims.ID, "")
		}
	}
	c.Status(http.StatusNoContent)
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
		"id":                   user.ID,
		"email":                user.Email,
		"role":                 cl.RoleName,
		"tenant_id":            user.TenantID,
		"active":               user.Active,
		"mfa_enabled":          user.MFAEnabled,
		"must_change_password": user.MustChangePassword,
	})
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=8"`
}

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	var req changePasswordRequest
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
	if !auth.CheckPassword(user.PasswordHash, req.CurrentPassword) {
		Unauthorized(c, "invalid current password")
		return
	}
	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		InternalError(c, err.Error())
		return
	}
	if err := h.DB.Model(&user).Updates(map[string]interface{}{
		"password_hash":        hash,
		"must_change_password": false,
	}).Error; err != nil {
		InternalError(c, err.Error())
		return
	}
	h.revokeAllRefresh(user.ID)
	OK(c, gin.H{"status": "password changed"})
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
	keys := h.mfaKeys(c, userID)
	if ok, remaining := h.Limiter.Allowed(keys...); !ok {
		h.tooMany(c, remaining)
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
	secret, err := h.decryptMFA(user)
	if err != nil {
		InternalError(c, "mfa secret error")
		return
	}
	if !totp.Validate(req.Code, secret) {
		h.Limiter.Fail(keys...)
		audit.Login(h.DB, user.Email, c.ClientIP(), false, nil, nil)
		Unauthorized(c, "invalid code")
		return
	}
	h.Limiter.Reset(keys...)
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
	if err := h.storeRefresh(user.ID, tokens); err != nil {
		InternalError(c, "token storage failed")
		return
	}
	audit.Login(h.DB, user.Email, c.ClientIP(), true, &user.ID, user.TenantID)
	h.DB.Model(&user).Update("last_login", gorm.Expr("NOW()"))
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"tokens": tokens,
			"user": gin.H{
				"id":                   user.ID,
				"email":                user.Email,
				"role":                 role.Name,
				"tenant_id":            user.TenantID,
				"must_change_password": user.MustChangePassword,
			},
		},
	})
}

func (h *AuthHandler) decryptMFA(user db.User) (string, error) {
	secret, err := auth.DecryptSecret(h.MFAKey, []byte(user.ID.String()), user.MFASecret)
	if err == nil {
		return secret, nil
	}
	legacy, legacyErr := auth.DecryptSecretLegacy(h.JWTSecret, user.MFASecret)
	if legacyErr != nil {
		return "", err
	}
	if enc, encErr := auth.EncryptSecret(h.MFAKey, []byte(user.ID.String()), legacy); encErr == nil {
		h.DB.Model(&user).Update("mfa_secret", enc)
	}
	return legacy, nil
}

type mfaCodeRequest struct {
	Code     string `json:"code" binding:"required"`
	Password string `json:"password"`
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
	enc, err := auth.EncryptSecret(h.MFAKey, []byte(user.ID.String()), key.Secret())
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
	secret, err := h.decryptMFA(user)
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
	if !reauth(c, h.DB, req.Password) {
		Forbidden(c, "re-authentication required")
		return
	}
	secret, err := h.decryptMFA(user)
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
