package model

import "time"

type EffectiveProtocolInfo struct {
	Source        string         `json:"source"`
	Adapter       string         `json:"adapter"`
	Config        map[string]any `json:"config"`
	ConfigVersion int            `json:"config_version"`
	ContractKey   string         `json:"contract_key"`
}

type ModelOperationInfo struct {
	Capability string                `json:"capability"`
	Operation  string                `json:"operation"`
	Enabled    bool                  `json:"enabled"`
	Mode       string                `json:"mode"`
	Adapter    string                `json:"adapter"`
	Config     map[string]any        `json:"config"`
	Effective  EffectiveProtocolInfo `json:"effective"`
}

type ModelPricingRuleInfo struct {
	ID              uint              `json:"id"`
	Capability      string            `json:"capability"`
	Scope           string            `json:"scope"`
	ScopeID         uint              `json:"scope_id"`
	CreditsPerUnit  int               `json:"credits_per_unit"`
	UnitType        CreditPricingUnit `json:"unit_type"`
	PricingMode     CreditPricingMode `json:"pricing_mode"`
	PricingRule     string            `json:"pricing_rule"`
	ConfigRevision  uint              `json:"config_revision"`
	EffectiveSource string            `json:"effective_source,omitempty"`
}

type ModelReadinessIssue struct {
	Code       string `json:"code"`
	Capability string `json:"capability,omitempty"`
	Operation  string `json:"operation,omitempty"`
	Message    string `json:"message"`
}

type ModelConfigInfo struct {
	ID               uint                   `json:"id"`
	ChannelID        uint                   `json:"channel_id"`
	ChannelName      string                 `json:"channel_name"`
	ChannelRemark    string                 `json:"channel_remark,omitempty"`
	CatalogModelID   uint                   `json:"catalog_model_id"`
	PublicKey        string                 `json:"public_key"`
	DisplayName      string                 `json:"display_name"`
	UpstreamModelID  string                 `json:"upstream_model_id"`
	Status           string                 `json:"status"`
	DiscoveryStatus  string                 `json:"discovery_status"`
	LastDiscoveredAt *time.Time             `json:"last_discovered_at,omitempty"`
	ConfigRevision   uint                   `json:"config_revision"`
	LegacyUnreviewed bool                   `json:"legacy_unreviewed"`
	Archived         bool                   `json:"archived"`
	SortOrder        int                    `json:"sort_order"`
	Operations       []ModelOperationInfo   `json:"operations"`
	Pricing          []ModelPricingRuleInfo `json:"pricing"`
	ReadinessIssues  []ModelReadinessIssue  `json:"readiness_issues"`
	Ready            bool                   `json:"ready"`
}

type SaveModelOperationInput struct {
	Capability string         `json:"capability"`
	Operation  string         `json:"operation"`
	Enabled    bool           `json:"enabled"`
	Mode       string         `json:"mode"`
	Adapter    string         `json:"adapter"`
	Config     map[string]any `json:"config"`
}

type SaveModelPricingInput struct {
	Capability     string            `json:"capability"`
	CreditsPerUnit int               `json:"credits_per_unit"`
	UnitType       CreditPricingUnit `json:"unit_type"`
	PricingMode    CreditPricingMode `json:"pricing_mode"`
	PricingRule    string            `json:"pricing_rule"`
}

type UpdateModelConfigInput struct {
	ExpectedRevision uint                      `json:"expected_revision"`
	PublicKey        string                    `json:"public_key"`
	DisplayName      string                    `json:"display_name"`
	UpstreamModelID  string                    `json:"upstream_model_id"`
	Status           string                    `json:"status"`
	SortOrder        int                       `json:"sort_order"`
	Operations       []SaveModelOperationInput `json:"operations"`
	PricingOverrides []SaveModelPricingInput   `json:"pricing_overrides"`
}

type SaveChannelProtocolDefaultInput struct {
	Capability string         `json:"capability"`
	Operation  string         `json:"operation"`
	Adapter    string         `json:"adapter"`
	Config     map[string]any `json:"config"`
}

type ModelServiceChannelInfo struct {
	ChannelAdminInfo
	Archived         bool                              `json:"archived"`
	ConfigRevision   uint                              `json:"config_revision"`
	ProtocolDefaults []SaveChannelProtocolDefaultInput `json:"protocol_defaults"`
	ModelCount       int                               `json:"model_count"`
	ReadyModelCount  int                               `json:"ready_model_count"`
}

type UpdateChannelDefaultsInput struct {
	ExpectedRevision uint                              `json:"expected_revision"`
	Defaults         []SaveChannelProtocolDefaultInput `json:"defaults"`
}

type ChannelDefaultsImpact struct {
	AffectedModelIDs []uint                `json:"affected_model_ids"`
	Issues           []ModelReadinessIssue `json:"issues"`
}

type ChannelSyncReport struct {
	Discovered int    `json:"discovered"`
	Created    int    `json:"created"`
	Restored   int    `json:"restored"`
	Missing    int    `json:"missing"`
	Unchanged  int    `json:"unchanged"`
	ModelIDs   []uint `json:"model_ids"`
}

type ModelTestDraftInput struct {
	Capability     string                  `json:"capability"`
	Operation      string                  `json:"operation"`
	Prompt         string                  `json:"prompt"`
	Size           string                  `json:"size"`
	AspectRatio    string                  `json:"aspect_ratio"`
	Seconds        int                     `json:"seconds"`
	ReferenceCount int                     `json:"reference_count"`
	Draft          *UpdateModelConfigInput `json:"draft,omitempty"`
}
