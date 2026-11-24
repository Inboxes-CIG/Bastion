package api

import (
	"github.com/gin-gonic/gin"
	"github.com/inboxes-cig/bastion/internal/version"
)

type VersionHandler struct {
	CachePath string
}

func (h *VersionHandler) Get(c *gin.Context) {
	OK(c, version.Check(h.CachePath))
}
