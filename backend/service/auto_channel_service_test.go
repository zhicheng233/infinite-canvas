package service

import (
	"context"
	"math"
	"net/http"
	"testing"
	"time"

	"infinite-canvas-server/model"
)

func TestAutoRoutingContractSeparatesImageProtocols(t *testing.T) {
	channel := &model.Channel{Enabled: true}
	base := &model.ChannelModel{ModelName: "image-model", Capabilities: `["image"]`, Enabled: true, ImageGenerateRoute: "generations", ImageEditRoute: "edits"}
	first, err := autoRoutingContract(channel, base, "image")
	if err != nil {
		t.Fatalf("build image contract: %v", err)
	}
	same := *base
	second, err := autoRoutingContract(channel, &same, "image")
	if err != nil || second != first {
		t.Fatalf("equivalent contracts differ: %q / %q, %v", first, second, err)
	}
	different := *base
	different.ImageEditRoute = "generations"
	third, err := autoRoutingContract(channel, &different, "image")
	if err != nil || third == first {
		t.Fatalf("different edit protocols shared a contract: %q / %q, %v", first, third, err)
	}
}

func TestSummarizeAutoHealthUsesSmoothingP95AndCircuitState(t *testing.T) {
	now := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	attempts := []model.GenerationAttempt{
		{BaseModel: model.BaseModel{CreatedAt: now.Add(-30 * time.Second)}, Success: false, Retryable: true, ResponseTimeMs: 300},
		{BaseModel: model.BaseModel{CreatedAt: now.Add(-time.Minute)}, Success: false, Retryable: true, ResponseTimeMs: 100},
		{BaseModel: model.BaseModel{CreatedAt: now.Add(-2 * time.Minute)}, Success: false, Retryable: true, ResponseTimeMs: 200},
	}
	rate, samples, p95, circuit := summarizeAutoHealth(attempts, now)
	if math.Abs(rate-69.230769) > 0.001 || samples != 3 || p95 != 300 || circuit != "open" {
		t.Fatalf("unexpected health summary: rate=%f samples=%d p95=%d circuit=%s", rate, samples, p95, circuit)
	}
	_, _, _, circuit = summarizeAutoHealth(attempts, now.Add(3*time.Minute))
	if circuit != "half_open" {
		t.Fatalf("cooled circuit = %s, want half_open", circuit)
	}
	attempts[0].Success = true
	_, _, _, circuit = summarizeAutoHealth(attempts, now)
	if circuit != "closed" {
		t.Fatalf("successful latest probe must close circuit, got %s", circuit)
	}
}

func TestClassifyAutoFailureRetriesOnlyChannelFailures(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       []byte
		err        error
		category   string
		retryable  bool
	}{
		{name: "timeout", err: context.DeadlineExceeded, category: "timeout", retryable: true},
		{name: "http timeout", statusCode: http.StatusRequestTimeout, category: "timeout", retryable: true},
		{name: "authentication", statusCode: http.StatusUnauthorized, category: "auth", retryable: true},
		{name: "rate limit", statusCode: http.StatusTooManyRequests, category: "rate_limited", retryable: true},
		{name: "upstream", statusCode: http.StatusBadGateway, category: "upstream_unavailable", retryable: true},
		{name: "content", statusCode: http.StatusBadRequest, body: []byte(`{"error":{"message":"content policy rejected"}}`), category: "content_rejected", retryable: false},
		{name: "parameter", statusCode: http.StatusBadRequest, body: []byte(`{"error":{"message":"invalid size"}}`), category: "request_invalid", retryable: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			category, retryable := classifyAutoFailure(test.statusCode, test.body, test.err)
			if category != test.category || retryable != test.retryable {
				t.Fatalf("classify = %s/%t, want %s/%t", category, retryable, test.category, test.retryable)
			}
		})
	}
}

func TestEnabledMemberCountAfterReplacementPreservesExistingSettings(t *testing.T) {
	current := []model.AutoRoutingPoolMember{
		{ChannelModelID: 11, Enabled: false},
		{ChannelModelID: 22, Enabled: true},
	}
	if count := enabledMemberCountAfterReplacement(current, []uint{11, 22, 33}); count != 2 {
		t.Fatalf("enabled replacement members = %d, want 2", count)
	}
}
