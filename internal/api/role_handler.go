package api

import (

	"github.com/gin-gonic/gin"
	"github.com/inboxes-cig/bastion/internal/db"
	"gorm.io/gorm"
)

type RoleHandler struct {
	DB *gorm.DB
}

func (h *RoleHandler) List(c *gin.Context) {
	var roles []db.Role
	if err := h.DB.Find(&roles).Error; err != nil {
		InternalError(c, err.Error())
		return
	}
	OK(c, roles)
}
