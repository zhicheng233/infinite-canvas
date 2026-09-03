package handler

import (
	"github.com/gin-gonic/gin"
	"infinite-canvas-server/model"
	"infinite-canvas-server/service"
)

type FeatureGuideHandler struct{ service *service.FeatureGuideService }

func NewFeatureGuideHandler(service *service.FeatureGuideService) *FeatureGuideHandler {
	return &FeatureGuideHandler{service: service}
}

func (h *FeatureGuideHandler) Get(c *gin.Context) {
	item, err := h.service.GetPublic(model.FeatureGuideSurface(c.Param("surface")))
	if err != nil {
		if service.IsFeatureGuideValidationError(err) {
			model.Fail(c, 400, err.Error())
		} else {
			model.Fail(c, 500, "读取功能引导失败")
		}
		return
	}
	model.OK(c, item)
}

func (h *FeatureGuideHandler) AdminList(c *gin.Context) {
	items, err := h.service.ListAdmin()
	if err != nil {
		model.Fail(c, 500, "读取功能引导配置失败")
		return
	}
	model.OK(c, items)
}

func (h *FeatureGuideHandler) AdminSave(c *gin.Context) {
	var input model.FeatureGuidePayload
	if err := c.ShouldBindJSON(&input); err != nil {
		model.Fail(c, 400, "无效的请求参数")
		return
	}
	item, err := h.service.Save(model.FeatureGuideSurface(c.Param("surface")), input)
	if err != nil {
		if service.IsFeatureGuideValidationError(err) {
			model.Fail(c, 400, err.Error())
		} else {
			model.Fail(c, 500, "保存功能引导配置失败")
		}
		return
	}
	model.OK(c, item)
}
