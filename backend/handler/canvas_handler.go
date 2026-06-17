package handler

import (
	"encoding/json"
	"net/http"

	"infinite-canvas-server/model"
	"infinite-canvas-server/repository"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type CanvasHandler struct {
	repo *repository.CanvasRepo
}

func NewCanvasHandler(repo *repository.CanvasRepo) *CanvasHandler {
	return &CanvasHandler{repo: repo}
}

type canvasSaveRequest struct {
	ID             string          `json:"id" binding:"required"`
	Title          string          `json:"title" binding:"required"`
	Nodes          json.RawMessage `json:"nodes"`
	Connections    json.RawMessage `json:"connections"`
	ChatSessions   json.RawMessage `json:"chat_sessions"`
	ActiveChatID   string          `json:"active_chat_id"`
	BackgroundMode string          `json:"background_mode"`
	ShowImageInfo  *bool           `json:"show_image_info"`
	ViewportX      float64         `json:"viewport_x"`
	ViewportY      float64         `json:"viewport_y"`
	ViewportK      float64         `json:"viewport_k"`
}

func (h *CanvasHandler) Save(c *gin.Context) {
	var req canvasSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}

	userID := c.GetUint("user_id")
	tenantID := c.GetUint("tenant_id")

	project := &model.CanvasProject{
		TenantID:       tenantID,
		UserID:         userID,
		ProjectID:      req.ID,
		Title:          req.Title,
		Nodes:          string(req.Nodes),
		Connections:    string(req.Connections),
		ChatSessions:   string(req.ChatSessions),
		ActiveChatID:   req.ActiveChatID,
		BackgroundMode: req.BackgroundMode,
		ShowImageInfo:  req.ShowImageInfo,
		ViewportX:      req.ViewportX,
		ViewportY:      req.ViewportY,
		ViewportK:      req.ViewportK,
	}

	if err := h.repo.Upsert(project); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok"})
}

func (h *CanvasHandler) Load(c *gin.Context) {
	projectID := c.Param("id")
	tenantID := c.GetUint("tenant_id")

	project, err := h.repo.FindByProjectID(tenantID, projectID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusOK, gin.H{"code": 0, "data": nil, "msg": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": project, "msg": "ok"})
}

func (h *CanvasHandler) List(c *gin.Context) {
	tenantID := c.GetUint("tenant_id")

	projects, err := h.repo.ListByTenant(tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	if projects == nil {
		projects = []model.CanvasProject{}
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": projects, "msg": "ok"})
}

func (h *CanvasHandler) Delete(c *gin.Context) {
	projectID := c.Param("id")
	tenantID := c.GetUint("tenant_id")

	if err := h.repo.Delete(tenantID, projectID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok"})
}

type batchDeleteRequest struct {
	IDs []string `json:"ids" binding:"required"`
}

func (h *CanvasHandler) DeleteBatch(c *gin.Context) {
	var req batchDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}

	tenantID := c.GetUint("tenant_id")
	if err := h.repo.DeleteBatch(tenantID, req.IDs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok"})
}
