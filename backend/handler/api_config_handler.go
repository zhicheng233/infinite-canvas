package handler

import (
	"github.com/gin-gonic/gin"
	"infinite-canvas-server/model"
	"infinite-canvas-server/repository"
	"infinite-canvas-server/service"
)

type ApiConfigHandler struct {
	apiConfigRepo *repository.ApiConfigRepo
}

func NewApiConfigHandler(apiConfigRepo *repository.ApiConfigRepo) *ApiConfigHandler {
	return &ApiConfigHandler{apiConfigRepo: apiConfigRepo}
}

type SaveApiConfigInput struct {
	BaseUrl string `json:"base_url"`
	ApiKey  string `json:"api_key"`
}

func (h *ApiConfigHandler) Get(c *gin.Context) {
	claims := c.MustGet("claims").(*service.Claims)
	cfg, err := h.apiConfigRepo.FindByTenant(claims.TenantID)
	if err != nil {
		model.Fail(c, 404, "未配置 API")
		return
	}
	model.OK(c, gin.H{
		"base_url": cfg.BaseUrl,
		"has_key":  len(cfg.ApiKey) > 0,
	})
}

func (h *ApiConfigHandler) Save(c *gin.Context) {
	claims := c.MustGet("claims").(*service.Claims)
	var input SaveApiConfigInput
	if err := c.ShouldBindJSON(&input); err != nil {
		model.Fail(c, 400, "无效的请求参数")
		return
	}
	cfg := &model.TenantApiConfig{
		TenantID: claims.TenantID,
		BaseUrl:  input.BaseUrl,
		ApiKey:   input.ApiKey,
	}
	if err := h.apiConfigRepo.Save(cfg); err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.OK(c, gin.H{"saved": true})
}
