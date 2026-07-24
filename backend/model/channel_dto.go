package model

import "time"

// ChannelInfo is returned to authenticated users (never contains ApiKey).
type ChannelInfo struct {
	ID              uint       `json:"id"`
	Name            string     `json:"name"`
	Enabled         bool       `json:"enabled"`
	NewApiChannelID *int       `json:"new_api_channel_id,omitempty"`
	MetricsBaseUrl  *string    `json:"metrics_base_url,omitempty"`
	SyncStatus      string     `json:"sync_status"`
	SyncError       string     `json:"sync_error,omitempty"`
	SyncedAt        *time.Time `json:"synced_at,omitempty"`
}

// ChannelAdminInfo is returned to SuperAdmin (has BaseUrl + HasKey, no raw key).
type ChannelAdminInfo struct {
	ChannelInfo
	BaseUrl string `json:"base_url"`
	HasKey  bool   `json:"has_key"`
	Remark  string `json:"remark,omitempty"`
}

// SaveChannelInput is the request body for SuperAdmin create/update.
type SaveChannelInput struct {
	Name            string  `json:"name"`
	BaseUrl         string  `json:"base_url"`
	ApiKey          string  `json:"api_key"` // empty means "keep existing"
	Enabled         *bool   `json:"enabled,omitempty"`
	NewApiChannelID *int    `json:"new_api_channel_id,omitempty"`
	MetricsBaseUrl  *string `json:"metrics_base_url,omitempty"`
	Remark          string  `json:"remark,omitempty"`
}

// ChannelModelInfo is a single model row (no key, no BaseUrl).
type ChannelModelInfo struct {
	ID                 uint     `json:"id"`
	ChannelID          uint     `json:"channel_id"`
	ModelName          string   `json:"model_name"`
	Capabilities       []string `json:"capabilities"`
	Enabled            bool     `json:"enabled"`
	ImageGenerateRoute string   `json:"image_generate_route"`
	ImageEditRoute     string   `json:"image_edit_route"`
	VideoRoute         string   `json:"video_route"`
	VideoDurations     []int    `json:"video_durations"`
	VideoCustomizable  bool     `json:"video_customizable"`
	SortOrder          int      `json:"sort_order"`
}

// UpdateChannelModelInput is for SuperAdmin enable/disable/route edits.
type UpdateChannelModelInput struct {
	Enabled            *bool     `json:"enabled,omitempty"`
	ImageGenerateRoute *string   `json:"image_generate_route,omitempty"`
	ImageEditRoute     *string   `json:"image_edit_route,omitempty"`
	VideoRoute         *string   `json:"video_route,omitempty"`
	VideoDurations     []int     `json:"video_durations,omitempty"`
	VideoCustomizable  *bool     `json:"video_customizable,omitempty"`
	SortOrder          *int      `json:"sort_order,omitempty"`
	Capabilities       []string  `json:"capabilities,omitempty"`
}

// ChannelCatalogItem is returned by GET /api-config/catalog for authenticated users.
// It contains only enabled and priced models keyed by channel.
type ChannelCatalogItem struct {
	ChannelID   uint               `json:"channel_id"`
	ChannelName string             `json:"channel_name"`
	Models      []ChannelModelInfo `json:"models"`
}

// MetricsModelRate is a single model's performance data.
// SuccessRate is nil when no data is available (distinct from 0.0).
type MetricsModelRate struct {
	ChannelModelID uint     `json:"channel_model_id"`
	ChannelID      uint     `json:"channel_id"`
	ModelName      string   `json:"model_name"`
	RequestCount   int      `json:"request_count"`
	SuccessCount   int      `json:"success_count"`
	SuccessRate    *float64 `json:"success_rate"` // nil = unavailable, 0.0 = real zero
	Status             string   `json:"status"`
	AvgLatencyMs       *float64 `json:"avg_latency_ms,omitempty"`
	AvgTps             *float64 `json:"avg_tps,omitempty"`
	RecentSuccessRates []int    `json:"recent_success_rates,omitempty"`
}

// MetricsChannelRate is one channel's aggregated metrics with nested models.
type MetricsChannelRate struct {
	ChannelID       uint               `json:"channel_id"` // application channel ID
	ChannelName     string             `json:"channel_name"`
	NewApiChannelID *int               `json:"new_api_channel_id,omitempty"`
	Models          []MetricsModelRate `json:"models"`
	RequestCount    int                `json:"request_count"`
	SuccessCount    int                `json:"success_count"`
	SuccessRate     *float64           `json:"success_rate"`
	Status          string             `json:"status"`
}

// MetricsResponse is the response for GET /backend-api/channels/metrics?hours=N.
type MetricsResponse struct {
	Channels  []MetricsChannelRate `json:"channels"`
	Hours     int                  `json:"hours"`
	Window    string               `json:"window"`
	Status    string               `json:"status"`
	Error     string               `json:"error,omitempty"`
	UpdatedAt time.Time            `json:"updated_at"`
}

// NewApiMetricsPayload is the raw new-api response shape (for unmarshaling).
type NewApiMetricsPayload struct {
	Data    NewApiMetricsData `json:"data"`
	Success bool              `json:"success"`
}

type NewApiMetricsData struct {
	Channels []NewApiChannelMetrics `json:"channels"`
}

type NewApiChannelMetrics struct {
	ChannelID    int                  `json:"channel_id"`
	RequestCount int                  `json:"request_count"`
	SuccessCount int                  `json:"success_count"`
	SuccessRate  float64              `json:"success_rate"`
	Models       []NewApiModelMetrics `json:"models"`
}

type NewApiModelMetrics struct {
	ModelName    string  `json:"model_name"`
	RequestCount int     `json:"request_count"`
	SuccessCount int     `json:"success_count"`
	SuccessRate  float64 `json:"success_rate"`
}

// NewApiSummaryPayload wraps the summary (old New-API) metrics response.
type NewApiSummaryPayload struct {
	Success bool              `json:"success"`
	Data    NewApiSummaryData `json:"data"`
}

// NewApiSummaryData holds the flat model list from the summary endpoint.
type NewApiSummaryData struct {
	Models []NewApiSummaryModelMetrics `json:"models"`
}

// NewApiSummaryModelMetrics represents per-model global metrics from summary.
type NewApiSummaryModelMetrics struct {
	ModelName          string  `json:"model_name"`
	AvgLatencyMs       float64 `json:"avg_latency_ms"`
	SuccessRate        float64 `json:"success_rate"`
	AvgTps             float64 `json:"avg_tps"`
	RecentSuccessRates []int   `json:"recent_success_rates"`
}

// MetricsURLConfig is the SuperAdmin-managed new-api metrics configuration.
type MetricsURLConfig struct {
	MetricsBaseURL string `json:"metrics_base_url"`
}
