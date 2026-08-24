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
	SchemaVersion int                            `json:"schema_version"`
	ExportedAt    time.Time                      `json:"exported_at"`
	Channels      []APIConfigTransferChannel     `json:"channels"`
	Pricing       []APIConfigTransferPricing     `json:"pricing"`
	VideoPresets  []APIConfigTransferVideoPreset `json:"video_config_presets"`
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
	Models           []APIConfigTransferModel      `json:"models"`
	MergeGroups      []APIConfigTransferMergeGroup `json:"merge_groups"`
}

type APIConfigTransferModel struct {
	ModelName          string             `json:"model_name"`
	Capabilities       []string           `json:"capabilities"`
	Enabled            bool               `json:"enabled"`
	ImageGenerateRoute string             `json:"image_generate_route"`
	ImageEditRoute     string             `json:"image_edit_route"`
	VideoRoute         string             `json:"video_route"`
	VideoDurations     []int              `json:"video_durations"`
	VideoCustomizable  bool               `json:"video_customizable"`
	VideoCustomConfig  *CustomVideoConfig `json:"video_custom_config,omitempty"`
	SortOrder          int                `json:"sort_order"`
}

type APIConfigTransferPricing struct {
	Model          string            `json:"model"`
	ChannelRef     string            `json:"channel_ref,omitempty"`
	CreditsPerUnit int               `json:"credits_per_unit"`
	UnitType       CreditPricingUnit `json:"unit_type"`
	PricingMode    CreditPricingMode `json:"pricing_mode"`
	PricingRule    string            `json:"pricing_rule,omitempty"`
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

type APIConfigTransferChangeStats struct {
	Create int `json:"create"`
	Update int `json:"update"`
	Skip   int `json:"skip"`
}

type APIConfigTransferStats struct {
	Channels     APIConfigTransferChangeStats `json:"channels"`
	Models       APIConfigTransferChangeStats `json:"models"`
	Pricing      APIConfigTransferChangeStats `json:"pricing"`
	MergeGroups  APIConfigTransferChangeStats `json:"merge_groups"`
	VideoPresets APIConfigTransferChangeStats `json:"video_config_presets"`
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
