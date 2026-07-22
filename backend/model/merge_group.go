package model

import "time"

type ModelMergeGroup struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	ChannelID uint      `gorm:"index;not null" json:"channel_id"`
	GroupName string    `gorm:"size:200;not null" json:"group_name"`
	Pattern   string    `gorm:"size:200;not null" json:"pattern"`
	Enabled   bool      `gorm:"default:true" json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (ModelMergeGroup) TableName() string { return "model_merge_groups" }
