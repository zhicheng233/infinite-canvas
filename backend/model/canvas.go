package model

type CanvasProject struct {
	BaseModel
	TenantID       uint    `gorm:"index;uniqueIndex:idx_canvas_project_identity;not null" json:"tenant_id"`
	UserID         uint    `gorm:"index;uniqueIndex:idx_canvas_project_identity;not null" json:"user_id"`
	ProjectID      string  `gorm:"size:64;uniqueIndex:idx_canvas_project_identity;not null" json:"project_id"`
	SchemaVersion  int     `gorm:"default:2;not null" json:"schema_version"`
	Revision       uint    `gorm:"default:1;not null" json:"revision"`
	Title          string  `gorm:"size:200;not null" json:"title"`
	Nodes          string  `gorm:"type:longtext" json:"nodes"`
	Connections    string  `gorm:"type:longtext" json:"connections"`
	ChatSessions   string  `gorm:"type:longtext" json:"chat_sessions"`
	ActiveChatID   string  `gorm:"size:64" json:"active_chat_id"`
	BackgroundMode string  `gorm:"size:20;default:lines" json:"background_mode"`
	ShowImageInfo  *bool   `gorm:"default:false" json:"show_image_info"`
	ViewportX      float64 `gorm:"default:0" json:"viewport_x"`
	ViewportY      float64 `gorm:"default:0" json:"viewport_y"`
	ViewportK      float64 `gorm:"default:1" json:"viewport_k"`
}

func (CanvasProject) TableName() string { return "canvas_projects" }
