package handler

import (
	"github.com/gin-gonic/gin"
	"infinite-canvas-server/model"
	"infinite-canvas-server/service"
)

type SiteAnnouncementHandler struct {
	service *service.SiteAnnouncementService
}

func NewSiteAnnouncementHandler(service *service.SiteAnnouncementService) *SiteAnnouncementHandler {
	return &SiteAnnouncementHandler{service: service}
}

func (h *SiteAnnouncementHandler) Public(c *gin.Context) {
	item, err := h.service.GetPublic()
	if err != nil {
		model.Fail(c, 500, "读取公告失败")
		return
	}
	model.OK(c, item)
}

func (h *SiteAnnouncementHandler) AdminGet(c *gin.Context) {
	item, err := h.service.GetAdmin()
	if err != nil {
		model.Fail(c, 500, "读取公告配置失败")
		return
	}
	model.OK(c, item)
}

func (h *SiteAnnouncementHandler) AdminSave(c *gin.Context) {
	var input model.SiteAnnouncementPayload
	if err := c.ShouldBindJSON(&input); err != nil {
		model.Fail(c, 400, "无效的请求参数")
		return
	}
	item, err := h.service.Save(input)
	if err != nil {
		model.Fail(c, 400, err.Error())
		return
	}
	model.OK(c, item)
}
