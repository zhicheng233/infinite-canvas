package model

import "time"

type ChannelModelCapability string

const (
	CapabilityImage ChannelModelCapability = "image"
	CapabilityVideo ChannelModelCapability = "video"
	CapabilityText  ChannelModelCapability = "text"
	CapabilityAudio ChannelModelCapability = "audio"
)

type ChannelModel struct {
	BaseModel
	ChannelID          uint       `gorm:"uniqueIndex:idx_channel_model;index;not null" json:"channel_id"`
	ModelName          string     `gorm:"size:200;uniqueIndex:idx_channel_model;not null" json:"model_name"`
	CatalogModelID     uint       `gorm:"index" json:"catalog_model_id"`
	UpstreamModelID    string     `gorm:"size:200;index" json:"upstream_model_id"`
	Status             string     `gorm:"size:20;index;default:draft" json:"status"`
	DiscoveryStatus    string     `gorm:"size:20;index;default:present" json:"discovery_status"`
	LastDiscoveredAt   *time.Time `json:"last_discovered_at,omitempty"`
	ConfigRevision     uint       `gorm:"default:1" json:"config_revision"`
	LegacyUnreviewed   bool       `gorm:"default:false;index" json:"legacy_unreviewed"`
	Capabilities       string     `gorm:"size:100" json:"capabilities"`
	Enabled            bool       `gorm:"default:true" json:"enabled"`
	ImageGenerateRoute string     `gorm:"size:30" json:"image_generate_route"`
	ImageEditRoute     string     `gorm:"size:30" json:"image_edit_route"`
	VideoRoute         string     `gorm:"size:30" json:"video_route"`
	VideoDurations     string     `gorm:"size:200" json:"video_durations"`
	VideoCustomizable  bool       `gorm:"default:false" json:"video_customizable"`
	VideoCustomConfig  string     `gorm:"type:text" json:"video_custom_config"`
	SortOrder          int        `gorm:"default:0" json:"sort_order"`
}

func (ChannelModel) TableName() string { return "channel_models" }
