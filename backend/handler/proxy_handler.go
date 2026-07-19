package handler

import (
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"infinite-canvas-server/service"
)

type ProxyHandler struct {
	generateService *service.GenerateService
}

func NewProxyHandler(generateService *service.GenerateService) *ProxyHandler {
	return &ProxyHandler{generateService: generateService}
}

func (h *ProxyHandler) Proxy(c *gin.Context) {
	claims := c.MustGet("claims").(*service.Claims)
	targetPath := c.Query("path")
	if targetPath == "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "path is required"})
		return
	}

	method := c.Request.Method
	contentType := c.GetHeader("Content-Type")
	body, _ := io.ReadAll(c.Request.Body)

	result, err := h.generateService.ProxyRawWithRepair(claims.TenantID, claims.UserID, method, targetPath, contentType, body, channelSelectionFromRequest(c))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	for key, values := range result.Headers {
		for _, value := range values {
			c.Header(key, value)
		}
	}
	c.Header("X-Credits-Cost", itoa(result.Cost))
	c.Header("X-Credits-Balance", itoa(result.Balance))

	respContentType := result.Headers.Get("Content-Type")
	if respContentType == "" {
		respContentType = "application/octet-stream"
	}
	c.Data(result.StatusCode, respContentType, result.Body)
}

func (h *ProxyHandler) ProxyGet(c *gin.Context) {
	claims := c.MustGet("claims").(*service.Claims)
	targetPath := c.Query("path")
	if targetPath == "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "path is required"})
		return
	}

	result, err := h.generateService.ProxyRawWithRepair(claims.TenantID, claims.UserID, "GET", targetPath, "", nil, channelSelectionFromRequest(c))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	for key, values := range result.Headers {
		for _, value := range values {
			c.Header(key, value)
		}
	}

	respContentType := result.Headers.Get("Content-Type")
	if respContentType == "" {
		respContentType = "application/octet-stream"
	}
	c.Data(result.StatusCode, respContentType, result.Body)
}

func (h *ProxyHandler) ProxyGetPath(c *gin.Context) {
	claims := c.MustGet("claims").(*service.Claims)
	targetPath := "/" + strings.TrimPrefix(c.Param("path"), "/")
	query := c.Request.URL.RawQuery
	if query != "" {
		targetPath += "?" + query
	}

	result, err := h.generateService.ProxyRawWithRepair(claims.TenantID, claims.UserID, "GET", targetPath, "", nil, channelSelectionFromRequest(c))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	for key, values := range result.Headers {
		for _, value := range values {
			c.Header(key, value)
		}
	}

	respContentType := result.Headers.Get("Content-Type")
	if respContentType == "" {
		respContentType = "application/octet-stream"
	}
	c.Data(result.StatusCode, respContentType, result.Body)
}
