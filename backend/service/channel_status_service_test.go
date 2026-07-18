package service

import (
	"testing"
	"time"

	"infinite-canvas-server/model"
)

func TestChannelStatusSkipsEmptyModelLogs(t *testing.T) {
	now := time.Now()
	service := &ChannelStatusService{}
	items := service.aggregateByModel([]model.ModelCallLog{
		{BaseModel: model.BaseModel{CreatedAt: now}, Generation: "video", Model: "", IsSuccess: false, ErrorMessage: "task failed"},
		{BaseModel: model.BaseModel{CreatedAt: now}, Generation: "image", Model: "gpt-image-2", IsSuccess: true, ResponseTime: 1200},
	}, nil, 1)
	if len(items) != 1 {
		t.Fatalf("items length = %d, want 1", len(items))
	}
	if items[0].Model != "gpt-image-2" {
		t.Fatalf("unexpected model: %q", items[0].Model)
	}
}
