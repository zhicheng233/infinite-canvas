package service

import (
	"fmt"
	"infinite-canvas-server/model"
	"infinite-canvas-server/repository"
	"time"
)

// PaymentGateway abstracts payment processing
type PaymentGateway interface {
	CreateOrder(order *model.RechargeOrder) (paymentURL string, paymentRef string, err error)
	VerifyCallback(paymentRef string, params map[string]string) (success bool, err error)
}

// MockPaymentGateway simulates payments for development
type MockPaymentGateway struct {
	rechargeRepo *repository.RechargeRepo
	creditSvc    *CreditService
}

func NewMockPaymentGateway(rechargeRepo *repository.RechargeRepo, creditSvc *CreditService) *MockPaymentGateway {
	return &MockPaymentGateway{rechargeRepo: rechargeRepo, creditSvc: creditSvc}
}

func (g *MockPaymentGateway) CreateOrder(order *model.RechargeOrder) (string, string, error) {
	ref := fmt.Sprintf("MOCK-%d-%d", order.ID, time.Now().UnixMilli())
	order.PaymentRef = ref
	order.Status = "paid"
	if err := g.rechargeRepo.UpdateStatus(order.ID, "paid"); err != nil {
		return "", "", err
	}
	// Auto-complete: add credits directly
	if err := g.creditSvc.Earn(order.UserID, order.Credits, "recharge", fmt.Sprintf("%d", order.ID), order.Note); err != nil {
		order.Status = "failed"
		g.rechargeRepo.UpdateStatus(order.ID, "failed")
		return "", "", fmt.Errorf("earn failed: %w", err)
	}
	order.Status = "completed"
	g.rechargeRepo.UpdateStatus(order.ID, "completed")
	return "", ref, nil
}

func (g *MockPaymentGateway) VerifyCallback(paymentRef string, params map[string]string) (bool, error) {
	return true, nil
}

// CreditPayout represents credit package pricing for user purchase
type CreditPayout struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Credits int    `json:"credits"`
	Price   string `json:"price"` // Display price in CNY
}

func GetDefaultPayouts() []CreditPayout {
	return []CreditPayout{
		{ID: "basic", Name: "基础套餐", Credits: 100, Price: "¥10"},
		{ID: "standard", Name: "标准套餐", Credits: 300, Price: "¥25"},
		{ID: "pro", Name: "进阶套餐", Credits: 800, Price: "¥60"},
		{ID: "premium", Name: "高级套餐", Credits: 2000, Price: "¥128"},
		{ID: "ultimate", Name: "旗舰套餐", Credits: 5000, Price: "¥298"},
	}
}
