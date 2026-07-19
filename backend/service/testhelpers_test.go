package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"

	"infinite-canvas-server/model"
)

// FakeUpstreamRequest records what the fake server received.
type FakeUpstreamRequest struct {
	Method string
	Path   string
	Query  string
	Body   []byte
	Header http.Header
}

// FakeUpstreamServer is a minimal fake OpenAI-compatible upstream.
type FakeUpstreamServer struct {
	mu       sync.Mutex
	requests []FakeUpstreamRequest
	handler  http.HandlerFunc
	server   *httptest.Server
}

// NewFakeUpstreamServer creates a server using the given handler.
// Use NewFakeModelsServer or NewFakeMetricsServer for common cases.
func NewFakeUpstreamServer(t *testing.T, handler http.HandlerFunc) *FakeUpstreamServer {
	fake := &FakeUpstreamServer{
		handler:  handler,
		requests: []FakeUpstreamRequest{},
	}

	// Wrap handler to record requests
	wrappedHandler := func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, r.ContentLength)
		if r.ContentLength > 0 {
			r.Body.Read(body)
			r.Body.Close()
		}

		fake.mu.Lock()
		fake.requests = append(fake.requests, FakeUpstreamRequest{
			Method: r.Method,
			Path:   r.URL.Path,
			Query:  r.URL.RawQuery,
			Body:   body,
			Header: r.Header.Clone(),
		})
		fake.mu.Unlock()

		// Call original handler
		handler(w, r)
	}

	fake.server = httptest.NewServer(http.HandlerFunc(wrappedHandler))
	return fake
}

// URL returns the base URL of the fake server.
func (f *FakeUpstreamServer) URL() string {
	return f.server.URL
}

// Requests returns a copy of all recorded requests.
func (f *FakeUpstreamServer) Requests() []FakeUpstreamRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	requests := make([]FakeUpstreamRequest, len(f.requests))
	copy(requests, f.requests)
	return requests
}

// Close shuts down the server.
func (f *FakeUpstreamServer) Close() {
	f.server.Close()
}

// NewFakeModelsServer returns a server that responds with a /models list.
// The handler responds to GET /models with OpenAI-compatible model list.
func NewFakeModelsServer(t *testing.T, modelIDs []string) *FakeUpstreamServer {
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if r.URL.Path != "/models" && r.URL.Path != "/v1/models" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		// Build OpenAI-compatible model list response
		models := make([]map[string]interface{}, len(modelIDs))
		for i, id := range modelIDs {
			models[i] = map[string]interface{}{
				"id":      id,
				"object":  "model",
				"created": 1234567890,
				"owned_by": "openai",
			}
		}

		response := map[string]interface{}{
			"object": "list",
			"data":   models,
		}

		json.NewEncoder(w).Encode(response)
	}

	return NewFakeUpstreamServer(t, handler)
}

// FakeChannelMetrics describes metrics data for a single channel.
type FakeChannelMetrics struct {
	ChannelID   int
	Models      []FakeModelMetrics
	SuccessRate float64
}

// FakeModelMetrics describes metrics data for a single model.
type FakeModelMetrics struct {
	ModelName    string
	SuccessRate  float64
	RequestCount int
	SuccessCount int
}

// NewFakeMetricsServer returns a server responding to /api/perf-metrics/channels?hours=N.
// It returns a new-api compatible metrics response as defined in channel_dto.go.
func NewFakeMetricsServer(t *testing.T, channels []FakeChannelMetrics, expectedHours int) *FakeUpstreamServer {
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if r.URL.Path != "/api/perf-metrics/channels" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		// Verify query param
		hoursStr := r.URL.Query().Get("hours")
		if hoursStr != "" {
			hours, err := strconv.Atoi(hoursStr)
			if err == nil && hours != expectedHours {
				t.Logf("metrics server: got hours=%d, expected %d", hours, expectedHours)
			}
		}

		w.Header().Set("Content-Type", "application/json")

		// Build new-api compatible metrics response
		channelsData := make([]model.NewApiChannelMetrics, len(channels))
		for i, ch := range channels {
			models := make([]model.NewApiModelMetrics, len(ch.Models))
			for j, m := range ch.Models {
				models[j] = model.NewApiModelMetrics{
					ModelName:    m.ModelName,
					RequestCount: m.RequestCount,
					SuccessCount: m.SuccessCount,
					SuccessRate:  m.SuccessRate,
				}
			}

			channelsData[i] = model.NewApiChannelMetrics{
				ChannelID:    ch.ChannelID,
				RequestCount: calculateTotalRequests(ch.Models),
				SuccessCount: calculateTotalSuccesses(ch.Models),
				SuccessRate:  ch.SuccessRate,
				Models:       models,
			}
		}

		payload := model.NewApiMetricsPayload{
			Data: model.NewApiMetricsData{
				Channels: channelsData,
			},
			Success: true,
		}

		json.NewEncoder(w).Encode(payload)
	}

	return NewFakeUpstreamServer(t, handler)
}

