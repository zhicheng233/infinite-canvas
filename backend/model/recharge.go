package model

type RechargeOrder struct {
	BaseModel
	TenantID     uint   `gorm:"index;not null" json:"tenant_id"`
	UserID       uint   `gorm:"index;not null" json:"user_id"`
	Amount       int    `gorm:"not null" json:"amount"`
	Credits      int    `gorm:"not null" json:"credits"`
	Status       string `gorm:"size:20;default:pending" json:"status"`
	PaymentRef   string `gorm:"size:200" json:"payment_ref"`
	Note         string `gorm:"size:500" json:"note"`
}

func (RechargeOrder) TableName() string { return "recharge_orders" }
