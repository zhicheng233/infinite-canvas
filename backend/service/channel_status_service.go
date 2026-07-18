package service

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"infinite-canvas-server/model"
	"infinite-canvas-server/repository"
)

type ChannelStatusService struct {
	logRepo       *repository.ModelCallLogRepo
	apiConfigRepo *repository.ApiConfigRepo
}

type ChannelStatusResponse struct {
	Models    []ModelChannelStatus `json:"models"`
	UpdatedAt time.Time            `json:"updated_at"`
}

type ModelChannelStatus struct {
	Model        string               `json:"model"`
	Generation   string               `json:"generation"`
	DisplayName  string               `json:"display_name"`
	Status       string               `json:"status"`
	Uptime1d     float64              `json:"uptime_1d"`
	Uptime7d     float64              `json:"uptime_7d"`
	Uptime15d    float64              `json:"uptime_15d"`
	Uptime30d    float64              `json:"uptime_30d"`
	AvgResponse  int                  `json:"avg_response_ms"`
	Timeline     []TimelinePoint      `json:"timeline"`
	RecentErrors []RecentErrorSummary `json:"recent_errors,omitempty"`
}

type TimelinePoint struct {
	Timestamp time.Time `json:"timestamp"`
	Status    string    `json:"status"`
	Uptime    float64   `json:"uptime"`
}

