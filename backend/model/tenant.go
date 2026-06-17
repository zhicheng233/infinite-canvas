package model

type TenantPlan   string
type TenantStatus string

const (
	PlanFree       TenantPlan = "free"
	PlanBasic      TenantPlan = "basic"
	PlanPro        TenantPlan = "pro"
	PlanEnterprise TenantPlan = "enterprise"

	TenantActive   TenantStatus = "active"
	TenantInactive TenantStatus = "inactive"
)

type Tenant struct {
	BaseModel
	Name   string       `gorm:"size:100;not null" json:"name"`
	Domain string       `gorm:"size:200;uniqueIndex" json:"domain"`
	Plan   TenantPlan   `gorm:"size:20;default:free" json:"plan"`
	Status TenantStatus `gorm:"size:20;default:active" json:"status"`
}

func (Tenant) TableName() string { return "tenants" }
