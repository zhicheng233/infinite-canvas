package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"infinite-canvas-server/model"
)

const (
	MetricsStatusOK          = "ok"
	MetricsStatusUnavailable = "unavailable"
	MetricsStatusUnmapped    = "unmapped"
	MetricsStatusStale       = "stale"
	MetricsStatusError       = "error"
)

type MetricsService struct {
	configRepo  metricsConfigRepo
	channelRepo metricsChannelRepo
	modelRepo   metricsModelRepo
	httpClient  *http.Client
}

type metricsConfigRepo interface {
	Get() (*model.MetricsConfig, error)
	Save(cfg *model.MetricsConfig) error
}

type metricsChannelRepo interface {
	ListEnabled() ([]model.Channel, error)
}

type metricsModelRepo interface {
	ListByChannel(channelID uint, enabledOnly bool) ([]model.ChannelModel, error)
}

func NewMetricsService(configRepo metricsConfigRepo, channelRepo metricsChannelRepo, modelRepo metricsModelRepo) *MetricsService {
	return &MetricsService{
		configRepo:  configRepo,
		channelRepo: channelRepo,
		modelRepo:   modelRepo,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (s *MetricsService) GetConfig() (*model.MetricsURLConfig, error) {
	cfg, err := s.configRepo.Get()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &model.MetricsURLConfig{MetricsBaseURL: ""}, nil
		}
		return nil, err
	}
	return &model.MetricsURLConfig{MetricsBaseURL: cfg.MetricsBaseURL}, nil
}

func (s *MetricsService) SaveConfig(input model.MetricsURLConfig) (*model.MetricsURLConfig, error) {
	baseURL, err := normalizeMetricsBaseURL(input.MetricsBaseURL)
	if err != nil {
		return nil, err
	}
	if err := s.configRepo.Save(&model.MetricsConfig{MetricsBaseURL: baseURL}); err != nil {
		return nil, err
	}
	return &model.MetricsURLConfig{MetricsBaseURL: baseURL}, nil
}

func (s *MetricsService) Read(hoursInput int) (*model.MetricsResponse, error) {
	hours := NormalizeMetricsHours(hoursInput)
	channels, err := s.channelRepo.ListEnabled()
	if err != nil {
		return nil, err
	}
	response := model.MetricsResponse{
		Channels:  s.unavailableChannels(channels, MetricsStatusUnavailable),
		Hours:     hours,
		Window:    fmt.Sprintf("%dh", hours),
		Status:    MetricsStatusUnavailable,
		UpdatedAt: time.Now(),
	}

	// Separate channels by type
	var idZeroChannels, idOtherChannels []model.Channel
	for _, ch := range channels {
		if ch.NewApiChannelID != nil && *ch.NewApiChannelID == 0 {
			idZeroChannels = append(idZeroChannels, ch)
		} else {
			idOtherChannels = append(idOtherChannels, ch)
		}
	}

	var channelRates []model.MetricsChannelRate

	// Process ID≠0 channels (use channels endpoint)
	if len(idOtherChannels) > 0 {
		// Group by metrics base URL for efficient requests
		urlGroups := make(map[string][]model.Channel)
		for _, ch := range idOtherChannels {
			baseURL := resolveMetricsBaseURL(ch)
			urlGroups[baseURL] = append(urlGroups[baseURL], ch)
		}
		for baseURL, group := range urlGroups {
			requestURL, err := buildMetricsRequestURL(baseURL, hours, "/api/perf-metrics/channels")
			if err != nil {
				for _, ch := range group {
					channelRates = append(channelRates, model.MetricsChannelRate{
						ChannelID: ch.ID, ChannelName: ch.Name,
						NewApiChannelID: ch.NewApiChannelID, Status: MetricsStatusError,
						Models: s.mapModelMetrics(ch.ID, nil, MetricsStatusError),
					})
				}
				continue
			}
			payload, err := s.fetchMetrics(requestURL)
			if err != nil {
				for _, ch := range group {
					channelRates = append(channelRates, model.MetricsChannelRate{
						ChannelID: ch.ID, ChannelName: ch.Name,
						NewApiChannelID: ch.NewApiChannelID, Status: MetricsStatusError,
						Models: s.mapModelMetrics(ch.ID, nil, MetricsStatusError),
					})
				}
				continue
			}
			channelRates = append(channelRates, s.mapMetrics(group, payload)...)
		}
	}

	// Process ID=0 channels (use summary endpoint)
	for _, ch := range idZeroChannels {
		baseURL := resolveMetricsBaseURL(ch)
		payload, err := s.fetchSummaryMetrics(baseURL, hours)
		if err != nil {
			channelRates = append(channelRates, model.MetricsChannelRate{
				ChannelID: ch.ID, ChannelName: ch.Name,
				NewApiChannelID: ch.NewApiChannelID, Status: MetricsStatusError,
				Models: s.mapModelMetrics(ch.ID, nil, MetricsStatusError),
			})
			continue
		}
		channelRates = append(channelRates, s.mapSummaryMetrics(ch, payload))
	}

	if len(channelRates) > 0 {
		response.Channels = channelRates
		response.Status = MetricsStatusOK
	}
	return &response, nil
}

