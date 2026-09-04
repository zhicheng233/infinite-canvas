package service

import (
	"strings"
	"testing"
	"time"

	"infinite-canvas-server/model"
)

func TestBuildModelCallErrorSummaryReadsNestedMessage(t *testing.T) {
	body := []byte(`{"error":{"message":"Invalid URL (POST /v1/videos/generations)","type":"invalid_request_error"}}`)
	got := buildModelCallErrorSummary(404, body, "")
	want := "Invalid URL (POST /v1/videos/generations)"
	if got != want {
		t.Fatalf("summary = %q, want %q", got, want)
	}
}

func TestBuildModelCallErrorSummaryTruncatesLongBody(t *testing.T) {
	body := []byte(`{"message":"` + strings.Repeat("x", 700) + `"}`)
	got := buildModelCallErrorSummary(500, body, "")
	if len(got) > 500 {
		t.Fatalf("summary length = %d, want <= 500", len(got))
	}
}

func TestReadFailedModelTaskResponse(t *testing.T) {
	body := []byte(`{"status":"failed","model":"sora_video2","error":{"message":"Video generation failed"}}`)
	failed, modelName, message := readFailedModelTaskResponse(body)
	if !failed || modelName != "sora_video2" || message != "Video generation failed" {
		t.Fatalf("failed=%v model=%q message=%q", failed, modelName, message)
	}
}

func TestReadFailedModelTaskResponse_SuccessStatus_ReturnsFalse(t *testing.T) {
	body := []byte(`{"status":"completed","output":"ok"}`)
	failed, modelName, message := readFailedModelTaskResponse(body)
	if failed {
		t.Fatalf("expected non-failure for completed status, got failed=true model=%q message=%q", modelName, message)
	}
}

func TestReadFailedModelTaskResponse_ProcessingStatus_ReturnsFalse(t *testing.T) {
	body := []byte(`{"status":"processing","task_id":"abc"}`)
	failed, _, _ := readFailedModelTaskResponse(body)
	if failed {
		t.Fatalf("expected non-failure for processing status (intermediate polling)")
	}
}

func TestReadFailedModelTaskResponse_QueuedStatus_ReturnsFalse(t *testing.T) {
	body := []byte(`{"status":"queued"}`)
	failed, _, _ := readFailedModelTaskResponse(body)
	if failed {
		t.Fatalf("expected non-failure for queued status")
	}
}

func TestReadFailedModelTaskResponse_ErrorStatus_ReturnsTrue(t *testing.T) {
	body := []byte(`{"status":"error","model":"sora","error":{"message":"processing error"}}`)
	failed, modelName, message := readFailedModelTaskResponse(body)
	if !failed || modelName != "sora" || message == "" {
		t.Fatalf("failed=%v model=%q message=%q", failed, modelName, message)
	}
}

func TestReadFailedModelTaskResponse_CancelledStatus_ReturnsTrue(t *testing.T) {
	body := []byte(`{"status":"cancelled","error":{"message":"user cancelled"}}`)
	failed, _, message := readFailedModelTaskResponse(body)
	if !failed || message == "" {
		t.Fatalf("failed=%v message=%q", failed, message)
	}
}

func TestReadFailedModelTaskResponse_ExpiredStatus_ReturnsTrue(t *testing.T) {
	body := []byte(`{"status":"expired","error":{"message":"task expired"}}`)
	failed, _, message := readFailedModelTaskResponse(body)
	if !failed || message != "task expired" {
		t.Fatalf("failed=%v message=%q", failed, message)
	}
}

func TestReadFailedModelTaskResponse_OKFalseErrorString_ReturnsTrue(t *testing.T) {
	body := []byte(`{"ok":false,"error":"视频生成失败：该模型为图生视频，必须提供 1 张参考图"}`)
	failed, _, message := readFailedModelTaskResponse(body)
	if !failed || message != "视频生成失败：该模型为图生视频，必须提供 1 张参考图" {
		t.Fatalf("failed=%v message=%q", failed, message)
	}
}

func TestReadFailedModelTaskResponse_SuccessFalse_ReturnsTrue(t *testing.T) {
	body := []byte(`{"success":false,"error":{"message":"processing error"}}`)
	failed, _, message := readFailedModelTaskResponse(body)
	if !failed || message == "" {
		t.Fatalf("failed=%v message=%q", failed, message)
	}
}

func TestReadFailedModelTaskResponse_InvalidJSON_ReturnsFalse(t *testing.T) {
	body := []byte(`not json`)
	failed, _, _ := readFailedModelTaskResponse(body)
	if failed {
		t.Fatalf("expected non-failure for invalid JSON")
	}
}

