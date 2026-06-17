package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"infinite-canvas-server/model"
	"infinite-canvas-server/service"
)

type AuthHandler struct {
	authService *service.AuthService
	userService *service.UserService
}

func NewAuthHandler(authService *service.AuthService, userService *service.UserService) *AuthHandler {
	return &AuthHandler{authService: authService, userService: userService}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var input service.RegisterInput
	if err := c.ShouldBindJSON(&input); err != nil {
		model.Fail(c, 400, "无效的请求参数")
		return
	}
	result, err := h.authService.Register(input)
	if err != nil {
		model.Fail(c, 400, err.Error())
		return
	}
	model.OK(c, result)
}

func (h *AuthHandler) Login(c *gin.Context) {
	var input service.LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		model.Fail(c, 400, "无效的请求参数")
		return
	}
	result, err := h.authService.Login(input)
	if err != nil {
		model.Fail(c, 400, err.Error())
		return
	}
	model.OK(c, result)
}

func (h *AuthHandler) Me(c *gin.Context) {
	claims := c.MustGet("claims").(*service.Claims)
	user, err := h.userService.GetUser(claims.UserID)
	if err != nil {
		model.FailStatus(c, http.StatusUnauthorized, 401, "user not found")
		return
	}
	model.OK(c, user)
}
