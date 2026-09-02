package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"gorm.io/gorm"
	"infinite-canvas-server/model"
	"infinite-canvas-server/repository"
)

type ChannelModelSyncService struct {
	channelRepo      *repository.ChannelRepo
	channelModelRepo *repository.ChannelModelRepo
	channelService   *ChannelService
	httpClient       *http.Client
}

func NewChannelModelSyncService(channelRepo *repository.ChannelRepo, channelModelRepo *repository.ChannelModelRepo, channelService *ChannelService) *ChannelModelSyncService {
	return &ChannelModelSyncService{
		channelRepo:      channelRepo,
		channelModelRepo: channelModelRepo,
		channelService:   channelService,
		httpClient:       &http.Client{Timeout: 2 * time.Minute},
	}
}

type upstreamModelList struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

func (s *ChannelModelSyncService) Sync(channelID uint) ([]model.ChannelModelInfo, error) {
	channel, err := s.channelRepo.FindByID(channelID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(channel.BaseUrl) == "" {
		return nil, errors.New("渠道未配置上游地址")
	}

	channel.SyncStatus = "syncing"
	channel.SyncError = ""
	_ = s.channelRepo.Save(channel)

	apiKey, err := s.channelService.DecryptedApiKey(channelID)
	if err != nil {
		return s.failSync(channel, err)
	}
	request, err := http.NewRequest(http.MethodGet, buildUpstreamURL(channel.BaseUrl, "/models"), nil)
	if err != nil {
		return s.failSync(channel, err)
	}
	request.Header.Set("Authorization", "Bearer "+apiKey)
	response, err := s.httpClient.Do(request)
	if err != nil {
		return s.failSync(channel, err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return s.failSync(channel, fmt.Errorf("上游模型接口返回 HTTP %d", response.StatusCode))
	}
	var payload upstreamModelList
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return s.failSync(channel, err)
	}

	seen := make(map[string]struct{}, len(payload.Data))
	for _, item := range payload.Data {
		name := strings.TrimSpace(item.ID)
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		existing, findErr := s.channelModelRepo.FindByChannelAndName(channelID, name)
		if findErr != nil && !errors.Is(findErr, gorm.ErrRecordNotFound) {
			return s.failSync(channel, findErr)
		}
		itemModel := &model.ChannelModel{ChannelID: channelID, ModelName: name, Enabled: true, Capabilities: defaultChannelModelCapabilitiesJSON(), ImageGenerateRoute: "auto", ImageEditRoute: "auto", VideoRoute: "auto", VideoDurations: "[]", VideoCustomConfig: ""}
		if findErr == nil {
			itemModel = existing
			if strings.TrimSpace(itemModel.Capabilities) == "" {
				itemModel.Capabilities = defaultChannelModelCapabilitiesJSON()
			}
		}
		if err := s.channelModelRepo.Upsert(itemModel); err != nil {
			return s.failSync(channel, err)
		}
	}

	now := time.Now()
	channel.SyncStatus = "success"
	channel.SyncError = ""
	channel.SyncedAt = &now
	if err := s.channelRepo.Save(channel); err != nil {
		return nil, err
	}
	return s.listInfo(channelID)
}

func (s *ChannelModelSyncService) failSync(channel *model.Channel, syncErr error) ([]model.ChannelModelInfo, error) {
	channel.SyncStatus = "failed"
	channel.SyncError = syncErr.Error()
	_ = s.channelRepo.Save(channel)
	return nil, syncErr
}

func (s *ChannelModelSyncService) List(channelID uint, enabledOnly bool) ([]model.ChannelModelInfo, error) {
	return s.listInfoWithEnabled(channelID, enabledOnly)
}

func (s *ChannelModelSyncService) Update(id uint, input model.UpdateChannelModelInput) (*model.ChannelModelInfo, error) {
	item, err := s.channelModelRepo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if input.Enabled != nil {
		item.Enabled = *input.Enabled
	}
	if input.ImageGenerateRoute != nil {
		route, routeErr := model.NormalizeImageGenerateRoute(*input.ImageGenerateRoute)
		if routeErr != nil {
			return nil, routeErr
		}
		item.ImageGenerateRoute = route
	}
	if input.ImageEditRoute != nil {
		route, routeErr := model.NormalizeImageEditRoute(*input.ImageEditRoute)
		if routeErr != nil {
			return nil, routeErr
		}
		item.ImageEditRoute = route
	}
	if input.VideoRoute != nil {
		route, routeErr := model.NormalizeVideoRoute(*input.VideoRoute)
		if routeErr != nil {
			return nil, routeErr
		}
		item.VideoRoute = route
	}
	if input.VideoDurations != nil {
		encoded, marshalErr := json.Marshal(input.VideoDurations)
		if marshalErr != nil {
			return nil, marshalErr
		}
		item.VideoDurations = string(encoded)
	}
	if input.VideoCustomizable != nil {
		item.VideoCustomizable = *input.VideoCustomizable
	}
	if input.SortOrder != nil {
		item.SortOrder = *input.SortOrder
	}
	if err := applyVideoCustomConfig(item, input.VideoCustomConfig); err != nil {
		return nil, err
	}
	if err := s.channelModelRepo.Save(item); err != nil {
		return nil, err
	}
	converted, err := channelModelToInfo(item)
	if err != nil {
		return nil, err
	}
	return &converted, nil
}

func (s *ChannelModelSyncService) listInfo(channelID uint) ([]model.ChannelModelInfo, error) {
	return s.listInfoWithEnabled(channelID, false)
}

func (s *ChannelModelSyncService) listInfoWithEnabled(channelID uint, enabledOnly bool) ([]model.ChannelModelInfo, error) {
	items, err := s.channelModelRepo.ListByChannel(channelID, enabledOnly)
	if err != nil {
		return nil, err
	}
	result := make([]model.ChannelModelInfo, 0, len(items))
	for index := range items {
		info, err := channelModelToInfo(&items[index])
		if err != nil {
			return nil, err
		}
		result = append(result, info)
	}
	return result, nil
}
