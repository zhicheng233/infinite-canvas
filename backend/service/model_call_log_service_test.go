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

func TestReadFailedModelTaskResponseReadsFailedEnvelope(t *testing.T) {
	body := []byte(`{"code":"fail_to_fetch_task","message":"{\"error\":{\"message\":\"invalid request body\",\"type\":\"invalid_request_error\"}}","data":null}`)
	failed, _, message := readFailedModelTaskResponse(body)
	if !failed || message != "invalid request body" {
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
