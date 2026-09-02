package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"infinite-canvas-server/service"
)

func TestChannelSelectionAndResolvedHeaders(t *testing.T) {
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest("GET", "/proxy?routing_pool_id=9&routing_model=omni-fast&routing_video_route=waninter&routing_capability=video", nil)

	selection := channelSelectionFromRequest(context)
	if selection.Kind != service.ModelSelectionAuto || selection.AutoRoutingPoolID != 9 || selection.ModelName != "omni-fast" || selection.VideoRoute != "waninter" || selection.Capability != "video" {
		t.Fatalf("unexpected routing selection: %#v", selection)
	}

	writeResolvedChannelHeaders(context, &service.ProxyResult{ResolvedChannelID: 2, ResolvedChannelModelID: 62})
	if recorder.Header().Get("X-Resolved-Channel-ID") != "2" || recorder.Header().Get("X-Resolved-Channel-Model-ID") != "62" {
		t.Fatalf("missing resolved channel headers: %#v", recorder.Header())
	}
}

func TestChannelSelectionCarriesMergeGroupFromOuterProxyQuery(t *testing.T) {
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest("GET", "/proxy?path=%2Fv1%2Fimages%2Fgenerations&channel_id=7&fuzzy_group_name=gpt-image", nil)

	selection := channelSelectionFromRequest(context)
	if selection.Kind != service.ModelSelectionMerge || selection.ChannelID != 7 || selection.MergeGroupName != "gpt-image" {
		t.Fatalf("unexpected merge selection: %#v", selection)
	}
}

func TestCopyProxyResponseHeadersSkipsBodyManagedHeaders(t *testing.T) {
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	headers := http.Header{
		"Content-Length":                []string{"99"},
		"Content-Encoding":              []string{"gzip"},
		"Transfer-Encoding":             []string{"chunked"},
		"Access-Control-Expose-Headers": []string{"X-Upstream-Only"},
		"X-Credits-Cost":                []string{"999"},
		"X-Resolved-Channel-Name":       []string{"spoofed"},
		"X-Upstream-Trace":              []string{"trace-1"},
	}

	copyProxyResponseHeaders(context, headers)

	if recorder.Header().Get("Content-Length") != "" {
		t.Fatalf("Content-Length should not be copied: %#v", recorder.Header())
	}
	if recorder.Header().Get("Content-Encoding") != "" {
		t.Fatalf("Content-Encoding should not be copied: %#v", recorder.Header())
	}
	if recorder.Header().Get("Transfer-Encoding") != "" {
		t.Fatalf("Transfer-Encoding should not be copied: %#v", recorder.Header())
	}
	if recorder.Header().Get("Access-Control-Expose-Headers") != "" {
		t.Fatalf("Access-Control-Expose-Headers should not be copied: %#v", recorder.Header())
	}
	if recorder.Header().Get("X-Credits-Cost") != "" || recorder.Header().Get("X-Resolved-Channel-Name") != "" {
		t.Fatalf("internal routing headers should not be copied: %#v", recorder.Header())
	}
	if recorder.Header().Get("X-Upstream-Trace") != "trace-1" {
		t.Fatalf("custom upstream header not copied: %#v", recorder.Header())
	}
}

func TestProxyResponseContentTypeDetectsVideo(t *testing.T) {
	mp4 := []byte{0, 0, 0, 24, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm'}
	if got := proxyResponseContentType("application/octet-stream", mp4); got != "video/mp4" {
		t.Fatalf("proxyResponseContentType mp4 = %q", got)
	}

	webm := []byte{0x1a, 0x45, 0xdf, 0xa3}
	if got := proxyResponseContentType("", webm); got != "video/webm" {
		t.Fatalf("proxyResponseContentType webm = %q", got)
	}

	jsonType := "application/json"
	if got := proxyResponseContentType(jsonType, []byte(`{"code":500}`)); got != jsonType {
		t.Fatalf("proxyResponseContentType json = %q", got)
	}
}
