package service

import (
	"encoding/json"
	"strings"
	"testing"

	"infinite-canvas-server/model"
)

func TestImageGenerationsModelTestUsesTheProductionReferenceShape(t *testing.T) {
	cfg := configForChannelModel(&model.ChannelModel{ModelName: "gpt-image-2", ImageGenerateRoute: "generations", ImageEditRoute: "generations"})
	for _, test := range []struct {
		name           string
		referenceCount int
		wantArray      bool
	}{
		{name: "single reference is scalar", referenceCount: 1},
		{name: "multiple references are array", referenceCount: 2, wantArray: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			request, err := buildImageModelTestRequest(cfg, ModelTestInput{
				Model: "gpt-image-2", Operation: "image_edit", ReferenceCount: test.referenceCount,
			}, "edit the image")
			if err != nil {
				t.Fatalf("build request: %v", err)
			}
			if request.Method != "POST" || request.Path != "/images/generations" || request.ContentType != "application/json" {
				t.Fatalf("unexpected request plan: %+v", request)
			}
			var payload map[string]interface{}
			if err := json.Unmarshal(request.Body, &payload); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			_, isArray := payload["image"].([]interface{})
			if isArray != test.wantArray {
				t.Fatalf("image payload = %#v, want array=%v", payload["image"], test.wantArray)
			}
		})
	}
}

func TestImageEditsModelTestUsesMultipartProtocol(t *testing.T) {
	cfg := configForChannelModel(&model.ChannelModel{ModelName: "image-model", ImageGenerateRoute: "edits", ImageEditRoute: "edits"})
	request, err := buildImageModelTestRequest(cfg, ModelTestInput{Model: "image-model", Operation: "image_edit", ReferenceCount: 1}, "edit")
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if request.Path != "/images/edits" || !strings.HasPrefix(request.ContentType, "multipart/form-data; boundary=") {
		t.Fatalf("unexpected edits request plan: %+v", request)
	}
}