func (s *MetricsService) fetchMetrics(requestURL string) (*model.NewApiMetricsPayload, error) {
	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("metrics upstream returned HTTP %d", resp.StatusCode)
	}
	var payload model.NewApiMetricsPayload
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&payload); err != nil {
		return nil, err
	}
	if !payload.Success {
		return nil, errors.New("metrics upstream returned unsuccessful payload")
	}
	return &payload, nil
}

func (s *MetricsService) fetchSummaryMetrics(baseURL string, hours int) (*model.NewApiSummaryPayload, error) {
	requestURL, err := buildMetricsRequestURL(baseURL, hours, "/api/perf-metrics/summary")
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("metrics upstream returned HTTP %d", resp.StatusCode)
	}
	var payload model.NewApiSummaryPayload
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&payload); err != nil {
		return nil, err
	}
	if !payload.Success {
		return nil, errors.New("summary metrics upstream returned unsuccessful payload")
	}
	return &payload, nil
}

func (s *MetricsService) mapMetrics(channels []model.Channel, payload *model.NewApiMetricsPayload) []model.MetricsChannelRate {
	byNewAPIID := make(map[int]model.NewApiChannelMetrics, len(payload.Data.Channels))
	for _, item := range payload.Data.Channels {
		byNewAPIID[item.ChannelID] = item
	}

	items := make([]model.MetricsChannelRate, 0, len(channels))
	for i := range channels {
		channel := channels[i]
		status := MetricsStatusUnmapped
		var source *model.NewApiChannelMetrics
		if channel.NewApiChannelID != nil {
			if value, ok := byNewAPIID[*channel.NewApiChannelID]; ok {
				source = &value
				status = MetricsStatusOK
			} else {
				status = MetricsStatusStale
			}
		}

		rate := model.MetricsChannelRate{
			ChannelID:       channel.ID,
			ChannelName:     channel.Name,
			NewApiChannelID: channel.NewApiChannelID,
			Status:          status,
			Models:          s.mapModelMetrics(channel.ID, source, status),
		}
		if source != nil {
			rate.RequestCount = source.RequestCount
			rate.SuccessCount = source.SuccessCount
			rate.SuccessRate = float64Ptr(source.SuccessRate)
		}
		items = append(items, rate)
	}
	return items
}

func (s *MetricsService) mapModelMetrics(channelID uint, source *model.NewApiChannelMetrics, fallbackStatus string) []model.MetricsModelRate {
	models, err := s.modelRepo.ListByChannel(channelID, true)
	if err != nil {
		return []model.MetricsModelRate{}
	}
	byName := map[string]model.NewApiModelMetrics{}
	if source != nil {
		for _, item := range source.Models {
			byName[item.ModelName] = item
		}
	}
	items := make([]model.MetricsModelRate, 0, len(models))
	for _, channelModel := range models {
		item := model.MetricsModelRate{ChannelModelID: channelModel.ID, ChannelID: channelID, ModelName: channelModel.ModelName, Status: fallbackStatus}
		if source != nil {
			if metrics, ok := byName[channelModel.ModelName]; ok {
				item.RequestCount = metrics.RequestCount
				item.SuccessCount = metrics.SuccessCount
				item.SuccessRate = float64Ptr(metrics.SuccessRate)
				item.Status = MetricsStatusOK
			} else {
				item.Status = MetricsStatusStale
			}
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].ModelName < items[j].ModelName
	})
	return items
}

