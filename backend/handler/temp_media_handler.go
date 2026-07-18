package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"infinite-canvas-server/model"
	"infinite-canvas-server/service"
)

type TempMediaHandler struct {
	service *service.TempMediaService
}

func NewTempMediaHandler(service *service.TempMediaService) *TempMediaHandler {
	return &TempMediaHandler{service: service}
}

func (h *TempMediaHandler) UploadImage(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		model.Fail(c, 400, "请上传图片文件")
		return
	}
	result, err := h.service.SaveImage(file)
	if err != nil {
		model.Fail(c, 400, err.Error())
		return
	}
	if result != nil && strings.HasPrefix(result.URL, "/") {
		result.URL = requestBaseURL(c) + result.URL
	}
	model.OK(c, result)
}

func (h *TempMediaHandler) Serve(c *gin.Context) {
	filename := c.Param("filename")
	if filename == "" {
		c.Status(http.StatusNotFound)
		return
	}
	c.File(h.service.FilePath(filename))
}

func requestBaseURL(c *gin.Context) string {
	scheme := c.GetHeader("X-Forwarded-Proto")
	if scheme == "" {
		if c.Request.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	host := c.GetHeader("X-Forwarded-Host")
	if host == "" {
		host = c.Request.Host
	}
	return scheme + "://" + host
}
