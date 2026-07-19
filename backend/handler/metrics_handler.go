package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"infinite-canvas-server/model"
	"infinite-canvas-server/service"
)

type MetricsHandler struct {
	service *service.MetricsService
}

func NewMetricsHandler(metricsService *service.MetricsService) *MetricsHandler {
	return &MetricsHandler{service: metricsService}
}

func (h *MetricsHandler) GetConfig(c *gin.Context) {
	cfg, err := h.service.GetConfig()
	if err != nil {
		model.Fail(c, http.StatusInternalServerError, "读取指标配置失败")
		return
	}
	model.OK(c, cfg)
}

func (h *MetricsHandler) SaveConfig(c *gin.Context) {
	var input model.MetricsURLConfig
	if err := c.ShouldBindJSON(&input); err != nil {
		model.Fail(c, http.StatusBadRequest, "无效的请求参数")
		return
	}
	cfg, err := h.service.SaveConfig(input)
	if err != nil {
		model.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	model.OK(c, cfg)
}

func (h *MetricsHandler) Read(c *gin.Context) {
	response, err := h.service.Read(service.ParseMetricsHours(c.Query("hours")))
	if err != nil {
		model.Fail(c, http.StatusInternalServerError, "读取指标失败")
		return
	}
	model.OK(c, response)
}
