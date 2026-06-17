package handler

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"infinite-canvas-server/service"
)

type GenerateHandler struct {
	generateService *service.GenerateService
}

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

type proxyFunc func(tenantID, userID uint, contentType string, body []byte) (*service.ProxyResult, error)

func (h *GenerateHandler) handleProxy(c *gin.Context, fn proxyFunc) {
	claims := c.MustGet("claims").(*service.Claims)
	contentType := c.GetHeader("Content-Type")
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "failed to read body"})
		return
	}

	result, err := fn(claims.TenantID, claims.UserID, contentType, body)
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
