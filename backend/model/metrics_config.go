package model

type MetricsConfig struct {
	BaseModel
	MetricsBaseURL string `gorm:"size:500" json:"metrics_base_url"`
}

func (MetricsConfig) TableName() string { return "metrics_configs" }
