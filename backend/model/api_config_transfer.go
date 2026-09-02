package model

import "time"

const (
	APIConfigTransferFormat        = "infinite-canvas-model-api-config"
	APIConfigTransferFormatVersion = 1
)

type APIConfigTransferKDF struct {
	Name        string `json:"name"`
	Time        uint32 `json:"time"`
	MemoryKiB   uint32 `json:"memory_kib"`
	Parallelism uint8  `json:"parallelism"`
}

type APIConfigTransferEnvelope struct {
	Format     string               `json:"format"`
	Version    int                  `json:"version"`
	Cipher     string               `json:"cipher"`
	KDF        APIConfigTransferKDF `json:"kdf"`
	Salt       string               `json:"salt"`
	Nonce      string               `json:"nonce"`
	Ciphertext string               `json:"ciphertext"`
}

type APIConfigTransferExportInput struct {
	Password string `json:"password"`
}

type APIConfigTransferImportInput struct {
	Password string                    `json:"password"`
	Envelope APIConfigTransferEnvelope `json:"envelope"`
}

type APIConfigTransferSnapshot struct {
	SchemaVersion    int                                `json:"schema_version"`
	ExportedAt       time.Time                          `json:"exported_at"`
	Channels         []APIConfigTransferChannel         `json:"channels"`
	Pricing          []APIConfigTransferPricing         `json:"pricing"`
	PricingRules     []APIConfigTransferPricingRule     `json:"model_pricing_rules,omitempty"`
	VideoPresets     []APIConfigTransferVideoPreset     `json:"video_config_presets"`
	AutoRoutingPools []APIConfigTransferAutoRoutingPool `json:"auto_routing_pools,omitempty"`
}

type APIConfigTransferChannel struct {
	Ref              string                        `json:"channel_ref"`
	Name             string                        `json:"name"`
	BaseURL          string                        `json:"base_url"`
	APIKey           string                        `json:"api_key,omitempty"`
	Enabled          bool                          `json:"enabled"`
	VideoAPIStandard string                        `json:"video_api_standard"`
	NewAPIChannelID  *int                          `json:"new_api_channel_id,omitempty"`
	MetricsBaseURL   *string                       `json:"metrics_base_url,omitempty"`
	Remark           string                        `json:"remark,omitempty"`
	ConfigRevision   uint                          `json:"config_revision,omitempty"`
	ProtocolDefaults []APIConfigTransferProtocol   `json:"protocol_defaults,omitempty"`
	Models           []APIConfigTransferModel      `json:"models"`
	MergeGroups      []APIConfigTransferMergeGroup `json:"merge_groups"`
}

type APIConfigTransferModel struct {
	ModelName          string                       `json:"model_name"`
	PublicKey          string                       `json:"public_key,omitempty"`
	DisplayName        string                       `json:"display_name,omitempty"`
	UpstreamModelID    string                       `json:"upstream_model_id,omitempty"`
	Status             string                       `json:"status,omitempty"`
	DiscoveryStatus    string                       `json:"discovery_status,omitempty"`
	ConfigRevision     uint                         `json:"config_revision,omitempty"`
	LegacyUnreviewed   bool                         `json:"legacy_unreviewed,omitempty"`
	Operations         []APIConfigTransferOperation `json:"operations,omitempty"`
	Capabilities       []string                     `json:"capabilities"`
	Enabled            bool                         `json:"enabled"`
	ImageGenerateRoute string                       `json:"image_generate_route"`
	ImageEditRoute     string                       `json:"image_edit_route"`
	VideoRoute         string                       `json:"video_route"`
	VideoDurations     []int                        `json:"video_durations"`
	VideoCustomizable  bool                         `json:"video_customizable"`
	VideoCustomConfig  *CustomVideoConfig           `json:"video_custom_config,omitempty"`
	SortOrder          int                          `json:"sort_order"`
}

