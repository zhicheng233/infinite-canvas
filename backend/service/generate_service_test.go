package service

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"infinite-canvas-server/model"
)

func TestIsUpstreamBalanceError(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{name: "chinese balance不足", body: "余额不足", want: true},
		{name: "english insufficient balance", body: "insufficient balance", want: true},
		{name: "quota exceeded", body: "quota exceeded", want: true},
		{name: "billing failed", body: "billing failed", want: true},
		{name: "insufficient_quota", body: "insufficient_quota", want: true},
		{name: "insufficient_user_quota", body: "insufficient_user_quota", want: true},
		{name: "chinese user quota insufficient", body: "用户额度不足, 剩余额度: ¥0.000000", want: true},
		{name: "扣费额度失败", body: "扣费额度失败", want: true},
		{name: "non-balance error", body: "Rate limit exceeded", want: false},
		{name: "content filter triggered", body: "Content filter triggered", want: false},
		{name: "empty string", body: "", want: false},
		{name: "case insensitive", body: "Insufficient Balance", want: true},
		{name: "mixed context with keyword", body: `{"error":{"message":"insufficient balance for this request"}}`, want: true},
		{name: "new api insufficient user quota response", body: `{"error":{"message":"用户额度不足, 剩余额度: ¥0.000000 (request id: 202607242118503872757708268d9d6DPiDlVHo)","type":"new_api_error","param":"","code":"insufficient_user_quota"}}`, want: true},
		{name: "normal error message", body: "invalid API key", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsUpstreamBalanceError(tt.body); got != tt.want {
				t.Fatalf("IsUpstreamBalanceError(%q) = %v, want %v", tt.body, got, tt.want)
			}
		})
	}
}

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

func TestExtractProxyModelNameUsesRoutingFallback(t *testing.T) {
	selection := ChannelSelection{ModelName: " omni-fast "}
	if got := extractProxyModelName("", nil, selection); got != "omni-fast" {
		t.Fatalf("model=%q, want omni-fast", got)
	}
	if got := extractProxyModelName("application/json", []byte(`{"model":"body-model"}`), selection); got != "body-model" {
		t.Fatalf("body model should take precedence, got %q", got)
	}
}

func TestGetProxyCostByGenerationSkipsGetPolling(t *testing.T) {
	pricing := &countingPricingReader{}
	svc := &GenerateService{creditRepo: pricing}
	cost, generation, _, err := svc.getProxyCostByGeneration(1, 1, http.MethodGet, "video", "", nil, "omni-fast")
	if err != nil || cost != 0 || generation != "video" || pricing.calls != 0 {
		t.Fatalf("cost=%d generation=%q pricingCalls=%d err=%v", cost, generation, pricing.calls, err)
	}
}

func TestFormatLoggedRequestBodySanitizesJSON(t *testing.T) {
	largeBase64 := strings.Repeat("a", 240)
	body := []byte(`{"model":"bh2.0","prompt":"hello","api_key":"secret","image":"data:image/png;base64,aaaa","nested":{"token":"abc","b64":"` + largeBase64 + `"},"ratio":"9:16"}`)
	text, truncated := formatLoggedRequestBody("application/json", body)

	if !strings.Contains(text, `"ratio": "9:16"`) || !strings.Contains(text, `"model": "bh2.0"`) {
		t.Fatalf("missing useful json fields: %s", text)
	}
	if strings.Contains(text, "secret") || strings.Contains(text, "data:image/png;base64,aaaa") || strings.Contains(text, largeBase64) {
		t.Fatalf("sensitive or large data leaked: %s", text)
	}
	if !strings.Contains(text, "[redacted]") || !strings.Contains(text, "[data url omitted") || !strings.Contains(text, "[base64 omitted") || !truncated {
		t.Fatalf("expected redaction and truncation marker: truncated=%v body=%s", truncated, text)
	}
}

func TestFormatLoggedRequestBodySummarizesMultipart(t *testing.T) {
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	_ = writer.WriteField("model", "video-model")
	file, err := writer.CreateFormFile("image", "ref.png")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = file.Write([]byte("fake-image-bytes"))
	_ = writer.Close()

	text, truncated := formatLoggedRequestBody(writer.FormDataContentType(), buffer.Bytes())
	if !strings.Contains(text, "model: video-model") || !strings.Contains(text, `image: [file filename="ref.png"`) {
		t.Fatalf("unexpected multipart summary: %s", text)
	}
	if strings.Contains(text, "fake-image-bytes") || !truncated {
		t.Fatalf("multipart file content should be omitted: truncated=%v body=%s", truncated, text)
	}
}

func TestBuildModelCallRequestSnapshotSanitizesURL(t *testing.T) {
	route := &channelRouteContext{Channel: &model.Channel{BaseUrl: "https://user:pass@example.com"}}
	snapshot := buildModelCallRequestSnapshot(route, http.MethodPost, "/video/generations?token=abc&keep=1", "application/json", []byte(`{"model":"m"}`))
	if snapshot == nil || !snapshot.Sent {
		t.Fatalf("missing request snapshot: %#v", snapshot)
	}
	if strings.Contains(snapshot.UpstreamURL, "user:pass") || strings.Contains(snapshot.UpstreamURL, "token=abc") {
		t.Fatalf("upstream url was not sanitized: %s", snapshot.UpstreamURL)
	}
	if !strings.Contains(snapshot.UpstreamURL, "token=%5Bredacted%5D") || !strings.Contains(snapshot.Body, `"model": "m"`) {
		t.Fatalf("unexpected request snapshot: %#v", snapshot)
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
			name:    "absolute url keeps selected base",
			baseURL: "https://hmgai.life",
			path:    "https://api.waninter.com/v1/videos/task_123/content",
			want:    "https://hmgai.life/v1/videos/task_123/content",
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

func TestRecalculateGenerationCost(t *testing.T) {
	t.Run("returns zero for empty generation", func(t *testing.T) {
		s := &GenerateService{}
		cost := s.recalculateGenerationCost(0, 0, "", "gpt-image-2")
		if cost != 0 {
			t.Fatalf("recalculateGenerationCost = %d, want 0", cost)
		}
	})

	t.Run("returns zero for empty modelName", func(t *testing.T) {
		s := &GenerateService{}
		cost := s.recalculateGenerationCost(0, 0, "image", "")
		if cost != 0 {
			t.Fatalf("recalculateGenerationCost = %d, want 0", cost)
		}
	})

	t.Run("returns zero for both empty", func(t *testing.T) {
		s := &GenerateService{}
		cost := s.recalculateGenerationCost(0, 0, "", "")
		if cost != 0 {
			t.Fatalf("recalculateGenerationCost = %d, want 0", cost)
		}
	})
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
