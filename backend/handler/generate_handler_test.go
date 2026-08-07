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

func TestCopyProxyResponseHeadersSkipsBodyManagedHeaders(t *testing.T) {
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	headers := http.Header{
		"Content-Length":                []string{"99"},
		"Content-Encoding":              []string{"gzip"},
		"Transfer-Encoding":             []string{"chunked"},
		"Access-Control-Expose-Headers": []string{"X-Upstream-Only"},
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
