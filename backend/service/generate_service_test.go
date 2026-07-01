package service

import (
	"encoding/json"
	"strings"
	"testing"

	"infinite-canvas-server/model"
)

func TestGenerationTypeFromPath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{path: "/v1/images/generations", want: "image"},
		{path: "/v1/images/edits", want: "image"},
		{path: "/v1/video/generations", want: "video"},
		{path: "/v1/video/generations/task_123", want: "video"},
		{path: "/v1/videos/generations", want: "video"},
		{path: "/v1/videos", want: "video"},
		{path: "/v1/videos/task_123", want: "video"},
		{path: "/contents/generations/tasks", want: "video"},
		{path: "/v1/audio/speech", want: "audio"},
		{path: "/v1/chat/completions", want: "text"},
		{path: "/v1/responses", want: "text"},
	}

	for _, tt := range tests {
		if got := generationTypeFromPath(tt.path); got != tt.want {
			t.Fatalf("generationTypeFromPath(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestBuildCreditSpendDetail(t *testing.T) {
	metadata, note := buildCreditSpendDetail("image", "gpt-image-2", "/v1/images/generations?x=1", CreditCostResult{
		TotalCost: 6,
		UnitCost:  2,
		UnitType:  model.UnitPerImage,
		Units:     3,
	})
	if note != "图片生成 · 模型 gpt-image-2 · 扣除 6 积分 · 按图片 × 3" {
		t.Fatalf("unexpected note: %s", note)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(metadata), &parsed); err != nil {
		t.Fatalf("metadata is not json: %v", err)
	}
	if parsed["model"] != "gpt-image-2" || parsed["path"] != "/v1/images/generations" || parsed["unit_label"] != "按图片" {
		t.Fatalf("unexpected metadata: %#v", parsed)
	}
	if parsed["total_cost"].(float64) != 6 || parsed["unit_cost"].(float64) != 2 || parsed["units"].(float64) != 3 {
		t.Fatalf("unexpected cost metadata: %#v", parsed)
	}
}

func TestTransformImageResponseToChatFormat(t *testing.T) {
	raw := []byte(`{"created":1782898083,"data":[{"url":"https://example.com/a.jfif"},{"b64_json":"Zm9v"}]}`)
	converted, ok := transformImageResponseToChatFormat("/v1/chat/completions", raw)
	if !ok {
		t.Fatalf("expected response to be converted")
	}

	var payload struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(converted, &payload); err != nil {
		t.Fatalf("unexpected json error: %v", err)
	}
	if len(payload.Choices) != 1 {
		t.Fatalf("unexpected choices length: %d", len(payload.Choices))
	}
	content := payload.Choices[0].Message.Content
	if !strings.Contains(content, "![image](https://example.com/a.jfif)") {
		t.Fatalf("missing url image markdown: %s", content)
	}
	if !strings.Contains(content, "![image](data:image/png;base64,Zm9v)") {
		t.Fatalf("missing base64 image markdown: %s", content)
	}
}
