package repository

import (
	"testing"
	"time"

	"gorm.io/gorm"
	"infinite-canvas-server/model"
)

func TestMergeChannelModelForUpsertRestoresDeletedModel(t *testing.T) {
	existing := &model.ChannelModel{
		BaseModel: model.BaseModel{
			ID:        9,
			DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true},
		},
		ChannelID:          3,
		ModelName:          "omni_flash_nowater",
		Enabled:            false,
		ImageGenerateRoute: "generations",
	}
	incoming := &model.ChannelModel{
		ChannelID:          3,
		ModelName:          "omni_flash_nowater",
		Enabled:            true,
		Capabilities:       `["image","video","text","audio"]`,
		ImageGenerateRoute: "auto",
	}

	mergeChannelModelForUpsert(existing, incoming)

	if existing.DeletedAt.Valid {
		t.Fatal("deleted model should be restored")
	}
	if existing.Enabled {
		t.Fatal("existing enabled setting should be preserved")
	}
	if existing.ImageGenerateRoute != "generations" {
		t.Fatalf("route=%q, want generations", existing.ImageGenerateRoute)
	}
	if existing.Capabilities != incoming.Capabilities {
		t.Fatalf("capabilities=%q, want %q", existing.Capabilities, incoming.Capabilities)
	}
}
