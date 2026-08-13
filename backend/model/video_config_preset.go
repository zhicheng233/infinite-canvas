package model

import "time"

type VideoConfigPreset struct {
	ID             uint      `gorm:"primarykey" json:"id"`
	TenantID       uint      `gorm:"uniqueIndex:idx_video_config_preset_tenant_name;index;not null" json:"-"`
	Name           string    `gorm:"size:200;not null" json:"name"`
	NormalizedName string    `gorm:"size:200;uniqueIndex:idx_video_config_preset_tenant_name;not null" json:"-"`
	Config         string    `gorm:"type:text;not null" json:"-"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (VideoConfigPreset) TableName() string { return "video_config_presets" }

type VideoConfigPresetInfo struct {
	ID        uint              `json:"id"`
	Name      string            `json:"name"`
	Config    CustomVideoConfig `json:"config"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

type CreateVideoConfigPresetInput struct {
	Name   string             `json:"name"`
	Config *CustomVideoConfig `json:"config"`
}
