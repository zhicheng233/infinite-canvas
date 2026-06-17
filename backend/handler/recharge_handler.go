package handler

import (
	"strconv"

	"infinite-canvas-server/model"
	"infinite-canvas-server/repository"
	"infinite-canvas-server/service"

	"github.com/gin-gonic/gin"
)

type RechargeHandler struct {
	rechargeRepo  *repository.RechargeRepo
	paymentSvc    service.PaymentGateway
	creditService *service.CreditService
}

func NewRechargeHandler(rechargeRepo *repository.RechargeRepo, paymentSvc service.PaymentGateway, creditService *service.CreditService) *RechargeHandler {
	return &RechargeHandler{rechargeRepo: rechargeRepo, paymentSvc: paymentSvc, creditService: creditService}
}

type CreateOrderInput struct {
	PayoutID string `json:"payout_id" binding:"required"`
}

func (h *RechargeHandler) CreateOrder(c *gin.Context) {
	claims := c.MustGet("claims").(*service.Claims)
	var input CreateOrderInput
	if err := c.ShouldBindJSON(&input); err != nil {
		model.Fail(c, 400, "无效的请求参数")
		return
	}

	payouts := service.GetDefaultPayouts()
	var chosen *service.CreditPayout
	for _, p := range payouts {
		if p.ID == input.PayoutID {
			chosen = &p
			break
		}
	}
	if chosen == nil {
		model.Fail(c, 400, "无效的套餐ID")
		return
	}

	order := &model.RechargeOrder{
		TenantID: claims.TenantID,
		UserID:   claims.UserID,
		Amount:   0,
		Credits:  chosen.Credits,
		Status:   "pending",
		Note:     chosen.Name,
	}
	if err := h.rechargeRepo.Create(order); err != nil {
		model.Fail(c, 500, err.Error())
		return
	}

	_, paymentRef, err := h.paymentSvc.CreateOrder(order)
	if err != nil {
		model.Fail(c, 500, "payment failed: "+err.Error())
		return
	}

	if paymentRef != "" {
		h.rechargeRepo.UpdatePaymentRef(order.ID, paymentRef)
	}

	// Reload order to get updated status
	updated, _ := h.rechargeRepo.FindByID(order.ID)
	model.OK(c, updated)
}

func (h *RechargeHandler) ListMyOrders(c *gin.Context) {
	claims := c.MustGet("claims").(*service.Claims)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	orders, total, err := h.rechargeRepo.ListByUser(claims.UserID, page, pageSize)
	if err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.OKPage(c, orders, total, page, pageSize)
}

func (h *RechargeHandler) ListPayouts(c *gin.Context) {
	payouts := service.GetDefaultPayouts()
	model.OK(c, payouts)
}
