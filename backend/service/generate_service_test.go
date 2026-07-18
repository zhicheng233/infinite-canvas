package service

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
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

func TestBuildUpstreamURL(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		path    string
		want    string
	}{
		{
			name:    "relative path adds v1",
			baseURL: "https://hmgai.life",
			path:    "/videos/task_123",
			want:    "https://hmgai.life/v1/videos/task_123",
		},
		{
			name:    "relative v1 path keeps v1",
			baseURL: "https://hmgai.life",
			path:    "/v1/videos/task_123",
			want:    "https://hmgai.life/v1/videos/task_123",
		},
		{
			name:    "absolute url keeps original",
			baseURL: "https://hmgai.life",
			path:    "https://api.waninter.com/v1/videos/task_123/content",
			want:    "https://api.waninter.com/v1/videos/task_123/content",
		},
	}

	for _, tt := range tests {
		if got := buildUpstreamURL(tt.baseURL, tt.path); got != tt.want {
			t.Fatalf("%s: buildUpstreamURL(%q, %q) = %q, want %q", tt.name, tt.baseURL, tt.path, got, tt.want)
		}
	}
}

func TestNormalizeVideoReferenceImagesCompressesLargeDataURL(t *testing.T) {
	var imageBuffer bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 800, 800))
	for y := 0; y < 800; y++ {
		for x := 0; x < 800; x++ {
			img.Set(x, y, color.RGBA{
				R: uint8((x * 17) ^ (y * 13)),
				G: uint8((x * y) % 251),
				B: uint8((x + y*3) % 253),
				A: 255,
			})
		}
	}
	if err := png.Encode(&imageBuffer, img); err != nil {
		t.Fatalf("png encode failed: %v", err)
	}
	encoded := base64.StdEncoding.EncodeToString(imageBuffer.Bytes())
	if len(encoded) <= maxVideoReferenceImageBase64Chars {
		t.Fatalf("test image is not large enough: %d", len(encoded))
	}

	body, err := json.Marshal(map[string]interface{}{
		"model":            "veo-omni-flash",
		"prompt":           "test",
		"reference_images": []string{"data:image/png;base64," + encoded},
	})
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	normalized, changed := normalizeVideoReferenceImages(http.MethodPost, "/v1/video/generations", "application/json", body)
	if !changed {
		t.Fatalf("expected large data URL to be compressed")
	}

	var parsed struct {
		ReferenceImages []string `json:"reference_images"`
	}
	if err := json.Unmarshal(normalized, &parsed); err != nil {
		t.Fatalf("normalized payload is invalid JSON: %v", err)
	}
	if len(parsed.ReferenceImages) != 1 {
		t.Fatalf("unexpected reference image count: %d", len(parsed.ReferenceImages))
	}
	dataURL := parsed.ReferenceImages[0]
	if !strings.HasPrefix(dataURL, "data:image/jpeg;base64,") {
		t.Fatalf("expected compressed JPEG data URL, got prefix: %s", dataURL[:30])
	}
	_, compressedEncoded, ok := splitBase64ImageDataURL(dataURL)
	if !ok {
		t.Fatalf("compressed image is not a base64 image data URL")
	}
	if len(compressedEncoded) > maxVideoReferenceImageBase64Chars {
		t.Fatalf("compressed image is still too large: %d", len(compressedEncoded))
	}
}

func TestNormalizeVideoReferenceImagesSkipsNonVideoRequests(t *testing.T) {
	body := []byte(`{"model":"gpt-image-2","image":"data:image/png;base64,AAAA"}`)
	normalized, changed := normalizeVideoReferenceImages(http.MethodPost, "/v1/images/generations", "application/json", body)
	if changed {
		t.Fatalf("image requests should not be normalized")
	}
	if !bytes.Equal(normalized, body) {
		t.Fatalf("body changed unexpectedly")
	}
}

func TestNormalizeVideoReferenceImagesAdjustsVeoOmniFlashDuration(t *testing.T) {
	body := []byte(`{"model":"veo-omni-flash","prompt":"test","duration":6,"seconds":"6"}`)
	normalized, changed := normalizeVideoReferenceImages(http.MethodPost, "/v1/video/generations", "application/json", body)
	if !changed {
		t.Fatalf("expected veo-omni-flash duration to be normalized")
	}

	var parsed struct {
		Duration float64 `json:"duration"`
		Seconds  string  `json:"seconds"`
	}
	if err := json.Unmarshal(normalized, &parsed); err != nil {
		t.Fatalf("normalized payload is invalid JSON: %v", err)
	}
	if parsed.Duration != 10 || parsed.Seconds != "10" {
		t.Fatalf("unexpected duration fields: duration=%v seconds=%q", parsed.Duration, parsed.Seconds)
	}
}

func TestBuildRepairRequestContextVideoImageToVideo(t *testing.T) {
	body := []byte(`{"model":"veo-omni-flash","prompt":"test","size":"720x1280","seconds":"6","reference_images":["https://example.com/a.png"]}`)
	ctx := buildRepairRequestContext("video", http.MethodPost, "/v1/videos", "application/json", body)
	if ctx == nil {
		t.Fatalf("expected repair context")
	}
	if ctx.Operation != "image_to_video" {
		t.Fatalf("operation=%q, want image_to_video", ctx.Operation)
	}
	if ctx.Size != "720x1280" || ctx.AspectRatio != "9:16" || ctx.Seconds != 6 {
		t.Fatalf("unexpected context: %#v", ctx)
	}
	if !ctx.HasReferences || ctx.ReferenceCount != 1 {
		t.Fatalf("unexpected reference context: %#v", ctx)
	}
}

func TestBuildRepairRequestContextImageEdit(t *testing.T) {
	body := []byte(`{"model":"gpt-image-2","prompt":"test","image":["https://example.com/a.png"],"size":"1536x1024"}`)
	ctx := buildRepairRequestContext("image", http.MethodPost, "/v1/images/edits", "application/json", body)
	if ctx == nil {
		t.Fatalf("expected repair context")
	}
	if ctx.Operation != "image_edit" {
		t.Fatalf("operation=%q, want image_edit", ctx.Operation)
	}
	if ctx.Size != "1536x1024" {
		t.Fatalf("size=%q, want 1536x1024", ctx.Size)
	}
	if !ctx.HasReferences || ctx.ReferenceCount != 1 {
		t.Fatalf("unexpected reference context: %#v", ctx)
	}
}
