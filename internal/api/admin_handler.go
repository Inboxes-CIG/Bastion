package api

import (
	"github.com/gin-gonic/gin"
	"github.com/inboxes-cig/bastion/internal/db"
	"gorm.io/gorm"
)

type AdminHandler struct {
	DB         *gorm.DB
	AdminHosts []string
}

func (h *AdminHandler) Info(c *gin.Context) {
	var certs []db.Certificate
	if len(h.AdminHosts) > 0 {
		h.DB.Where("source = ? AND domain IN ?", "letsencrypt", h.AdminHosts).Find(&certs)
	}
	OK(c, gin.H{
		"admin_hosts": h.AdminHosts,
		"certs":       certs,
	})
}
