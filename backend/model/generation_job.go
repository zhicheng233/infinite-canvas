package model

type GenerationJobStatus string

const (
	GenerationJobReserved  GenerationJobStatus = "reserved"
	GenerationJobPending   GenerationJobStatus = "pending"
	GenerationJobSucceeded GenerationJobStatus = "succeeded"
	GenerationJobRefunded  GenerationJobStatus = "refunded"
)

type GenerationJob struct {
	BaseModel
	RequestID               string              `gorm:"size:64;uniqueIndex;not null" json:"request_id"`
	TenantID                uint                `gorm:"index;not null" json:"tenant_id"`
	UserID                  uint                `gorm:"index;not null" json:"user_id"`
	Capability              string              `gorm:"size:20;index;not null" json:"capability"`
	ModelName               string              `gorm:"size:191;not null" json:"model_name"`
	AutoRoutingPoolID       uint                `gorm:"index;not null;default:0" json:"auto_routing_pool_id"`
	ChannelID               uint                `gorm:"index;not null" json:"channel_id"`
	ChannelModelID          uint                `gorm:"index;not null" json:"channel_model_id"`
	ChannelName             string              `gorm:"size:100" json:"channel_name"`
	ChannelBaseURL          string              `gorm:"size:500" json:"channel_base_url"`
	VideoRoute              string              `gorm:"size:50" json:"video_route"`
	AuthorizedAmount        int                 `gorm:"not null" json:"authorized_amount"`
	BillingAmount           int                 `gorm:"not null" json:"billing_amount"`
	SpendTransactionID      uint                `gorm:"index" json:"spend_transaction_id"`
	SettlementTransactionID uint                `gorm:"index" json:"settlement_transaction_id"`
	RefundTransactionID     uint                `gorm:"index" json:"refund_transaction_id"`
	UpstreamTaskID          string              `gorm:"size:191;index" json:"upstream_task_id"`
	Status                  GenerationJobStatus `gorm:"size:20;index;not null" json:"status"`
	FailureReason           string              `gorm:"size:500" json:"failure_reason"`
}

func (GenerationJob) TableName() string { return "generation_jobs" }