type RecentErrorSummary struct {
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`
	Count     int       `json:"count"`
}

func NewChannelStatusService(logRepo *repository.ModelCallLogRepo, apiConfigRepo *repository.ApiConfigRepo) *ChannelStatusService {
	return &ChannelStatusService{logRepo: logRepo, apiConfigRepo: apiConfigRepo}
}

func (s *ChannelStatusService) GetChannelStatus(tenantID uint, days int) (*ChannelStatusResponse, error) {
	if days <= 0 {
		days = 7
	}
	if days > 30 {
		days = 30
	}

	since := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	logs, err := s.logRepo.ListSince(tenantID, since, 100000)
	if err != nil {
		return nil, err
	}

	var cfg *model.TenantApiConfig
	if s.apiConfigRepo != nil {
		cfg, _ = s.apiConfigRepo.FindByTenant(tenantID)
	}
	models := s.aggregateByModel(logs, cfg, days)
	return &ChannelStatusResponse{
		Models:    models,
		UpdatedAt: time.Now(),
	}, nil
}

func (s *ChannelStatusService) aggregateByModel(logs []model.ModelCallLog, cfg *model.TenantApiConfig, days int) []ModelChannelStatus {
	modelMap := make(map[string]*modelAggregator)
	now := time.Now()

	for _, log := range logs {
		if strings.TrimSpace(log.Model) == "" {
			continue
		}
		generation := resolveGenerationByApiConfig(cfg, log.Model, log.Generation)
		key := fmt.Sprintf("%s|%s", generation, log.Model)
		if modelMap[key] == nil {
			modelMap[key] = &modelAggregator{
				generation: generation,
				model:      log.Model,
				records:    []model.ModelCallLog{},
			}
		}
		modelMap[key].records = append(modelMap[key].records, log)
	}

	results := []ModelChannelStatus{}
	for _, agg := range modelMap {
		status := s.buildModelStatus(agg, now, days)
		results = append(results, status)
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Generation != results[j].Generation {
			return results[i].Generation < results[j].Generation
		}
		return results[i].Model < results[j].Model
	})

	return results
}

type modelAggregator struct {
	generation string
	model      string
	records    []model.ModelCallLog
}

func (s *ChannelStatusService) buildModelStatus(agg *modelAggregator, now time.Time, days int) ModelChannelStatus {
	uptime1d := s.calculateUptime(agg.records, now, 1)
	uptime7d := s.calculateUptime(agg.records, now, 7)
	uptime15d := s.calculateUptime(agg.records, now, 15)
	uptime30d := s.calculateUptime(agg.records, now, 30)
	avgResponse := s.calculateAvgResponse(agg.records, now, days)
	timeline := s.buildTimeline(agg.records, now, days)
	recentErrors := s.extractRecentErrors(agg.records, 3)

	status := "operational"
	primaryUptime := uptime1d
	if days >= 30 {
		primaryUptime = uptime30d
	} else if days >= 15 {
		primaryUptime = uptime15d
	} else if days >= 7 {
		primaryUptime = uptime7d
	}

	if primaryUptime < 95.0 {
		status = "down"
	} else if primaryUptime < 99.0 {
		status = "degraded"
	}

	displayName := s.formatDisplayName(agg.generation, agg.model)

	return ModelChannelStatus{
		Model:        agg.model,
		Generation:   agg.generation,
		DisplayName:  displayName,
		Status:       status,
		Uptime1d:     uptime1d,
		Uptime7d:     uptime7d,
		Uptime15d:    uptime15d,
		Uptime30d:    uptime30d,
		AvgResponse:  avgResponse,
		Timeline:     timeline,
		RecentErrors: recentErrors,
	}
}

func (s *ChannelStatusService) calculateUptime(records []model.ModelCallLog, now time.Time, days int) float64 {
	cutoff := now.Add(-time.Duration(days) * 24 * time.Hour)
	total := 0
	success := 0

	for _, record := range records {
		if record.CreatedAt.Before(cutoff) {
			continue
		}
		total++
		if record.IsSuccess {
			success++
		}
	}

	if total == 0 {
		return 100.0
	}
	return math.Round(float64(success)*10000.0/float64(total)) / 100.0
}

func (s *ChannelStatusService) calculateAvgResponse(records []model.ModelCallLog, now time.Time, days int) int {
	cutoff := now.Add(-time.Duration(days) * 24 * time.Hour)
	sum := 0
	count := 0

	for _, record := range records {
		if record.CreatedAt.Before(cutoff) || !record.IsSuccess || record.ResponseTime == 0 {
			continue
		}
		sum += record.ResponseTime
		count++
	}

	if count == 0 {
		return 0
	}
	return sum / count
}

func (s *ChannelStatusService) buildTimeline(records []model.ModelCallLog, now time.Time, days int) []TimelinePoint {
	cutoff := now.Add(-time.Duration(days) * 24 * time.Hour)
	buckets := make(map[string]*timelineBucket)

	for _, record := range records {
		if record.CreatedAt.Before(cutoff) {
			continue
		}
		hour := record.CreatedAt.Truncate(time.Hour)
		key := hour.Format(time.RFC3339)
		if buckets[key] == nil {
			buckets[key] = &timelineBucket{timestamp: hour}
		}
		buckets[key].total++
		if record.IsSuccess {
			buckets[key].success++
		}
	}

	points := []TimelinePoint{}
	for _, bucket := range buckets {
		uptime := 100.0
		if bucket.total > 0 {
			uptime = math.Round(float64(bucket.success)*10000.0/float64(bucket.total)) / 100.0
		}
		status := "operational"
		if uptime < 95.0 {
			status = "down"
		} else if uptime < 99.0 {
			status = "degraded"
		}
		points = append(points, TimelinePoint{
			Timestamp: bucket.timestamp,
			Status:    status,
			Uptime:    uptime,
		})
	}

	sort.Slice(points, func(i, j int) bool {
		return points[i].Timestamp.Before(points[j].Timestamp)
	})

	return points
}

type timelineBucket struct {
	timestamp time.Time
	total     int
	success   int
}

func (s *ChannelStatusService) extractRecentErrors(records []model.ModelCallLog, limit int) []RecentErrorSummary {
	errors := []model.ModelCallLog{}
	for i := len(records) - 1; i >= 0 && len(errors) < limit*3; i-- {
		if !records[i].IsSuccess && records[i].ErrorMessage != "" {
			errors = append(errors, records[i])
		}
	}

	grouped := make(map[string]*RecentErrorSummary)
	for _, err := range errors {
		msg := strings.TrimSpace(err.ErrorMessage)
		if msg == "" {
			continue
		}
		if len(msg) > 100 {
			msg = msg[:100] + "..."
		}
		if grouped[msg] == nil {
			grouped[msg] = &RecentErrorSummary{
				Timestamp: err.CreatedAt,
				Message:   msg,
				Count:     0,
			}
		}
		grouped[msg].Count++
		if err.CreatedAt.After(grouped[msg].Timestamp) {
			grouped[msg].Timestamp = err.CreatedAt
		}
	}

	result := []RecentErrorSummary{}
	for _, summary := range grouped {
		result = append(result, *summary)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.After(result[j].Timestamp)
	})

	if len(result) > limit {
		result = result[:limit]
	}

	return result
}

func (s *ChannelStatusService) formatDisplayName(generation, model string) string {
	parts := []string{}
	if generation != "" {
		switch generation {
		case "image":
			parts = append(parts, "图像生成")
		case "video":
			parts = append(parts, "视频生成")
		case "text":
			parts = append(parts, "文本生成")
		case "audio":
			parts = append(parts, "音频生成")
		default:
			parts = append(parts, generation)
		}
	}
	if model != "" {
		parts = append(parts, model)
	}
	return strings.Join(parts, " - ")
}
