package model

import "time"

type Channel struct {
	BaseModel
	Name               string     `gorm:"size:100;index" json:"name"`
	BaseUrl            string     `gorm:"size:500" json:"base_url"`
	ApiKey             string     `gorm:"size:500" json:"-"`
	Enabled            bool       `gorm:"index;default:true" json:"enabled"`
	NewApiChannelID    *int       `json:"new_api_channel_id,omitempty"`
	NewApiChannelIDVal string     `gorm:"size:100" json:"new_api_channel_id_val"`
	SyncStatus         string     `gorm:"size:20" json:"sync_status"`
	SyncError          string     `gorm:"size:500" json:"sync_error"`
	SyncedAt           *time.Time `json:"synced_at,omitempty"`
}

func (Channel) TableName() string { return "channels" }
