package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/inboxes-cig/bastion/internal/crowdsec"
	"github.com/inboxes-cig/bastion/internal/db"
	"github.com/inboxes-cig/bastion/internal/iplist"
	"gorm.io/gorm"
)

type GoodActorHandler struct {
	DB *gorm.DB
	CS *crowdsec.Client
}

const goodActorsPath = "/var/lib/bastion/good-actors.yaml"

func (h *GoodActorHandler) List(c *gin.Context) {
	var actors []db.GoodActor
	if err := h.DB.Order("created_at DESC").Find(&actors).Error; err != nil {
		InternalError(c, err.Error())
		return
	}
	OK(c, actors)
}

type goodActorRequest struct {
	Value string `json:"value" binding:"required"`
	Note  string `json:"note"`
}

func (h *GoodActorHandler) sync(value string) {
	_ = crowdsec.WriteGoodActors(h.DB, goodActorsPath)
	if h.CS != nil {
		if value != "" {
			_ = h.CS.DeleteDecisionsByIP(value)
		}
		_ = h.CS.Refresh()
	}
}

func (h *GoodActorHandler) Create(c *gin.Context) {
	if !requireSuperadmin(c) {
		return
	}
	var req goodActorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}
	if err := iplist.Validate(req.Value); err != nil {
		BadRequest(c, err.Error())
		return
	}
	a := db.GoodActor{Value: strings.TrimSpace(req.Value), Note: req.Note}
	if err := h.DB.Create(&a).Error; err != nil {
		BadRequest(c, err.Error())
		return
	}
	h.sync(a.Value)
	Created(c, a)
}

func (h *GoodActorHandler) Update(c *gin.Context) {
	if !requireSuperadmin(c) {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		BadRequest(c, "invalid id")
		return
	}
	var a db.GoodActor
	if err := h.DB.First(&a, "id = ?", id).Error; err != nil {
		NotFound(c, "entry not found")
		return
	}
	var req goodActorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}
	if err := iplist.Validate(req.Value); err != nil {
		BadRequest(c, err.Error())
		return
	}
	a.Value = strings.TrimSpace(req.Value)
	a.Note = req.Note
	if err := h.DB.Save(&a).Error; err != nil {
		InternalError(c, err.Error())
		return
	}
	h.sync(a.Value)
	OK(c, a)
}

func (h *GoodActorHandler) Delete(c *gin.Context) {
	if !requireSuperadmin(c) {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		BadRequest(c, "invalid id")
		return
	}
	if err := h.DB.Delete(&db.GoodActor{}, "id = ?", id).Error; err != nil {
		InternalError(c, err.Error())
		return
	}
	h.sync("")
	c.Status(http.StatusNoContent)
}
