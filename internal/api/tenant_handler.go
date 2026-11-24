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

type TenantHandler struct {
	DB *gorm.DB
}

func (h *TenantHandler) List(c *gin.Context) {
	cl := auth.ClaimsFromContext(c)
	var tenants []db.Tenant
	q := h.DB
	if cl != nil && cl.RoleName != "superadmin" {
		// non-superadmin: only own tenant
		if cl.TenantID != nil {
			q = q.Where("id = ?", *cl.TenantID)
		} else {
			q = q.Where("1 = 0")
		}
	}
	if err := q.Find(&tenants).Error; err != nil {
		InternalError(c, err.Error())
		return
	}
	OK(c, tenants)
}

func (h *TenantHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		BadRequest(c, "invalid id")
		return
	}
	var t db.Tenant
	if err := h.DB.First(&t, "id = ?", id).Error; err != nil {
		NotFound(c, "tenant not found")
		return
	}
	OK(c, t)
}

type tenantRequest struct {
	Name string `json:"name" binding:"required"`
	Slug string `json:"slug" binding:"required"`
}

func (h *TenantHandler) Create(c *gin.Context) {
	var req tenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}
	t := db.Tenant{Name: req.Name, Slug: req.Slug}
	if err := h.DB.Create(&t).Error; err != nil {
		BadRequest(c, err.Error())
		return
	}
	Created(c, t)
}

func (h *TenantHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		BadRequest(c, "invalid id")
		return
	}
	var t db.Tenant
	if err := h.DB.First(&t, "id = ?", id).Error; err != nil {
		NotFound(c, "tenant not found")
		return
	}
	var req tenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}
	t.Name = req.Name
	t.Slug = req.Slug
	if err := h.DB.Save(&t).Error; err != nil {
		InternalError(c, err.Error())
		return
	}
	OK(c, t)
}

func (h *TenantHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		BadRequest(c, "invalid id")
		return
	}
	var t db.Tenant
	if err := h.DB.Select("name").First(&t, "id = ?", id).Error; err == nil {
		audit.SetDetail(c, fmt.Sprintf("Tenant %q gelöscht", t.Name))
	}
	if err := h.DB.Delete(&db.Tenant{}, "id = ?", id).Error; err != nil {
		InternalError(c, err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}
