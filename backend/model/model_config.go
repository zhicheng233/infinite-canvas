package model

import "time"

const (
	ModelStatusDraft    = "draft"
	ModelStatusActive   = "active"
	ModelStatusDisabled = "disabled"

	DiscoveryStatusPresent = "present"
	DiscoveryStatusMissing = "missing"

	ProtocolModeInherit  = "inherit"
	ProtocolModeOverride = "override"

	PricingScopeDefault        = "default"
	PricingScopeImplementation = "implementation"
)

type CatalogModel struct {
	BaseModel
	PublicKey   string `gorm:"size:200;uniqueIndex;not null" json:"public_key"`
	DisplayName string `gorm:"size:200;not null" json:"display_name"`
}

func (CatalogModel) TableName() string { return "catalog_models" }

type ChannelModelOperation struct {
	BaseModel
	ChannelModelID uint   `gorm:"uniqueIndex:idx_channel_model_operation;index;not null" json:"channel_model_id"`
	Capability     string `gorm:"size:20;uniqueIndex:idx_channel_model_operation;index;not null" json:"capability"`
	Operation      string `gorm:"size:30;uniqueIndex:idx_channel_model_operation;not null" json:"operation"`
	Enabled        bool   `gorm:"index" json:"enabled"`
	ProtocolMode   string `gorm:"size:20;not null" json:"protocol_mode"`
	Adapter        string `gorm:"size:50" json:"adapter"`
	ConfigJSON     string `gorm:"type:longtext" json:"config_json"`
	ConfigVersion  int    `gorm:"default:1" json:"config_version"`
	ContractKey    string `gorm:"size:64;index" json:"contract_key"`
}

func (ChannelModelOperation) TableName() string { return "channel_model_operations" }

type ChannelProtocolDefault struct {
	BaseModel
	ChannelID     uint   `gorm:"uniqueIndex:idx_channel_protocol_default;index;not null" json:"channel_id"`
	Capability    string `gorm:"size:20;uniqueIndex:idx_channel_protocol_default;not null" json:"capability"`
	Operation     string `gorm:"size:30;uniqueIndex:idx_channel_protocol_default;not null" json:"operation"`
	Adapter       string `gorm:"size:50;not null" json:"adapter"`
	ConfigJSON    string `gorm:"type:longtext" json:"config_json"`
	ConfigVersion int    `gorm:"default:1" json:"config_version"`
}

func (ChannelProtocolDefault) TableName() string { return "channel_protocol_defaults" }

type ModelPricingRule struct {
	BaseModel
	TenantID       uint              `gorm:"uniqueIndex:idx_model_pricing_rule;index;not null" json:"tenant_id"`
	CatalogModelID uint              `gorm:"uniqueIndex:idx_model_pricing_rule;index;not null" json:"catalog_model_id"`
	Capability     string            `gorm:"size:20;uniqueIndex:idx_model_pricing_rule;not null" json:"capability"`
	Scope          string            `gorm:"size:20;uniqueIndex:idx_model_pricing_rule;not null" json:"scope"`
	ScopeID        uint              `gorm:"uniqueIndex:idx_model_pricing_rule;not null" json:"scope_id"`
	CreditsPerUnit int               `gorm:"not null" json:"credits_per_unit"`
	UnitType       CreditPricingUnit `gorm:"size:20;not null" json:"unit_type"`
	PricingMode    CreditPricingMode `gorm:"size:30;not null" json:"pricing_mode"`
	PricingRule    string            `gorm:"type:longtext" json:"pricing_rule"`
	ConfigRevision uint              `gorm:"default:1" json:"config_revision"`
}

func (ModelPricingRule) TableName() string { return "model_pricing_rules" }

func (p ModelPricingRule) HasValidPricingRule() bool {
	return CreditPricing{CreditsPerUnit: p.CreditsPerUnit, UnitType: p.UnitType, PricingMode: p.PricingMode, PricingRule: p.PricingRule}.HasValidPricingRule()
}

type ModelConfigAuditLog struct {
	BaseModel
	TenantID    uint   `gorm:"index;not null" json:"tenant_id"`
	ActorUserID uint   `gorm:"index;not null" json:"actor_user_id"`
	Resource    string `gorm:"size:50;index;not null" json:"resource"`
	ResourceID  uint   `gorm:"index;not null" json:"resource_id"`
	Action      string `gorm:"size:30;index;not null" json:"action"`
	BeforeJSON  string `gorm:"type:longtext" json:"before_json"`
	AfterJSON   string `gorm:"type:longtext" json:"after_json"`
}

func (ModelConfigAuditLog) TableName() string { return "model_config_audit_logs" }

type ModelConfigMigration struct {
	BaseModel
	Source      string     `gorm:"size:50;uniqueIndex:idx_model_config_migration;not null" json:"source"`
	SourceID    uint       `gorm:"uniqueIndex:idx_model_config_migration;not null" json:"source_id"`
	Version     int        `gorm:"uniqueIndex:idx_model_config_migration;not null" json:"version"`
	Status      string     `gorm:"size:20;index;not null" json:"status"`
	TargetID    uint       `gorm:"index" json:"target_id"`
	Detail      string     `gorm:"type:longtext" json:"detail"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

func (ModelConfigMigration) TableName() string { return "model_config_migrations" }

type ModelConfigMigrationIssue struct {
	BaseModel
	MigrationID uint   `gorm:"index;not null" json:"migration_id"`
	Resource    string `gorm:"size:50;index;not null" json:"resource"`
	Identifier  string `gorm:"size:250;not null" json:"identifier"`
	Reason      string `gorm:"size:500;not null" json:"reason"`
	PayloadJSON string `gorm:"type:longtext" json:"payload_json"`
	Resolved    bool   `gorm:"default:false;index" json:"resolved"`
}

func (ModelConfigMigrationIssue) TableName() string { return "model_config_migration_issues" }