func TestReadFailedModelTaskResponseReadsFailedEnvelope(t *testing.T) {
	body := []byte(`{"code":"fail_to_fetch_task","message":"{\"error\":{\"message\":\"invalid request body\",\"type\":\"invalid_request_error\"}}","data":null}`)
	failed, _, message := readFailedModelTaskResponse(body)
	if !failed || message != "invalid request body" {
		t.Fatalf("failed=%v message=%q", failed, message)
	}
}

func TestReadFailedModelTaskResponseReadsUnavailableChannelEnvelope(t *testing.T) {
	body := []byte(`{"code":"fail_to_fetch_task","message":"{\"error\":{\"code\":\"model_not_found\",\"message\":\"No available channel for model omni-fast under group default (distributor)\",\"type\":\"new_api_error\"}}","data":null}`)
	failed, _, message := readFailedModelTaskResponse(body)
	if !failed || message != "No available channel for model omni-fast under group default (distributor)" {
		t.Fatalf("failed=%v message=%q", failed, message)
	}
}

func TestBuildModelHealthSummary(t *testing.T) {
	now := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
	logs := []model.ModelCallLog{
		{BaseModel: model.BaseModel{CreatedAt: now.Add(-2 * time.Hour)}, Model: "gpt-image-2", Generation: "image", ErrorMessage: "invalid request"},
		{BaseModel: model.BaseModel{CreatedAt: now.Add(-3 * time.Hour)}, Model: "gpt-image-2", Generation: "image", IsSuccess: true},
		{BaseModel: model.BaseModel{CreatedAt: now.Add(-4 * time.Hour)}, Model: "gpt-image-2", Generation: "image", ErrorMessage: "bad response"},
		{BaseModel: model.BaseModel{CreatedAt: now.Add(-30 * time.Hour)}, Model: "veo-omni-flash", Generation: "video", ErrorMessage: "timeout"},
		{BaseModel: model.BaseModel{CreatedAt: now.Add(-8 * 24 * time.Hour)}, Model: "old-model", Generation: "image", ErrorMessage: "too old"},
	}

	summary := buildModelHealthSummary(logs, now)

	if summary.Total24h != 2 || summary.Total7d != 3 {
		t.Fatalf("totals = %d/%d, want 2/3", summary.Total24h, summary.Total7d)
	}
	if len(summary.TopModels) != 2 || summary.TopModels[0].Model != "gpt-image-2" || summary.TopModels[0].Failures != 2 {
		t.Fatalf("unexpected top models: %#v", summary.TopModels)
	}
	if len(summary.RecentErrors) != 3 || summary.RecentErrors[0].ErrorMessage != "invalid request" {
		t.Fatalf("unexpected recent errors: %#v", summary.RecentErrors)
	}
}

func TestBuildModelHealthSummarySeparatesSameNameByChannel(t *testing.T) {
	now := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
	channelA, channelB := uint(1), uint(2)
	modelA, modelB := uint(11), uint(22)
	logs := []model.ModelCallLog{
		{BaseModel: model.BaseModel{CreatedAt: now.Add(-time.Hour)}, Model: "same-model", Generation: "image", ChannelID: &channelA, ChannelModelID: &modelA, ChannelName: "A", ChannelRemark: "主线路", ErrorMessage: "A failed"},
		{BaseModel: model.BaseModel{CreatedAt: now.Add(-2 * time.Hour)}, Model: "same-model", Generation: "image", ChannelID: &channelB, ChannelModelID: &modelB, ChannelName: "B", ErrorMessage: "B failed"},
	}

	summary := buildModelHealthSummary(logs, now)
	if len(summary.TopModels) != 2 {
		t.Fatalf("top models = %#v, want separate channel entries", summary.TopModels)
	}
	for _, item := range summary.TopModels {
		if item.Failures != 1 {
			t.Fatalf("same-name channel entry merged unexpectedly: %#v", summary.TopModels)
		}
		if item.ChannelID == nil || item.ChannelModelID == nil || item.ChannelName == "" {
			t.Fatalf("missing channel identity in health model: %#v", item)
		}
		if *item.ChannelID == channelA && item.ChannelRemark != "主线路" {
			t.Fatalf("missing channel remark in health model: %#v", item)
		}
	}
	if len(summary.RecentErrors) != 2 || summary.RecentErrors[0].ChannelName != "A" || summary.RecentErrors[0].ChannelRemark != "主线路" || summary.RecentErrors[0].ChannelID == nil || *summary.RecentErrors[0].ChannelID != channelA {
		t.Fatalf("missing channel identity in recent errors: %#v", summary.RecentErrors)
	}
}
