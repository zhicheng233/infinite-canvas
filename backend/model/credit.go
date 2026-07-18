package model

import "encoding/json"

type CreditTransactionType string
type CreditPricingUnit string
type CreditPricingMode string

const (
	TxTypeEarn   CreditTransactionType = "earn"
	TxTypeSpend  CreditTransactionType = "spend"
	TxTypeRefund CreditTransactionType = "refund"
	TxTypeAdjust CreditTransactionType = "adjust"

	UnitPerImage       CreditPricingUnit = "per_image"
	UnitPerVideo       CreditPricingUnit = "per_video"
	UnitPerVideoSecond CreditPricingUnit = "per_video_second"
	UnitPerToken       CreditPricingUnit = "per_token"

	PricingModePerUnit      CreditPricingMode = "per_unit"
	PricingModeVideoDynamic CreditPricingMode = "video_dynamic"
)

type CreditAccount struct {
	BaseModel
	TenantID    uint `gorm:"index;not null" json:"tenant_id"`
	UserID      uint `gorm:"uniqueIndex;not null" json:"user_id"`
	Balance     int  `gorm:"default:0" json:"balance"`
	TotalEarned int  `gorm:"default:0" json:"total_earned"`
	TotalSpent  int  `gorm:"default:0" json:"total_spent"`
}

func (CreditAccount) TableName() string { return "credit_accounts" }

type CreditTransaction struct {
	BaseModel
	AccountID     uint                  `gorm:"index;not null" json:"account_id"`
	Type          CreditTransactionType `gorm:"size:20;not null" json:"type"`
	Amount        int                   `gorm:"not null" json:"amount"`
	BalanceBefore *int                  `json:"balance_before,omitempty"`
	BalanceAfter  int                   `gorm:"not null" json:"balance_after"`
	RefType       string                `gorm:"size:50" json:"ref_type"`
	RefID         string                `gorm:"size:100" json:"ref_id"`
	Note          string                `gorm:"size:500" json:"note"`
	Metadata      string                `gorm:"type:longtext" json:"metadata"`
}

func (CreditTransaction) TableName() string { return "credit_transactions" }

type CreditPricing struct {
	BaseModel
	TenantID       uint              `gorm:"index;not null" json:"tenant_id"`
	Model          string            `gorm:"size:100;not null" json:"model"`
	CreditsPerUnit int               `gorm:"not null" json:"credits_per_unit"`
	UnitType       CreditPricingUnit `gorm:"size:20;default:per_image" json:"unit_type"`
	PricingMode    CreditPricingMode `gorm:"size:30;default:per_unit" json:"pricing_mode"`
	PricingRule    string            `gorm:"type:longtext" json:"pricing_rule"`
}

func (CreditPricing) TableName() string { return "credit_pricing" }

type VideoPricingRule struct {
	BaseCredits           int            `json:"base_credits"`
	ResolutionSecondRates map[string]int `json:"resolution_second_rates"`
}

func (p CreditPricing) HasValidPricingRule() bool {
	if p.PricingMode == PricingModeVideoDynamic || p.UnitType == UnitPerVideoSecond {
		var rule VideoPricingRule
		if p.PricingRule == "" || json.Unmarshal([]byte(p.PricingRule), &rule) != nil {
			return false
		}
		for _, rate := range rule.ResolutionSecondRates {
			if rate > 0 {
				return true
			}
		}
		return false
	}
	return p.CreditsPerUnit > 0
}
