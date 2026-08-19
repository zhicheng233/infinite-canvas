package handler

import (
	"errors"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"infinite-canvas-server/model"
	"infinite-canvas-server/service"
)

type AdjustCreditsInput struct {
	UserID        uint                       `json:"user_id"`
	Mode          model.CreditAdjustmentMode `json:"mode"`
	Amount        int                        `json:"amount"`
	TargetBalance *int                       `json:"target_balance"`
	Note          string                     `json:"note"`
}

type CreditRechargeInput struct {
	UserID uint   `json:"user_id"`
	Amount int    `json:"amount"`
	Note   string `json:"note"`
}

func (h *AdminHandler) AdjustCredits(c *gin.Context) {
	h.adjustCredits(c, true)
}

func (h *AdminHandler) AdjustTenantCredits(c *gin.Context) {
	h.adjustCredits(c, false)
}

func (h *AdminHandler) RechargeTenantCredits(c *gin.Context) {
	claims := c.MustGet("claims").(*service.Claims)
	var input CreditRechargeInput
	if err := c.ShouldBindJSON(&input); err != nil {
		model.Fail(c, 400, "无效的请求参数")
		return
	}
	h.applyCreditAdjustment(c, service.AdministratorCreditAdjustment{
		OperatorUserID:   claims.UserID,
		OperatorTenantID: claims.TenantID,
		TargetUserID:     input.UserID,
		Mode:             model.CreditAdjustmentAdd,
		Amount:           input.Amount,
		Note:             input.Note,
	}, "充值成功")
}

func (h *AdminHandler) adjustCredits(c *gin.Context, crossTenantAllowed bool) {
	claims := c.MustGet("claims").(*service.Claims)
	var input AdjustCreditsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		model.Fail(c, 400, "无效的请求参数")
		return
	}
	if input.Mode == "" {
		model.Fail(c, 400, "无效的积分调整")
		return
	}
	h.applyCreditAdjustment(c, service.AdministratorCreditAdjustment{
		OperatorUserID:     claims.UserID,
		OperatorTenantID:   claims.TenantID,
		TargetUserID:       input.UserID,
		Mode:               input.Mode,
		Amount:             input.Amount,
		TargetBalance:      input.TargetBalance,
		Note:               input.Note,
		CrossTenantAllowed: crossTenantAllowed,
	}, "积分调整成功")
}

func (h *AdminHandler) applyCreditAdjustment(c *gin.Context, input service.AdministratorCreditAdjustment, successMessage string) {
	result, err := h.creditService.AdjustAdministratorCredits(input)
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			model.Fail(c, 404, "用户不存在")
		case errors.Is(err, service.ErrCreditTargetForbidden):
			model.Fail(c, 403, err.Error())
		case errors.Is(err, service.ErrInvalidCreditAdjustment), errors.Is(err, service.ErrCreditAdjustmentNoop), errors.Is(err, service.ErrInsufficientCredits):
			model.Fail(c, 400, err.Error())
		default:
			model.Fail(c, 500, err.Error())
		}
		return
	}
	model.OK(c, gin.H{
		"user_id": result.UserID,
		"amount":  result.Amount,
		"balance": result.Balance,
		"message": successMessage,
	})
}
