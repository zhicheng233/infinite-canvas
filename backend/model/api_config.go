package model

type TenantApiConfig struct {
	BaseModel
	TenantID uint   `gorm:"uniqueIndex;not null" json:"tenant_id"`
	BaseUrl  string `gorm:"size:500;not null" json:"base_url"`
	ApiKey   string `gorm:"size:500;not null" json:"api_key"`
}

func (TenantApiConfig) TableName() string { return "tenant_api_configs" }
