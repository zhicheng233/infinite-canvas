package handler

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"infinite-canvas-server/service"
)

func TestChannelSelectionAndResolvedHeaders(t *testing.T) {
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest("GET", "/proxy?channel_id=0&channel_model_id=0&routing_model=omni-fast&routing_video_route=waninter", nil)

	selection := channelSelectionFromRequest(context)
	if selection.ModelName != "omni-fast" || selection.VideoRoute != "waninter" {
		t.Fatalf("unexpected routing selection: %#v", selection)
	}

	writeResolvedChannelHeaders(context, &service.ProxyResult{ResolvedChannelID: 2, ResolvedChannelModelID: 62})
	if recorder.Header().Get("X-Resolved-Channel-ID") != "2" || recorder.Header().Get("X-Resolved-Channel-Model-ID") != "62" {
		t.Fatalf("missing resolved channel headers: %#v", recorder.Header())
	}
}
