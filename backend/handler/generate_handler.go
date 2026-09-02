package handler

import (
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"infinite-canvas-server/service"
)

type GenerateHandler struct {
	generateService *service.GenerateService
}

const generationMaxRequestBytes = 64 << 20

func NewGenerateHandler(generateService *service.GenerateService) *GenerateHandler {
	return &GenerateHandler{generateService: generateService}
}

func (h *GenerateHandler) Image(c *gin.Context) {
	h.handleProxy(c, h.generateService.ProxyImage)
}

func (h *GenerateHandler) Text(c *gin.Context) {
	h.handleProxy(c, h.generateService.ProxyText)
}

func (h *GenerateHandler) Video(c *gin.Context) {
	h.handleProxy(c, h.generateService.ProxyVideo)
}

func (h *GenerateHandler) Audio(c *gin.Context) {
	h.handleProxy(c, h.generateService.ProxyAudio)
}

type proxyFunc func(tenantID, userID uint, contentType string, body []byte, selection service.ChannelSelection) (*service.ProxyResult, error)

func (h *GenerateHandler) handleProxy(c *gin.Context, fn proxyFunc) {
	claims := c.MustGet("claims").(*service.Claims)
	contentType := c.GetHeader("Content-Type")
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, generationMaxRequestBytes)
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"code": 413, "msg": "请求内容过大"})
		return
	}

	result, err := fn(claims.TenantID, claims.UserID, contentType, body, channelSelectionFromRequest(c))
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
	c.Data(result.StatusCode, result.Headers.Get("Content-Type"), result.Body)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func channelSelectionFromRequest(c *gin.Context) service.ChannelSelection {
	selection := service.ChannelSelection{
		ChannelID:         uintQuery(c, "channel_id"),
		ChannelModelID:    uintQuery(c, "channel_model_id"),
		ModelName:         strings.TrimSpace(c.Query("routing_model")),
		VideoRoute:        strings.TrimSpace(c.Query("routing_video_route")),
		MergeGroupName:    strings.TrimSpace(c.Query("fuzzy_group_name")),
		Capability:        strings.TrimSpace(c.Query("routing_capability")),
		AutoRoutingPoolID: uintQuery(c, "routing_pool_id"),
	}
	if selection.MergeGroupName != "" {
		selection.Kind = service.ModelSelectionMerge
	} else if selection.AutoRoutingPoolID > 0 {
		selection.Kind = service.ModelSelectionAuto
	} else if selection.ChannelID > 0 || selection.ChannelModelID > 0 {
		selection.Kind = service.ModelSelectionPhysical
	}
	return selection
}

func writeResolvedChannelHeaders(c *gin.Context, result *service.ProxyResult) {
	if result.RequestID != "" {
		c.Header("X-Generation-Request-ID", result.RequestID)
	}
	if result.ResolvedChannelID > 0 {
		c.Header("X-Resolved-Channel-ID", itoa(int(result.ResolvedChannelID)))
	}
	if result.ResolvedChannelModelID > 0 {
		c.Header("X-Resolved-Channel-Model-ID", itoa(int(result.ResolvedChannelModelID)))
	}
	if result.ResolvedChannelName != "" {
		c.Header("X-Resolved-Channel-Name", result.ResolvedChannelName)
	}
}

func copyProxyResponseHeaders(c *gin.Context, headers http.Header) {
	for key, values := range headers {
		if !shouldCopyProxyHeader(key) {
			continue
		}
		for _, value := range values {
			c.Header(key, value)
		}
	}
}

func shouldCopyProxyHeader(key string) bool {
	switch http.CanonicalHeaderKey(key) {
	case "Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization", "Te", "Trailer", "Transfer-Encoding", "Upgrade",
		"Content-Length", "Content-Encoding",
		"X-Credits-Cost", "X-Credits-Balance", "X-Credits-Refund", "X-Generation-Request-Id",
		"X-Resolved-Channel-Id", "X-Resolved-Channel-Model-Id", "X-Resolved-Channel-Name",
		"Access-Control-Allow-Origin", "Access-Control-Allow-Methods", "Access-Control-Allow-Headers", "Access-Control-Expose-Headers",
		"Access-Control-Allow-Credentials", "Access-Control-Max-Age":
		return false
	default:
		return true
	}
}

func uintQuery(c *gin.Context, key string) uint {
	value, _ := strconv.ParseUint(c.Query(key), 10, 64)
	return uint(value)
}
