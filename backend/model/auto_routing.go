package model

type AutoRoutingPool struct {
	BaseModel
	PublicModelName string                  `gorm:"size:200;uniqueIndex:idx_auto_pool_model_capability;not null" json:"model"`
	Capability      string                  `gorm:"size:20;uniqueIndex:idx_auto_pool_model_capability;not null" json:"capability"`
	ContractKey     string                  `gorm:"size:64;not null" json:"contract_key"`
	Enabled         bool                    `gorm:"default:false;index" json:"enabled"`
	MaxAttempts     int                     `gorm:"default:2" json:"max_attempts"`
	Members         []AutoRoutingPoolMember `gorm:"foreignKey:PoolID" json:"members,omitempty"`
}

func (AutoRoutingPool) TableName() string { return "auto_routing_pools" }

type AutoRoutingPoolMember struct {
	BaseModel
	PoolID         uint `gorm:"uniqueIndex:idx_auto_pool_member;index;not null" json:"pool_id"`
	ChannelModelID uint `gorm:"uniqueIndex:idx_auto_pool_member;index;not null" json:"channel_model_id"`
	Priority       int  `gorm:"default:0" json:"priority"`
	Enabled        bool `gorm:"index;not null" json:"enabled"`
}

func (AutoRoutingPoolMember) TableName() string { return "auto_routing_pool_members" }

type GenerationAttempt struct {
	BaseModel
	RequestID       string `gorm:"size:64;index;uniqueIndex:idx_generation_attempt_request;not null" json:"request_id"`
	AttemptNo       int    `gorm:"uniqueIndex:idx_generation_attempt_request;not null" json:"attempt_no"`
	PoolID          uint   `gorm:"index;not null" json:"pool_id"`
	ChannelID       uint   `gorm:"index;not null" json:"channel_id"`
	ChannelModelID  uint   `gorm:"index;not null" json:"channel_model_id"`
	StatusCode      int    `gorm:"index" json:"status_code"`
	ResponseTimeMs  int    `gorm:"default:0" json:"response_time_ms"`
	Success         bool   `gorm:"index;default:false" json:"success"`
	FailureCategory string `gorm:"size:30;index" json:"failure_category"`
	Retryable       bool   `gorm:"index;default:false" json:"retryable"`
	CountsForHealth bool   `gorm:"index;default:false" json:"counts_for_health"`
	ErrorMessage    string `gorm:"size:500" json:"error_message"`
}

func (GenerationAttempt) TableName() string { return "generation_attempts" }
