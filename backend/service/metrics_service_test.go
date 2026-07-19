package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"gorm.io/gorm"
	"infinite-canvas-server/model"
)

type fakeMetricsConfigRepo struct {
	cfg *model.MetricsConfig
	err error
}

func (r *fakeMetricsConfigRepo) Get() (*model.MetricsConfig, error) {
	if r.err != nil {
		return nil, r.err
	}
	if r.cfg == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return r.cfg, nil
}

func (r *fakeMetricsConfigRepo) Save(cfg *model.MetricsConfig) error {
	r.cfg = cfg
	return nil
}

type fakeMetricsChannelRepo struct {
	channels []model.Channel
}

func (r *fakeMetricsChannelRepo) ListEnabled() ([]model.Channel, error) {
	return r.channels, nil
}

type fakeMetricsModelRepo struct {
	models map[uint][]model.ChannelModel
}

func (r *fakeMetricsModelRepo) ListByChannel(channelID uint, enabledOnly bool) ([]model.ChannelModel, error) {
	return r.models[channelID], nil
}

func TestNormalizeAndParseMetricsHours(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want int
	}{
		{name: "missing", raw: "", want: 24},
		{name: "invalid", raw: "abc", want: 24},
		{name: "zero", raw: "0", want: 24},
		{name: "negative", raw: "-7", want: 24},
		{name: "preserved", raw: "48", want: 48},
		{name: "clamped", raw: "721", want: 720},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ParseMetricsHours(tc.raw); got != tc.want {
				t.Fatalf("ParseMetricsHours(%q) = %d, want %d", tc.raw, got, tc.want)
			}
		})
	}
}

func TestMetricsReadSendsExactHoursQueryAndNoAuth(t *testing.T) {
	newAPIID := 7
	var gotPath, gotQuery, gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotAuth = r.Header.Get("Authorization")
		json.NewEncoder(w).Encode(model.NewApiMetricsPayload{
			Success: true,
			Data: model.NewApiMetricsData{Channels: []model.NewApiChannelMetrics{{
				ChannelID:    newAPIID,
				RequestCount: 10,
				SuccessCount: 5,
				SuccessRate:  50,
				Models:       []model.NewApiModelMetrics{{ModelName: "gpt-5", RequestCount: 2, SuccessCount: 1, SuccessRate: 50}},
			}}},
		})
	}))
	defer server.Close()

	svc := newTestMetricsService(server.URL+"/api", []model.Channel{{BaseModel: model.BaseModel{ID: 1}, Name: "Primary", Enabled: true, NewApiChannelID: &newAPIID}}, map[uint][]model.ChannelModel{
		1: {{BaseModel: model.BaseModel{ID: 11}, ChannelID: 1, ModelName: "gpt-5", Enabled: true}},
	})
	resp, err := svc.Read(48)
	if err != nil {
		t.Fatalf("Read returned error: %v", err)
	}
	if gotPath != "/api/perf-metrics/channels" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotQuery != "hours=48" {
		t.Fatalf("query = %q, want hours=48", gotQuery)
	}
	if gotAuth != "" {
		t.Fatalf("Authorization header = %q, want empty", gotAuth)
	}
	if resp.Status != MetricsStatusOK || resp.Hours != 48 {
		t.Fatalf("unexpected response metadata: status=%s hours=%d", resp.Status, resp.Hours)
	}
}

