package model

type SiteAnnouncement struct {
	BaseModel
	Enabled bool   `gorm:"default:false;index" json:"enabled"`
	Title   string `gorm:"size:100" json:"title"`
	Content string `gorm:"type:text" json:"content"`
	Version int    `gorm:"default:1" json:"version"`
}

func (SiteAnnouncement) TableName() string { return "site_announcements" }

type SiteAnnouncementPayload struct {
	Enabled bool   `json:"enabled"`
	Title   string `json:"title"`
	Content string `json:"content"`
	Version int    `json:"version"`
}