func (s *MetricsService) mapSummaryMetrics(channel model.Channel, source *model.NewApiSummaryPayload) model.MetricsChannelRate {
	status := MetricsStatusOK
	rate := model.MetricsChannelRate{
		ChannelID:       channel.ID,
		ChannelName:     channel.Name,
		NewApiChannelID: channel.NewApiChannelID,
		Status:          status,
	}

	models, err := s.modelRepo.ListByChannel(channel.ID, true)
	if err != nil {
		return rate
	}

	byName := map[string]model.NewApiSummaryModelMetrics{}
	for _, m := range source.Data.Models {
		byName[m.ModelName] = m
	}

	var sumSuccessRate float64
	count := 0
	items := make([]model.MetricsModelRate, 0, len(models))
	for _, channelModel := range models {
		item := model.MetricsModelRate{
			ChannelModelID: channelModel.ID, ChannelID: channel.ID,
			ModelName: channelModel.ModelName, Status: MetricsStatusStale,
		}
		if sm, ok := byName[channelModel.ModelName]; ok {
			item.Status = MetricsStatusOK
			item.SuccessRate = float64Ptr(sm.SuccessRate)
			item.AvgLatencyMs = &sm.AvgLatencyMs
			item.AvgTps = &sm.AvgTps
			item.RecentSuccessRates = sm.RecentSuccessRates
			sumSuccessRate += sm.SuccessRate
			count++
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ModelName < items[j].ModelName })
	rate.Models = items
	if count > 0 {
		rate.SuccessRate = float64Ptr(sumSuccessRate / float64(count))
	}
	return rate
}

func (s *MetricsService) unavailableChannels(channels []model.Channel, status string) []model.MetricsChannelRate {
	items := make([]model.MetricsChannelRate, 0, len(channels))
	for i := range channels {
		channel := channels[i]
		items = append(items, model.MetricsChannelRate{
			ChannelID:       channel.ID,
			ChannelName:     channel.Name,
			NewApiChannelID: channel.NewApiChannelID,
			Status:          status,
			Models:          s.mapModelMetrics(channel.ID, nil, status),
		})
	}
	return items
}

func ParseMetricsHours(raw string) int {
	hours, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return MetricsHoursDefault
	}
	return NormalizeMetricsHours(hours)
}

func normalizeMetricsBaseURL(raw string) (string, error) {
	base := strings.TrimRight(strings.TrimSpace(raw), "/")
	if base == "" {
		return "", nil
	}
	parsed, err := url.ParseRequestURI(base)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("metrics_base_url must be a valid http or https URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("metrics_base_url must be a valid http or https URL")
	}
	return base, nil
}

func resolveMetricsBaseURL(channel model.Channel) string {
	if channel.MetricsBaseUrl != nil && *channel.MetricsBaseUrl != "" {
		return *channel.MetricsBaseUrl
	}
	return strings.TrimRight(channel.BaseUrl, "/") + "/api"
}

func buildMetricsRequestURL(baseURL string, hours int, endpointPath string) (string, error) {
	base, err := normalizeMetricsBaseURL(baseURL)
	if err != nil {
		return "", err
	}
	if base == "" {
		return "", errors.New("metrics base url is not configured")
	}
	if strings.HasSuffix(strings.ToLower(base), "/api") {
		base = strings.TrimSuffix(base, "/api")
	}
	endpoint := base + endpointPath
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	query.Set("hours", strconv.Itoa(NormalizeMetricsHours(hours)))
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func float64Ptr(value float64) *float64 {
	return &value
}
