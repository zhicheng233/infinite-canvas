package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"infinite-canvas-server/model"
	"infinite-canvas-server/service"
)

type AutoRoutingHandler struct{ service *service.AutoChannelService }

func NewAutoRoutingHandler(svc *service.AutoChannelService) *AutoRoutingHandler {
	return &AutoRoutingHandler{service: svc}
}

func (h *AutoRoutingHandler) Suggestions(c *gin.Context) {
	items, err := h.service.Suggestions()
	if err != nil {
		model.Fail(c, http.StatusInternalServerError, "读取智能路由建议失败")
		return
	}
	model.OK(c, gin.H{"suggestions": items})
}

func (h *AutoRoutingHandler) List(c *gin.Context) {
	items, err := h.service.ListPools()
	if err != nil {
		model.Fail(c, http.StatusInternalServerError, "读取智能路由池失败")
		return
	}
	model.OK(c, gin.H{"pools": items})
}

func (h *AutoRoutingHandler) Create(c *gin.Context) {
	var input service.SaveAutoRoutingPoolInput
	if c.ShouldBindJSON(&input) != nil {
		model.Fail(c, http.StatusBadRequest, "无效的请求参数")
		return
	}
	item, err := h.service.CreatePool(input)
	if err != nil {
		model.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	model.OK(c, item)
}

func (h *AutoRoutingHandler) Update(c *gin.Context) {
	id, err := parsePositiveID(c.Param("id"))
	if err != nil {
		model.Fail(c, http.StatusBadRequest, "无效的路由池 ID")
		return
	}
	var input service.UpdateAutoRoutingPoolInput
	if c.ShouldBindJSON(&input) != nil {
		model.Fail(c, http.StatusBadRequest, "无效的请求参数")
		return
	}
	item, err := h.service.UpdatePool(id, input)
	if err != nil {
		model.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	model.OK(c, item)
}

func (h *AutoRoutingHandler) UpdateMember(c *gin.Context) {
	poolID, poolErr := parsePositiveID(c.Param("id"))
	memberID, memberErr := parsePositiveID(c.Param("memberId"))
	if poolErr != nil || memberErr != nil {
		model.Fail(c, http.StatusBadRequest, "无效的候选 ID")
		return
	}
	var input service.UpdateAutoRoutingMemberInput
	if c.ShouldBindJSON(&input) != nil {
		model.Fail(c, http.StatusBadRequest, "无效的请求参数")
		return
	}
	item, err := h.service.UpdateMember(poolID, memberID, input)
	if err != nil {
		model.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	model.OK(c, item)
}

func (h *AutoRoutingHandler) Delete(c *gin.Context) {
	id, err := parsePositiveID(c.Param("id"))
	if err != nil {
		model.Fail(c, http.StatusBadRequest, "无效的路由池 ID")
		return
	}
	if err := h.service.DeletePool(id); err != nil {
		model.Fail(c, http.StatusInternalServerError, "删除智能路由池失败")
		return
	}
	model.OK(c, gin.H{"deleted": true})
}
