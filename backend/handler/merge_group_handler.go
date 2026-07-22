package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"infinite-canvas-server/model"
	"infinite-canvas-server/service"
)

type MergeGroupHandler struct {
	service *service.MergeGroupService
}

func NewMergeGroupHandler(svc *service.MergeGroupService) *MergeGroupHandler {
	return &MergeGroupHandler{service: svc}
}

func (h *MergeGroupHandler) List(c *gin.Context) {
	channelID, err := parsePositiveID(c.Param("id"))
	if err != nil {
		model.Fail(c, http.StatusBadRequest, "无效的渠道 ID")
		return
	}
	groups, err := h.service.ListByChannel(channelID)
	if err != nil {
		model.Fail(c, http.StatusInternalServerError, "读取合并组失败")
		return
	}
	model.OK(c, gin.H{"groups": groups})
}

func (h *MergeGroupHandler) Create(c *gin.Context) {
	channelID, err := parsePositiveID(c.Param("id"))
	if err != nil {
		model.Fail(c, http.StatusBadRequest, "无效的渠道 ID")
		return
	}
	var input struct {
		GroupName string `json:"group_name"`
		Pattern   string `json:"pattern"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		model.Fail(c, http.StatusBadRequest, "无效的请求参数")
		return
	}
	group := &model.ModelMergeGroup{
		ChannelID: channelID,
		GroupName: input.GroupName,
		Pattern:   input.Pattern,
		Enabled:   true,
	}
	if err := h.service.Create(group); err != nil {
		model.Fail(c, http.StatusInternalServerError, "创建合并组失败")
		return
	}
	model.OK(c, group)
}

func (h *MergeGroupHandler) Delete(c *gin.Context) {
	groupID, err := parsePositiveID(c.Param("groupId"))
	if err != nil {
		model.Fail(c, http.StatusBadRequest, "无效的合并组 ID")
		return
	}
	if err := h.service.Delete(groupID); err != nil {
		model.Fail(c, http.StatusInternalServerError, "删除合并组失败")
		return
	}
	model.OK(c, gin.H{"deleted": true})
}

func (h *MergeGroupHandler) AutoCreate(c *gin.Context) {
	channelID, err := parsePositiveID(c.Param("id"))
	if err != nil {
		model.Fail(c, http.StatusBadRequest, "无效的渠道 ID")
		return
	}
	groups, err := h.service.AutoCreate(channelID)
	if err != nil {
		model.Fail(c, http.StatusInternalServerError, "自动生成合并组失败")
		return
	}
	model.OK(c, gin.H{"groups": groups})
}
