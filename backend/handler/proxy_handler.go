package handler

import (
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"infinite-canvas-server/service"
)

type ProxyHandler struct {
	generateService *service.GenerateService
}

const proxyMaxRequestBytes = 64 << 20

func NewProxyHandler(generateService *service.GenerateService) *ProxyHandler {
	return &ProxyHandler{generateService: generateService}
}

func (h *ProxyHandler) Proxy(c *gin.Context) {
	claims := c.MustGet("claims").(*service.Claims)
	targetPath := c.Query("path")
	if !validProxyPath(targetPath) {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "path is required"})
		return
	}

	method := c.Request.Method
	contentType := c.GetHeader("Content-Type")
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, proxyMaxRequestBytes)
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"code": 413, "msg": "请求内容过大"})
		return
	}

	result, err := h.generateService.ProxyRawWithRepair(claims.TenantID, claims.UserID, method, targetPath, contentType, body, channelSelectionFromRequest(c))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	c.Header("X-Credits-Cost", itoa(result.Cost))
	c.Header("X-Credits-Balance", itoa(result.Balance))
	c.Header("X-Credits-Refund", itoa(result.Refund))
	writeResolvedChannelHeaders(c, result)

	if result.StatusCode >= http.StatusBadRequest {
		bodyStr := string(result.Body)
		if !service.IsUpstreamBalanceError(bodyStr) {
			c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "上游请求失败", "error_detail": bodyStr})
			return
		}
	}
	copyProxyResponseHeaders(c, result.Headers)
	respContentType := result.Headers.Get("Content-Type")
	respContentType = proxyResponseContentType(respContentType, result.Body)
	c.Header("Content-Type", respContentType)
	c.Data(result.StatusCode, respContentType, result.Body)
}

func (h *ProxyHandler) ProxyGet(c *gin.Context) {
	claims := c.MustGet("claims").(*service.Claims)
	targetPath := c.Query("path")
	if !validProxyPath(targetPath) {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "path is required"})
		return
	}

	result, err := h.generateService.ProxyRawWithRepair(claims.TenantID, claims.UserID, "GET", targetPath, "", nil, channelSelectionFromRequest(c))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	writeResolvedChannelHeaders(c, result)
	c.Header("X-Credits-Balance", itoa(result.Balance))
	c.Header("X-Credits-Refund", itoa(result.Refund))

	if result.StatusCode >= http.StatusBadRequest {
		bodyStr := string(result.Body)
		if !service.IsUpstreamBalanceError(bodyStr) {
			c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "上游请求失败", "error_detail": bodyStr})
			return
		}
	}
	copyProxyResponseHeaders(c, result.Headers)
	respContentType := result.Headers.Get("Content-Type")
	respContentType = proxyResponseContentType(respContentType, result.Body)
	c.Header("Content-Type", respContentType)
	c.Data(result.StatusCode, respContentType, result.Body)
}

func validProxyPath(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || strings.Contains(value, "\\") || strings.Contains(value, "://") {
		return false
	}
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.IsAbs() || parsed.Host != "" {
		return false
	}
	for _, part := range strings.Split(parsed.Path, "/") {
		if part == ".." {
			return false
		}
	}
	return strings.HasPrefix(parsed.Path, "/")
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

	writeResolvedChannelHeaders(c, result)
	c.Header("X-Credits-Balance", itoa(result.Balance))
	c.Header("X-Credits-Refund", itoa(result.Refund))

	if result.StatusCode >= http.StatusBadRequest {
		bodyStr := string(result.Body)
		if !service.IsUpstreamBalanceError(bodyStr) {
			c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "上游请求失败", "error_detail": bodyStr})
			return
		}
	}
	copyProxyResponseHeaders(c, result.Headers)
	respContentType := result.Headers.Get("Content-Type")
	respContentType = proxyResponseContentType(respContentType, result.Body)
	c.Header("Content-Type", respContentType)
	c.Data(result.StatusCode, respContentType, result.Body)
}

func proxyResponseContentType(contentType string, body []byte) string {
	normalized := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	if normalized == "" || normalized == "application/octet-stream" || normalized == "binary/octet-stream" {
		if detected := detectVideoContentType(body); detected != "" {
			return detected
		}
		if contentType == "" {
			return "application/octet-stream"
		}
	}
	return contentType
}

func detectVideoContentType(body []byte) string {
	if len(body) >= 12 && string(body[4:8]) == "ftyp" {
		if string(body[8:12]) == "qt  " {
			return "video/quicktime"
		}
		return "video/mp4"
	}
	if len(body) >= 4 && body[0] == 0x1a && body[1] == 0x45 && body[2] == 0xdf && body[3] == 0xa3 {
		return "video/webm"
	}
	return ""
}
