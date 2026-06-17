package model

type CreditTransactionType string
type CreditPricingUnit   string

const (
	TxTypeEarn   CreditTransactionType = "earn"
	TxTypeSpend  CreditTransactionType = "spend"
	TxTypeRefund CreditTransactionType = "refund"
	TxTypeAdjust CreditTransactionType = "adjust"

	UnitPerImage CreditPricingUnit = "per_image"
	UnitPerVideo CreditPricingUnit = "per_video"
	UnitPerToken CreditPricingUnit = "per_token"
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
	AccountID    uint                   `gorm:"index;not null" json:"account_id"`
	Type         CreditTransactionType  `gorm:"size:20;not null" json:"type"`
	Amount       int                    `gorm:"not null" json:"amount"`
	BalanceAfter int                    `gorm:"not null" json:"balance_after"`
	RefType      string                 `gorm:"size:50" json:"ref_type"`
	RefID        string                 `gorm:"size:100" json:"ref_id"`
	Note         string                 `gorm:"size:500" json:"note"`
}

func (CreditTransaction) TableName() string { return "credit_transactions" }

type CreditPricing struct {
	BaseModel
	TenantID       uint              `gorm:"index;not null" json:"tenant_id"`
	Model          string            `gorm:"size:100;not null" json:"model"`
	CreditsPerUnit int               `gorm:"not null" json:"credits_per_unit"`
	UnitType       CreditPricingUnit `gorm:"size:20;default:per_image" json:"unit_type"`
}

func (CreditPricing) TableName() string { return "credit_pricing" }
