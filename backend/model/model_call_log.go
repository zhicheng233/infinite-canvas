package model

type ModelCallLog struct {
	BaseModel
	TenantID     uint   `gorm:"index;not null" json:"tenant_id"`
	UserID       uint   `gorm:"index;not null" json:"user_id"`
	Username     string `gorm:"size:64" json:"username"`
	DisplayName  string `gorm:"size:64" json:"display_name"`
	Generation   string `gorm:"size:20;index" json:"generation"`
	Model        string `gorm:"size:100;index" json:"model"`
	Method       string `gorm:"size:10" json:"method"`
	Path         string `gorm:"size:255;index" json:"path"`
	StatusCode   int    `gorm:"index" json:"status_code"`
	ErrorMessage string `gorm:"size:500" json:"error_message"`
	ErrorBody    string `gorm:"type:longtext" json:"error_body"`
	IsSuccess    bool   `gorm:"index:idx_success_time;default:0" json:"is_success"`
	ResponseTime int    `gorm:"default:0" json:"response_time_ms"`
}

func (ModelCallLog) TableName() string { return "model_call_logs" }
