package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

type OnDemandRepairService struct {
	endpoint   string
	username   string
	password   string
	httpClient *http.Client
}

type OnDemandRepairResult struct {
	OK          bool   `json:"ok"`
	Repaired    bool   `json:"repaired"`
	Switched    bool   `json:"switched"`
	Locked      bool   `json:"locked"`
	ChannelID   int    `json:"channel_id"`
	ChannelName string `json:"channel_name"`
	Message     string `json:"message"`
}

type RepairRequestContext struct {
	Method         string `json:"method,omitempty"`
	Path           string `json:"path,omitempty"`
	ContentType    string `json:"content_type,omitempty"`
	Operation      string `json:"operation,omitempty"`
	Size           string `json:"size,omitempty"`
	AspectRatio    string `json:"aspect_ratio,omitempty"`
	Seconds        int    `json:"seconds,omitempty"`
	HasReferences  bool   `json:"has_references,omitempty"`
	ReferenceCount int    `json:"reference_count,omitempty"`
}

func NewOnDemandRepairService(endpoint, username, password string, timeoutSeconds int) *OnDemandRepairService {
	endpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")
	if endpoint == "" {
		return nil
	}
	if timeoutSeconds <= 0 {
		timeoutSeconds = 420
	}
	return &OnDemandRepairService{
		endpoint: endpoint,
		username: strings.TrimSpace(username),
		password: password,
		httpClient: &http.Client{
			Timeout: time.Duration(timeoutSeconds) * time.Second,
		},
	}
}

func (s *OnDemandRepairService) Enabled() bool {
	return s != nil && s.endpoint != ""
}

func (s *OnDemandRepairService) Repair(ctx context.Context, generation, modelName, reason string, requestContext *RepairRequestContext) (*OnDemandRepairResult, error) {
	if !s.Enabled() {
		return nil, nil
	}
	payloadMap := map[string]interface{}{
		"generation": strings.TrimSpace(generation),
		"model":      strings.TrimSpace(modelName),
		"reason":     truncateString(reason, 500),
	}
	if requestContext != nil {
		payloadMap["request_context"] = requestContext
	}
	payload, err := json.Marshal(payloadMap)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint+"/api/repair-one", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if s.username != "" || s.password != "" {
		req.SetBasicAuth(s.username, s.password)
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result OnDemandRepairResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 && !result.Locked {
		return &result, fmt.Errorf("repair endpoint returned HTTP %d", resp.StatusCode)
	}
	log.Printf("on-demand repair result generation=%s model=%s ok=%v repaired=%v switched=%v channel_id=%d channel=%s", generation, modelName, result.OK, result.Repaired, result.Switched, result.ChannelID, result.ChannelName)
	return &result, nil
}
