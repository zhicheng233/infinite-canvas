package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"infinite-canvas-server/model"
	"infinite-canvas-server/service"
)

const apiConfigTransferMaxRequestBytes = 20 << 20

type APIConfigTransferHandler struct {
	service *service.APIConfigTransferService
}

func NewAPIConfigTransferHandler(transferService *service.APIConfigTransferService) *APIConfigTransferHandler {
	return &APIConfigTransferHandler{service: transferService}
}

func (h *APIConfigTransferHandler) Export(c *gin.Context) {
	var input model.APIConfigTransferExportInput
	if err := bindAPIConfigTransferJSON(c, &input); err != nil {
		model.Fail(c, http.StatusBadRequest, "无效的请求参数")
		return
	}
	result, err := h.service.Export(currentTenantID(c), input.Password)
	if err != nil {
		model.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	model.OK(c, result)
}

func (h *APIConfigTransferHandler) Preview(c *gin.Context) {
	var input model.APIConfigTransferImportInput
	if err := bindAPIConfigTransferJSON(c, &input); err != nil {
		model.Fail(c, http.StatusBadRequest, "无效或过大的配置文件")
		return
	}
	result, err := h.service.Preview(currentTenantID(c), input)
	if err != nil {
		model.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	model.OK(c, result)
}

func (h *APIConfigTransferHandler) Import(c *gin.Context) {
	var input model.APIConfigTransferImportInput
	if err := bindAPIConfigTransferJSON(c, &input); err != nil {
		model.Fail(c, http.StatusBadRequest, "无效或过大的配置文件")
		return
	}
	result, err := h.service.Import(currentTenantID(c), input)
	if err != nil {
		model.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	model.OK(c, result)
}

func bindAPIConfigTransferJSON(c *gin.Context, target interface{}) error {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, apiConfigTransferMaxRequestBytes)
	return c.ShouldBindJSON(target)
}
