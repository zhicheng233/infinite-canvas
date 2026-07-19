package service

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"infinite-canvas-server/model"
	"infinite-canvas-server/repository"
)

type ModelCallLogService struct {
	repo     *repository.ModelCallLogRepo
	userRepo *repository.UserRepo
}

type ModelCallLogInput struct {
	TenantID       uint
	UserID         uint
	Generation     string
	Model          string
	Method         string
	Path           string
	StatusCode     int
	ErrorMessage   string
	ErrorBody      []byte
	ChannelID      *uint
	ChannelModelID *uint
}

type ModelHealthSummary struct {
	Total24h     int64                    `json:"total_24h"`
	Total7d      int64                    `json:"total_7d"`
	TopModels    []ModelHealthModel       `json:"top_models"`
	RecentErrors []ModelHealthRecentError `json:"recent_errors"`
}

type ModelHealthModel struct {
	Model      string `json:"model"`
	Generation string `json:"generation"`
	Failures   int64  `json:"failures"`
	LastError  string `json:"last_error"`
}

type ModelHealthRecentError struct {
	ID           uint      `json:"id"`
	CreatedAt    time.Time `json:"created_at"`
	UserID       uint      `json:"user_id"`
	Username     string    `json:"username"`
	DisplayName  string    `json:"display_name"`
	Generation   string    `json:"generation"`
	Model        string    `json:"model"`
	Path         string    `json:"path"`
	StatusCode   int       `json:"status_code"`
	ErrorMessage string    `json:"error_message"`
}

func NewModelCallLogService(repo *repository.ModelCallLogRepo, userRepo *repository.UserRepo) *ModelCallLogService {
	return &ModelCallLogService{repo: repo, userRepo: userRepo}
}

func (s *ModelCallLogService) RecordSuccess(input ModelCallLogInput, responseTimeMs int) {
	if s == nil || s.repo == nil || input.UserID == 0 {
		return
	}
	username := ""
	displayName := ""
	if s.userRepo != nil {
		if user, err := s.userRepo.FindByID(input.UserID); err == nil && user != nil {
			username = user.Username
			displayName = user.DisplayName
		}
	}
	if err := s.repo.Create(&model.ModelCallLog{
		TenantID:       input.TenantID,
		UserID:         input.UserID,
		Username:       username,
		DisplayName:    displayName,
		Generation:     cleanShort(input.Generation, 20),
		Model:          cleanShort(input.Model, 100),
		Method:         cleanShort(strings.ToUpper(input.Method), 10),
		Path:           cleanShort(cleanPath(input.Path), 255),
		StatusCode:     input.StatusCode,
		IsSuccess:      true,
		ResponseTime:   responseTimeMs,
		ChannelID:      input.ChannelID,
		ChannelModelID: input.ChannelModelID,
	}); err != nil {
		log.Printf("record model call success failed: %v", err)
	}
}

func (s *ModelCallLogService) RecordFailure(input ModelCallLogInput) {
	if s == nil || s.repo == nil || input.UserID == 0 {
		return
	}
	username := ""
	displayName := ""
	if s.userRepo != nil {
		if user, err := s.userRepo.FindByID(input.UserID); err == nil && user != nil {
			username = user.Username
			displayName = user.DisplayName
		}
	}
	if err := s.repo.Create(&model.ModelCallLog{
		TenantID:       input.TenantID,
		UserID:         input.UserID,
		Username:       username,
		DisplayName:    displayName,
		Generation:     cleanShort(input.Generation, 20),
		Model:          cleanShort(input.Model, 100),
		Method:         cleanShort(strings.ToUpper(input.Method), 10),
		Path:           cleanShort(cleanPath(input.Path), 255),
		StatusCode:     input.StatusCode,
		ErrorMessage:   buildModelCallErrorSummary(input.StatusCode, input.ErrorBody, input.ErrorMessage),
		ErrorBody:      truncateString(string(input.ErrorBody), 10000),
		ChannelID:      input.ChannelID,
		ChannelModelID: input.ChannelModelID,
	}); err != nil {
		log.Printf("record model call failure failed: %v", err)
	}
}

func (s *ModelCallLogService) HealthSummary(tenantID uint) (*ModelHealthSummary, error) {
	logs, err := s.repo.ListSince(tenantID, time.Now().Add(-7*24*time.Hour), 500)
	if err != nil {
		return nil, err
	}
	summary := buildModelHealthSummary(logs, time.Now())
	return &summary, nil
}

