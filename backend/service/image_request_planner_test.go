package service

import (
	"testing"

	"infinite-canvas-server/model"
)

func TestImageRequestPlannerEnforcesConfiguredProtocol(t *testing.T) {
	generations := &channelRouteContext{ChannelModel: &model.ChannelModel{ModelName: "gpt-image-2", ImageGenerateRoute: "generations", ImageEditRoute: "generations"}}
	for name, body := range map[string]string{
		"single reference":    `{"model":"gpt-image-2","image":"data:image/png;base64,AA=="}`,
		"multiple references": `{"model":"gpt-image-2","image":["data:image/png;base64,AA==","data:image/png;base64,BB=="]}`,
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateResolvedRequestProtocol(generations, "image", "POST", "/v1/images/generations", "application/json", []byte(body)); err != nil {
				t.Fatalf("valid generations request rejected: %v", err)
			}
		})
	}
	if err := validateResolvedRequestProtocol(generations, "image", "POST", "/v1/images/generations", "application/json", []byte(`{"image":{"url":"x"}}`)); err == nil {
		t.Fatal("object image value was accepted")
	}
	if err := validateResolvedRequestProtocol(generations, "image", "POST", "/v1/images/edits", "multipart/form-data; boundary=test", nil); err == nil {
		t.Fatal("generations route accepted multipart edits")
	}

	edits := &channelRouteContext{ChannelModel: &model.ChannelModel{ModelName: "edit-model", ImageGenerateRoute: "generations", ImageEditRoute: "edits"}}
	if err := validateResolvedRequestProtocol(edits, "image", "POST", "/v1/images/edits", "multipart/form-data; boundary=test", []byte("multipart")); err != nil {
		t.Fatalf("valid edits request rejected: %v", err)
	}
	if err := validateResolvedRequestProtocol(edits, "image", "POST", "/v1/images/generations", "application/json", []byte(`{"image":"x"}`)); err == nil {
		t.Fatal("edits route accepted JSON generations editing")
	}
}

func TestImageRequestPlannerAllowsConfiguredChatImageRoute(t *testing.T) {
	route := &channelRouteContext{ChannelModel: &model.ChannelModel{ModelName: "image-chat", ImageGenerateRoute: "chat", ImageEditRoute: "chat"}}
	if err := validateResolvedRequestProtocol(route, "image", "POST", "/v1/chat/completions", "application/json", []byte(`{"messages":[]}`)); err != nil {
		t.Fatalf("chat image request rejected: %v", err)
	}
}