func TestMetricsMappingExplicitChannelAndZeroVsUnavailable(t *testing.T) {
	newAPIID := 3
	unmappedNewAPIID := 4
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(model.NewApiMetricsPayload{
			Success: true,
			Data: model.NewApiMetricsData{Channels: []model.NewApiChannelMetrics{{
				ChannelID:    newAPIID,
				RequestCount: 8,
				SuccessCount: 0,
				SuccessRate:  0,
				Models:       []model.NewApiModelMetrics{{ModelName: "zero-model", RequestCount: 8, SuccessCount: 0, SuccessRate: 0}},
			}}},
		})
	}))
	defer server.Close()

	svc := newTestMetricsService(server.URL, []model.Channel{
		{BaseModel: model.BaseModel{ID: 10}, Name: "Mapped", Enabled: true, NewApiChannelID: &newAPIID},
		{BaseModel: model.BaseModel{ID: 20}, Name: "Unmapped", Enabled: true, NewApiChannelID: &unmappedNewAPIID},
		{BaseModel: model.BaseModel{ID: 30}, Name: "No Mapping", Enabled: true},
	}, map[uint][]model.ChannelModel{
		10: {{BaseModel: model.BaseModel{ID: 101}, ChannelID: 10, ModelName: "zero-model", Enabled: true}, {BaseModel: model.BaseModel{ID: 102}, ChannelID: 10, ModelName: "missing-model", Enabled: true}},
		20: {{BaseModel: model.BaseModel{ID: 201}, ChannelID: 20, ModelName: "zero-model", Enabled: true}},
		30: {{BaseModel: model.BaseModel{ID: 301}, ChannelID: 30, ModelName: "zero-model", Enabled: true}},
	})
	resp, err := svc.Read(24)
	if err != nil {
		t.Fatalf("Read returned error: %v", err)
	}
	if len(resp.Channels) != 3 {
		t.Fatalf("channels length = %d", len(resp.Channels))
	}
	mapped := resp.Channels[0]
	if mapped.ChannelID != 10 || mapped.Status != MetricsStatusOK {
		t.Fatalf("mapped channel identity/status = %d/%s", mapped.ChannelID, mapped.Status)
	}
	if mapped.SuccessRate == nil || *mapped.SuccessRate != 0 {
		t.Fatalf("mapped channel success_rate = %#v, want real zero", mapped.SuccessRate)
	}
	if mapped.Models[0].ChannelModelID != 102 || mapped.Models[0].SuccessRate != nil || mapped.Models[0].Status != MetricsStatusStale {
		t.Fatalf("missing model should be stale/unavailable, got %#v", mapped.Models[0])
	}
	if mapped.Models[1].ChannelModelID != 101 || mapped.Models[1].SuccessRate == nil || *mapped.Models[1].SuccessRate != 0 {
		t.Fatalf("zero model should preserve real zero, got %#v", mapped.Models[1])
	}
	if resp.Channels[1].Status != MetricsStatusStale || resp.Channels[1].SuccessRate != nil {
		t.Fatalf("unreturned mapped channel = %#v", resp.Channels[1])
	}
	if resp.Channels[2].Status != MetricsStatusUnmapped || resp.Channels[2].SuccessRate != nil {
		t.Fatalf("unmapped channel = %#v", resp.Channels[2])
	}
}

func TestMetricsReadMalformedAnd503AreAdvisory(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{
		{name: "malformed", status: http.StatusOK, body: `{"success":true,"data":{"channels":[{"channel_id":1,"success_rate":"bad"}]}}`},
		{name: "503", status: http.StatusServiceUnavailable, body: `unavailable`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			newAPIID := 1
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				w.Write([]byte(tc.body))
			}))
			defer server.Close()

			svc := newTestMetricsService(server.URL, []model.Channel{{BaseModel: model.BaseModel{ID: 1}, Name: "Primary", Enabled: true, NewApiChannelID: &newAPIID}}, map[uint][]model.ChannelModel{
				1: {{BaseModel: model.BaseModel{ID: 11}, ChannelID: 1, ModelName: "gpt-5", Enabled: true}},
			})
			resp, err := svc.Read(24)
			if err != nil {
				t.Fatalf("Read returned hard error: %v", err)
			}
			if resp.Status != MetricsStatusError || resp.Error == "" {
				t.Fatalf("expected advisory error response, got %#v", resp)
			}
			if len(resp.Channels) != 1 || len(resp.Channels[0].Models) != 1 {
				t.Fatalf("catalog identity was not preserved: %#v", resp.Channels)
			}
			if resp.Channels[0].SuccessRate != nil || resp.Channels[0].Models[0].SuccessRate != nil {
				t.Fatalf("unavailable metrics should have nil rates: %#v", resp.Channels[0])
			}
		})
	}
}

func newTestMetricsService(baseURL string, channels []model.Channel, models map[uint][]model.ChannelModel) *MetricsService {
	return NewMetricsService(
		&fakeMetricsConfigRepo{cfg: &model.MetricsConfig{MetricsBaseURL: baseURL}},
		&fakeMetricsChannelRepo{channels: channels},
		&fakeMetricsModelRepo{models: models},
	)
}