// calculateTotalRequests sums request counts across all models.
func calculateTotalRequests(models []FakeModelMetrics) int {
	total := 0
	for _, m := range models {
		total += m.RequestCount
	}
	return total
}

// calculateTotalSuccesses sums success counts across all models.
func calculateTotalSuccesses(models []FakeModelMetrics) int {
	total := 0
	for _, m := range models {
		total += m.SuccessCount
	}
	return total
}

// TestFakeServers verifies the helpers work end-to-end.
func TestFakeServers(t *testing.T) {
	// Test 1: FakeModelsServer
	t.Run("models server", func(t *testing.T) {
		modelIDs := []string{"gpt-4", "gpt-3.5-turbo", "davinci-003"}
		modelsServer := NewFakeModelsServer(t, modelIDs)
		defer modelsServer.Close()

		// Make HTTP GET request
		resp, err := http.Get(modelsServer.URL() + "/models")
		if err != nil {
			t.Fatalf("GET /models failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}

		// Parse response
		var payload map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&payload)
		if err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		// Verify structure
		data, ok := payload["data"].([]interface{})
		if !ok {
			t.Fatalf("expected data array in response, got %T", payload["data"])
		}

		if len(data) != len(modelIDs) {
			t.Fatalf("expected %d models, got %d", len(modelIDs), len(data))
		}

		// Verify first model
		firstModel, ok := data[0].(map[string]interface{})
		if !ok {
			t.Fatalf("expected model object, got %T", data[0])
		}
		if firstModel["id"] != modelIDs[0] {
			t.Fatalf("expected model id %q, got %q", modelIDs[0], firstModel["id"])
		}

		// Verify requests were recorded
		requests := modelsServer.Requests()
		if len(requests) != 1 {
			t.Fatalf("expected 1 request recorded, got %d", len(requests))
		}
		if requests[0].Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", requests[0].Method)
		}
	})

	// Test 2: FakeMetricsServer
	t.Run("metrics server", func(t *testing.T) {
		channels := []FakeChannelMetrics{
			{
				ChannelID:   1,
				SuccessRate: 0.95,
				Models: []FakeModelMetrics{
					{
						ModelName:    "gemini-2.5-pro",
						SuccessRate:  0.95,
						RequestCount: 100,
						SuccessCount: 95,
					},
					{
						ModelName:    "gpt-4",
						SuccessRate:  0.92,
						RequestCount: 50,
						SuccessCount: 46,
					},
				},
			},
			{
				ChannelID:   2,
				SuccessRate: 0.88,
				Models: []FakeModelMetrics{
					{
						ModelName:    "claude-opus",
						SuccessRate:  0.88,
						RequestCount: 25,
						SuccessCount: 22,
					},
				},
			},
		}

		metricsServer := NewFakeMetricsServer(t, channels, 24)
		defer metricsServer.Close()

		// Make HTTP GET request with hours parameter
		resp, err := http.Get(metricsServer.URL() + "/api/perf-metrics/channels?hours=24")
		if err != nil {
			t.Fatalf("GET /api/perf-metrics/channels failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}

		// Parse response
		var payload model.NewApiMetricsPayload
		err = json.NewDecoder(resp.Body).Decode(&payload)
		if err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if !payload.Success {
			t.Fatalf("expected success=true")
		}

		if len(payload.Data.Channels) != len(channels) {
			t.Fatalf("expected %d channels, got %d", len(channels), len(payload.Data.Channels))
		}

		// Verify first channel
		firstChannel := payload.Data.Channels[0]
		if firstChannel.ChannelID != channels[0].ChannelID {
			t.Fatalf("expected channel_id %d, got %d", channels[0].ChannelID, firstChannel.ChannelID)
		}
		if firstChannel.SuccessRate != channels[0].SuccessRate {
			t.Fatalf("expected success_rate %f, got %f", channels[0].SuccessRate, firstChannel.SuccessRate)
		}

		// Verify models in first channel
		if len(firstChannel.Models) != len(channels[0].Models) {
			t.Fatalf("expected %d models in channel 0, got %d", len(channels[0].Models), len(firstChannel.Models))
		}

		firstModel := firstChannel.Models[0]
		if firstModel.ModelName != channels[0].Models[0].ModelName {
			t.Fatalf("expected model name %q, got %q", channels[0].Models[0].ModelName, firstModel.ModelName)
		}
		if firstModel.SuccessRate != channels[0].Models[0].SuccessRate {
			t.Fatalf("expected model success_rate %f, got %f", channels[0].Models[0].SuccessRate, firstModel.SuccessRate)
		}

		// Verify requests were recorded
		requests := metricsServer.Requests()
		if len(requests) < 1 {
			t.Fatalf("expected at least 1 request recorded, got %d", len(requests))
		}
		if requests[0].Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", requests[0].Method)
		}
		if !contains(requests[0].Query, "hours=24") {
			t.Logf("metrics request query: %s", requests[0].Query)
		}
	})
}

// contains checks if a string contains a substring.
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
