package model

type WebhookConfig struct {
	BaseModel
	TenantID        uint   `gorm:"index;not null" json:"tenant_id"`
	Platform        string `gorm:"size:50;not null" json:"platform"`
	WebhookURL      string `gorm:"size:500;not null" json:"webhook_url"`
	Enabled         bool   `gorm:"default:true" json:"enabled"`
	TemplateDown    string `gorm:"type:text" json:"template_down"`
	TemplateUp      string `gorm:"type:text" json:"template_up"`
	IntervalSeconds int    `gorm:"default:300" json:"interval_seconds"`
	CooldownMinutes int    `gorm:"default:10" json:"cooldown_minutes"`
}

func (WebhookConfig) TableName() string { return "webhook_configs" }

type WebhookLog struct {
	BaseModel
	TenantID        uint   `gorm:"index;not null" json:"tenant_id"`
	Platform        string `gorm:"size:50;not null" json:"platform"`
	ModelName       string `gorm:"size:100" json:"model_name"`
	Status          string `gorm:"size:50;not null" json:"status"`
	Message         string `gorm:"type:text" json:"message"`
	Success         bool   `gorm:"not null;default:false" json:"success"`
	ResponseBody    string `gorm:"type:longtext" json:"response_body"`
	CooldownSkipped bool   `gorm:"default:false" json:"cooldown_skipped"`
}

func (WebhookLog) TableName() string { return "webhook_logs" }
