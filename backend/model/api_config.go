package model

type TenantApiConfig struct {
	BaseModel
	TenantID    uint   `gorm:"uniqueIndex;not null" json:"tenant_id"`
	BaseUrl     string `gorm:"size:500;not null" json:"base_url"`
	ApiKey      string `gorm:"size:500;not null" json:"api_key"`
	Models      string `gorm:"type:longtext" json:"models"`
	ImageModels string `gorm:"type:longtext" json:"image_models"`
	VideoModels string `gorm:"type:longtext" json:"video_models"`
	TextModels  string `gorm:"type:longtext" json:"text_models"`
	AudioModels string `gorm:"type:longtext" json:"audio_models"`
	ModelRoutes string `gorm:"type:longtext" json:"model_routes"`
}

func (TenantApiConfig) TableName() string { return "tenant_api_configs" }
