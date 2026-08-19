package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"infinite-canvas-server/model"
	"infinite-canvas-server/repository"
	"infinite-canvas-server/service"
)

type UserHandler struct {
	userService *service.UserService
}

func NewUserHandler(userService *service.UserService) *UserHandler {
	return &UserHandler{userService: userService}
}

func (h *UserHandler) List(c *gin.Context) {
	claims := c.MustGet("claims").(*service.Claims)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	query := (repository.UserListQuery{TenantID: claims.TenantID, Page: page, PageSize: pageSize, Keyword: c.Query("keyword")}).Normalize()
	users, total, err := h.userService.ListUsers(query)
	if err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.OKPage(c, users, total, query.Page, query.PageSize)
}

type resetUserPasswordInput struct {
	NewPassword string `json:"new_password"`
}

func (h *UserHandler) ResetPassword(c *gin.Context) {
	targetID, ok := parseUserLifecycleID(c)
	if !ok {
		return
	}
	var input resetUserPasswordInput
	if err := c.ShouldBindJSON(&input); err != nil {
		model.Fail(c, http.StatusBadRequest, "无效的请求参数")
		return
	}
	claims := c.MustGet("claims").(*service.Claims)
	if err := h.userService.ResetPassword(claims, targetID, input.NewPassword); err != nil {
		writeUserLifecycleError(c, err)
		return
	}
	model.OK(c, nil)
}

func (h *UserHandler) Delete(c *gin.Context) {
	targetID, ok := parseUserLifecycleID(c)
	if !ok {
		return
	}
	claims := c.MustGet("claims").(*service.Claims)
	if err := h.userService.DeleteAccount(claims, targetID); err != nil {
		writeUserLifecycleError(c, err)
		return
	}
	model.OK(c, gin.H{"deleted": true})
}

func parseUserLifecycleID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		model.Fail(c, http.StatusBadRequest, "无效的用户 ID")
		return 0, false
	}
	return uint(id), true
}

func writeUserLifecycleError(c *gin.Context, err error) {
	if errors.Is(err, service.ErrUserLifecycleNotFound) || errors.Is(err, gorm.ErrRecordNotFound) {
		model.Fail(c, http.StatusNotFound, service.ErrUserLifecycleNotFound.Error())
		return
	}
	if errors.Is(err, service.ErrUserLifecycleSelfTarget) || errors.Is(err, service.ErrUserLifecycleAdminRequired) {
		model.Fail(c, http.StatusForbidden, err.Error())
		return
	}
	if errors.Is(err, service.ErrUserLifecyclePersistence) {
		model.Fail(c, http.StatusInternalServerError, service.ErrUserLifecyclePersistence.Error())
		return
	}
	model.Fail(c, http.StatusBadRequest, err.Error())
}
