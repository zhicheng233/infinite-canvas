package model

type FeatureGuideSurface string

const (
	FeatureGuideSurfaceCanvas FeatureGuideSurface = "canvas"
	FeatureGuideSurfaceImage  FeatureGuideSurface = "image"
	FeatureGuideSurfaceVideo  FeatureGuideSurface = "video"
)

func (surface FeatureGuideSurface) Valid() bool {
	return surface == FeatureGuideSurfaceCanvas || surface == FeatureGuideSurfaceImage || surface == FeatureGuideSurfaceVideo
}

type FeatureGuide struct {
	BaseModel
	Surface FeatureGuideSurface `gorm:"size:20;uniqueIndex;not null" json:"surface"`
	Enabled bool                `gorm:"default:false;not null" json:"enabled"`
	Title   string              `gorm:"size:100;not null" json:"title"`
	Pages   string              `gorm:"type:longtext;not null" json:"-"`
	Version int                 `gorm:"default:1;not null" json:"version"`
}

func (FeatureGuide) TableName() string { return "feature_guides" }

type FeatureGuidePayload struct {
	Surface FeatureGuideSurface `json:"surface"`
	Enabled bool                `json:"enabled"`
	Title   string              `json:"title"`
	Pages   []string            `json:"pages"`
	Version int                 `json:"version"`
}
