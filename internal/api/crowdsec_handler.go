package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/inboxes-cig/bastion/internal/crowdsec"
)

type CrowdSecHandler struct {
	CS *crowdsec.Client
}

func (h *CrowdSecHandler) Status(c *gin.Context) {
	if h.CS == nil {
		OK(c, gin.H{"enabled": false})
		return
	}
	OK(c, h.CS.Status())
}

func (h *CrowdSecHandler) Alerts(c *gin.Context) {
	if h.CS == nil {
		OK(c, []crowdsec.Alert{})
		return
	}
	limit := 50
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil {
			limit = n
		}
	}
	alerts, err := h.CS.Alerts(limit)
	if err != nil {
		InternalError(c, err.Error())
		return
	}
	OK(c, alerts)
}

func (h *CrowdSecHandler) Decisions(c *gin.Context) {
	if h.CS == nil {
		OK(c, []crowdsec.Decision{})
		return
	}
	OK(c, h.CS.List())
}

type banRequest struct {
	IP       string `json:"ip" binding:"required"`
	Duration string `json:"duration"`
	Reason   string `json:"reason"`
}

func (h *CrowdSecHandler) Add(c *gin.Context) {
	if !requireSuperadmin(c) {
		return
	}
	if h.CS == nil {
		BadRequest(c, "crowdsec is not enabled")
		return
	}
	var req banRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}
	if err := h.CS.AddBan(req.IP, req.Duration, req.Reason); err != nil {
		InternalError(c, err.Error())
		return
	}
	_ = h.CS.Refresh()
	Created(c, gin.H{"ip": req.IP})
}

func (h *CrowdSecHandler) Delete(c *gin.Context) {
	if !requireSuperadmin(c) {
		return
	}
	if h.CS == nil {
		BadRequest(c, "crowdsec is not enabled")
		return
	}
	if err := h.CS.DeleteDecision(c.Param("id")); err != nil {
		InternalError(c, err.Error())
		return
	}
	_ = h.CS.Refresh()
	c.Status(http.StatusNoContent)
}