type APIConfigTransferProtocol struct {
	Capability    string         `json:"capability"`
	Operation     string         `json:"operation"`
	Adapter       string         `json:"adapter"`
	Config        map[string]any `json:"config,omitempty"`
	ConfigVersion int            `json:"config_version,omitempty"`
}

type APIConfigTransferOperation struct {
	Capability    string         `json:"capability"`
	Operation     string         `json:"operation"`
	Enabled       bool           `json:"enabled"`
	ProtocolMode  string         `json:"protocol_mode"`
	Adapter       string         `json:"adapter,omitempty"`
	Config        map[string]any `json:"config,omitempty"`
	ConfigVersion int            `json:"config_version,omitempty"`
	ContractKey   string         `json:"contract_key,omitempty"`
}

type APIConfigTransferPricing struct {
	Model          string            `json:"model"`
	ChannelRef     string            `json:"channel_ref,omitempty"`
	CreditsPerUnit int               `json:"credits_per_unit"`
	UnitType       CreditPricingUnit `json:"unit_type"`
	PricingMode    CreditPricingMode `json:"pricing_mode"`
	PricingRule    string            `json:"pricing_rule,omitempty"`
}

type APIConfigTransferPricingRule struct {
	PublicKey       string            `json:"public_key"`
	Capability      string            `json:"capability"`
	Scope           string            `json:"scope"`
	ChannelRef      string            `json:"channel_ref,omitempty"`
	UpstreamModelID string            `json:"upstream_model_id,omitempty"`
	CreditsPerUnit  int               `json:"credits_per_unit"`
	UnitType        CreditPricingUnit `json:"unit_type"`
	PricingMode     CreditPricingMode `json:"pricing_mode"`
	PricingRule     string            `json:"pricing_rule,omitempty"`
	ConfigRevision  uint              `json:"config_revision,omitempty"`
}

type APIConfigTransferMergeGroup struct {
	GroupName string `json:"group_name"`
	Pattern   string `json:"pattern"`
	Enabled   bool   `json:"enabled"`
}

type APIConfigTransferVideoPreset struct {
	Name   string            `json:"name"`
	Config CustomVideoConfig `json:"config"`
}

type APIConfigTransferAutoRoutingPool struct {
	Model       string                               `json:"model"`
	Capability  string                               `json:"capability"`
	ContractKey string                               `json:"contract_key"`
	Enabled     bool                                 `json:"enabled"`
	MaxAttempts int                                  `json:"max_attempts"`
	Members     []APIConfigTransferAutoRoutingMember `json:"members"`
}

type APIConfigTransferAutoRoutingMember struct {
	ChannelRef string `json:"channel_ref"`
	Model      string `json:"model"`
	Priority   int    `json:"priority"`
	Enabled    bool   `json:"enabled"`
}

type APIConfigTransferChangeStats struct {
	Create int `json:"create"`
	Update int `json:"update"`
	Skip   int `json:"skip"`
}

type APIConfigTransferStats struct {
	Channels         APIConfigTransferChangeStats `json:"channels"`
	Models           APIConfigTransferChangeStats `json:"models"`
	Pricing          APIConfigTransferChangeStats `json:"pricing"`
	MergeGroups      APIConfigTransferChangeStats `json:"merge_groups"`
	VideoPresets     APIConfigTransferChangeStats `json:"video_config_presets"`
	AutoRoutingPools APIConfigTransferChangeStats `json:"auto_routing_pools"`
}

type APIConfigTransferConflict struct {
	Resource   string `json:"resource"`
	Identifier string `json:"identifier"`
	Reason     string `json:"reason"`
}

type APIConfigTransferResult struct {
	Stats     APIConfigTransferStats      `json:"stats"`
	Conflicts []APIConfigTransferConflict `json:"conflicts"`
	Applied   bool                        `json:"applied"`
}

type APIConfigTransferExportResult struct {
	FileName string                      `json:"file_name"`
	Envelope APIConfigTransferEnvelope   `json:"envelope"`
	Summary  APIConfigTransferStats      `json:"summary"`
	Warnings []APIConfigTransferConflict `json:"warnings"`
}
