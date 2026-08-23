package handler

import (
	"github.com/gin-gonic/gin"
	"infinite-canvas-server/model"
)

type HealthHandler struct {
	version string
}

func NewHealthHandler(version string) *HealthHandler {
	return &HealthHandler{version: version}
}

func (h *HealthHandler) Get(c *gin.Context) {
	model.OK(c, gin.H{
		"service": "backend",
		"status":  "ok",
		"version": h.version,
	})
}