func buildModelHealthSummary(logs []model.ModelCallLog, now time.Time) ModelHealthSummary {
	dayAgo := now.Add(-24 * time.Hour)
	weekAgo := now.Add(-7 * 24 * time.Hour)
	models := map[string]*ModelHealthModel{}
	latestAt := map[string]time.Time{}
	recentErrors := make([]ModelHealthRecentError, 0, 10)
	summary := ModelHealthSummary{}
	for _, item := range logs {
		if item.IsSuccess {
			continue
		}
		if item.CreatedAt.Before(weekAgo) {
			continue
		}
		summary.Total7d++
		if !item.CreatedAt.Before(dayAgo) {
			summary.Total24h++
		}
		key := modelHealthKey(item)
		row := models[key]
		if row == nil {
			row = &ModelHealthModel{Model: item.Model, Generation: item.Generation, LastError: item.ErrorMessage}
			models[key] = row
		}
		row.Failures++
		if item.CreatedAt.After(latestAt[key]) {
			row.LastError = item.ErrorMessage
			latestAt[key] = item.CreatedAt
		}
		if len(recentErrors) < 10 {
			recentErrors = append(recentErrors, ModelHealthRecentError{
				ID:           item.ID,
				CreatedAt:    item.CreatedAt,
				UserID:       item.UserID,
				Username:     item.Username,
				DisplayName:  item.DisplayName,
				Generation:   item.Generation,
				Model:        item.Model,
				Path:         item.Path,
				StatusCode:   item.StatusCode,
				ErrorMessage: item.ErrorMessage,
			})
		}
	}
	summary.TopModels = topModelFailures(models, 5)
	summary.RecentErrors = recentErrors
	return summary
}

func modelHealthKey(item model.ModelCallLog) string {
	channelID := uint(0)
	channelModelID := uint(0)
	if item.ChannelID != nil {
		channelID = *item.ChannelID
	}
	if item.ChannelModelID != nil {
		channelModelID = *item.ChannelModelID
	}
	return fmt.Sprintf("%d\x00%d\x00%s\x00%s", channelID, channelModelID, item.Generation, item.Model)
}

func topModelFailures(models map[string]*ModelHealthModel, limit int) []ModelHealthModel {
	items := make([]ModelHealthModel, 0, len(models))
	for _, item := range models {
		items = append(items, *item)
	}
	for index := 0; index < len(items); index++ {
		for compareIndex := index + 1; compareIndex < len(items); compareIndex++ {
			if items[compareIndex].Failures > items[index].Failures {
				items[index], items[compareIndex] = items[compareIndex], items[index]
			}
		}
	}
	if len(items) > limit {
		return items[:limit]
	}
	return items
}

func buildModelCallErrorSummary(statusCode int, body []byte, fallback string) string {
	message := strings.TrimSpace(fallback)
	if message == "" {
		message = readErrorMessage(body)
	}
	if message == "" && len(body) > 0 {
		message = strings.TrimSpace(string(body))
	}
	if message == "" && statusCode > 0 {
		message = fmt.Sprintf("HTTP %d", statusCode)
	}
	return truncateString(message, 500)
}

func readErrorMessage(body []byte) string {
	var payload map[string]interface{}
	if json.Unmarshal(body, &payload) != nil {
		return ""
	}
	return unwrapErrorMessage(readStringPath(payload, "error.message", "data.error.message", "data.message", "message", "msg", "detail", "code"))
}

func readFailedModelTaskResponse(body []byte) (bool, string, string) {
	var payload map[string]interface{}
	if json.Unmarshal(body, &payload) != nil {
		return false, "", ""
	}
	status := strings.ToLower(readStringPath(payload, "status", "state", "task_status", "data.status", "data.state", "data.task_status"))

	// 视频轮询中间状态不算失败
	if status == "processing" || status == "in_progress" || status == "queued" || status == "pending" {
		return false, "", ""
	}
	code := strings.ToLower(readStringPath(payload, "code"))
	success, hasSuccess := readBoolPath(payload, "success", "data.success")
	failed := status == "failed" || status == "error" || status == "cancelled" || (hasSuccess && !success) || (code != "" && code != "success" && code != "ok")
	if !failed {
		return false, "", ""
	}
	return true, readStringPath(payload, "model", "data.model", "data.properties.upstream_model_name", "data.properties.origin_model_name", "properties.upstream_model_name", "properties.origin_model_name"), buildModelCallErrorSummary(200, body, "")
}

func readStringPath(payload map[string]interface{}, paths ...string) string {
	for _, path := range paths {
		var value interface{} = payload
		for _, key := range strings.Split(path, ".") {
			next, ok := value.(map[string]interface{})
			if !ok {
				value = nil
				break
			}
			value = next[key]
		}
		if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
			return strings.TrimSpace(text)
		}
	}
	return ""
}

func readBoolPath(payload map[string]interface{}, paths ...string) (bool, bool) {
	for _, path := range paths {
		var value interface{} = payload
		for _, key := range strings.Split(path, ".") {
			next, ok := value.(map[string]interface{})
			if !ok {
				value = nil
				break
			}
			value = next[key]
		}
		if flag, ok := value.(bool); ok {
			return flag, true
		}
	}
	return false, false
}

func unwrapErrorMessage(message string) string {
	message = strings.TrimSpace(message)
	for i := 0; i < 3 && strings.HasPrefix(message, "{"); i++ {
		var payload map[string]interface{}
		if json.Unmarshal([]byte(message), &payload) != nil {
			return message
		}
		next := readStringPath(payload, "error.message", "message", "msg", "detail", "code")
		if next == "" || next == message {
			return message
		}
		message = strings.TrimSpace(next)
	}
	return message
}

func cleanPath(path string) string {
	return strings.Split(strings.TrimSpace(path), "?")[0]
}

func cleanShort(value string, limit int) string {
	return truncateString(strings.TrimSpace(value), limit)
}

func truncateString(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
