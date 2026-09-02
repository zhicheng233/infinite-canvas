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
)

type ChannelModelService struct {
	channelService channelModelChannelService
	channelRepo    channelModelChannelRepo
	modelRepo      channelModelRepo
	creditRepo     pricingMapReader
	httpClient     *http.Client
}

type channelModelChannelService interface {
	ListEnabled() ([]model.ChannelInfo, error)
	DecryptedApiKey(id uint) (string, error)
}

type channelModelChannelRepo interface {
	FindByID(id uint) (*model.Channel, error)
	Save(channel *model.Channel) error
}

type channelModelRepo interface {
	FindByID(id uint) (*model.ChannelModel, error)
	FindByChannelAndName(channelID uint, modelName string) (*model.ChannelModel, error)
	ListByChannel(channelID uint, enabledOnly bool) ([]model.ChannelModel, error)
	Save(item *model.ChannelModel) error
	Upsert(item *model.ChannelModel) error
	DeleteStaleModels(channelID uint, keepNames []string) error
}

type pricingMapReader interface {
	FindPricingMap(tenantID uint) (map[string]map[uint]model.CreditPricing, error)
}

func NewChannelModelService(channelService channelModelChannelService, channelRepo channelModelChannelRepo, modelRepo channelModelRepo, creditRepo pricingMapReader) *ChannelModelService {
	return &ChannelModelService{
		channelService: channelService,
		channelRepo:    channelRepo,
		modelRepo:      modelRepo,
		creditRepo:     creditRepo,
		httpClient:     &http.Client{Timeout: 30 * time.Second},
	}
}

func (s *ChannelModelService) ListTenantCatalog(tenantID uint) ([]model.ChannelCatalogItem, error) {
	if s.channelService == nil {
		return nil, errors.New("channel service is not configured")
	}
	channels, err := s.channelService.ListEnabled()
	if err != nil {
		return nil, err
	}
	result := make([]model.ChannelCatalogItem, 0, len(channels))
	for _, channel := range channels {
		models, err := s.ListUserCatalog(tenantID, channel.ID)
		if err != nil {
			return nil, err
		}
		result = append(result, model.ChannelCatalogItem{ChannelID: channel.ID, ChannelName: channel.Name, Models: models})
	}
	return result, nil
}

type discoveredModel struct {
	ID string `json:"id"`
}

type discoveredModelsResponse struct {
	Data []discoveredModel `json:"data"`
}

func uniqueDiscoveredModelNames(items []discoveredModel) []string {
	names := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item.ID)
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	return names
}

func (s *ChannelModelService) List(channelID uint, enabledOnly bool) ([]model.ChannelModelInfo, error) {
	items, err := s.modelRepo.ListByChannel(channelID, enabledOnly)
	if err != nil {
		return nil, err
	}
	result := make([]model.ChannelModelInfo, 0, len(items))
	for i := range items {
		info, err := channelModelToInfo(&items[i])
		if err != nil {
			return nil, err
		}
		result = append(result, info)
	}
	return result, nil
}

func (s *ChannelModelService) ListUserCatalog(tenantID, channelID uint) ([]model.ChannelModelInfo, error) {
	if s.creditRepo == nil {
		return nil, errors.New("credit repository is not configured")
	}
	channel, err := s.channelRepo.FindByID(channelID)
	if err != nil {
		return nil, err
	}
	if !channel.Enabled {
		return []model.ChannelModelInfo{}, nil
	}
	pricingMap, err := s.creditRepo.FindPricingMap(tenantID)
	if err != nil {
		return nil, err
	}
	items, err := s.modelRepo.ListByChannel(channelID, true)
	if err != nil {
		return nil, err
	}
	result := make([]model.ChannelModelInfo, 0, len(items))
	for i := range items {
		pricingByChannel, ok := pricingMap[items[i].ModelName]
		if !ok || len(pricingByChannel) == 0 {
			continue
		}
		pricing, hasPricing := pricingByChannel[items[i].ChannelID]
		if !hasPricing {
			pricing, hasPricing = pricingByChannel[0]
		}
		if !hasPricing || !pricing.HasValidPricingRule() {
			continue
		}
		info, err := channelModelToInfo(&items[i])
		if err != nil {
			return nil, err
		}
		result = append(result, info)
	}
	return result, nil
}

func (s *ChannelModelService) Update(id uint, input model.UpdateChannelModelInput) (*model.ChannelModelInfo, error) {
	item, err := s.modelRepo.FindByID(id)
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
		encoded, encodeErr := json.Marshal(input.VideoDurations)
		if encodeErr != nil {
			return nil, encodeErr
		}
		item.VideoDurations = string(encoded)
	}
	if input.VideoCustomizable != nil {
		item.VideoCustomizable = *input.VideoCustomizable
	}
	if input.SortOrder != nil {
		item.SortOrder = *input.SortOrder
	}
	if input.Capabilities != nil {
		if len(input.Capabilities) == 0 {
			return nil, errors.New("至少选择一个能力")
		}
		encoded, encodeErr := json.Marshal(input.Capabilities)
		if encodeErr != nil {
			return nil, encodeErr
		}
		item.Capabilities = string(encoded)
	}
	if input.VideoRoute != nil || input.VideoCustomConfig != nil {
		if err := applyVideoCustomConfig(item, input.VideoCustomConfig); err != nil {
			return nil, err
		}
	}
	if err := s.modelRepo.Save(item); err != nil {
		return nil, err
	}
	info, err := channelModelToInfo(item)
	if err != nil {
		return nil, err
	}
	return &info, nil
}

func (s *ChannelModelService) Sync(channelID uint) error {
	channel, err := s.channelRepo.FindByID(channelID)
	if err != nil {
		return err
	}
	if !channel.Enabled {
		return errors.New("channel is disabled")
	}
	apiKey, err := s.channelService.DecryptedApiKey(channelID)
	if err != nil {
		return err
	}
	channel.SyncStatus = "syncing"
	channel.SyncError = ""
	if err := s.channelRepo.Save(channel); err != nil {
		return err
	}

	requestURL := buildChannelModelsURL(channel.BaseUrl)
	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return s.markSyncFailure(channel, err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	response, err := s.httpClient.Do(req)
	if err != nil {
		return s.markSyncFailure(channel, err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return s.markSyncFailure(channel, fmt.Errorf("upstream returned HTTP %d", response.StatusCode))
	}
	var payload discoveredModelsResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return s.markSyncFailure(channel, err)
	}

	discoveredNames := uniqueDiscoveredModelNames(payload.Data)
	for _, name := range discoveredNames {
		item := &model.ChannelModel{ChannelID: channelID, ModelName: name, Enabled: true, Capabilities: defaultChannelModelCapabilitiesJSON(), ImageGenerateRoute: "auto", ImageEditRoute: "auto", VideoRoute: "auto"}
		if existing, findErr := s.modelRepo.FindByChannelAndName(channelID, name); findErr == nil {
			item = existing
			if strings.TrimSpace(item.Capabilities) == "" {
				item.Capabilities = defaultChannelModelCapabilitiesJSON()
			}
		} else if !errors.Is(findErr, gorm.ErrRecordNotFound) {
			return s.markSyncFailure(channel, findErr)
		}
		if err := s.modelRepo.Upsert(item); err != nil {
			return s.markSyncFailure(channel, err)
		}
	}

	if err := s.modelRepo.DeleteStaleModels(channelID, discoveredNames); err != nil {
		return s.markSyncFailure(channel, err)
	}

	now := time.Now()
	channel.SyncStatus = "success"
	channel.SyncError = ""
	channel.SyncedAt = &now
	return s.channelRepo.Save(channel)
}

func (s *ChannelModelService) markSyncFailure(channel *model.Channel, syncErr error) error {
	channel.SyncStatus = "failed"
	channel.SyncError = syncErr.Error()
	if len(channel.SyncError) > 500 {
		channel.SyncError = channel.SyncError[:500]
	}
	if err := s.channelRepo.Save(channel); err != nil {
		return err
	}
	return syncErr
}

func channelModelToInfo(item *model.ChannelModel) (model.ChannelModelInfo, error) {
	capabilities := make([]string, 0)
	if strings.TrimSpace(item.Capabilities) != "" {
		if err := json.Unmarshal([]byte(item.Capabilities), &capabilities); err != nil {
			return model.ChannelModelInfo{}, err
		}
	}
	durations := make([]int, 0)
	if strings.TrimSpace(item.VideoDurations) != "" {
		if err := json.Unmarshal([]byte(item.VideoDurations), &durations); err != nil {
			return model.ChannelModelInfo{}, err
		}
	}
	var customConfig *model.CustomVideoConfig
	if item.VideoRoute == "custom" {
		if strings.TrimSpace(item.VideoCustomConfig) == "" {
			return model.ChannelModelInfo{}, errors.New("video_custom_config 不能为空")
		}
		customConfig = &model.CustomVideoConfig{}
		if err := json.Unmarshal([]byte(item.VideoCustomConfig), customConfig); err != nil {
			return model.ChannelModelInfo{}, fmt.Errorf("解析 video_custom_config: %w", err)
		}
		if err := model.NormalizeAndValidateCustomVideoConfig(customConfig); err != nil {
			return model.ChannelModelInfo{}, fmt.Errorf("校验 video_custom_config: %w", err)
		}
	}
	return model.ChannelModelInfo{ID: item.ID, ChannelID: item.ChannelID, ModelName: item.ModelName, Capabilities: capabilities, Enabled: item.Enabled, ImageGenerateRoute: item.ImageGenerateRoute, ImageEditRoute: item.ImageEditRoute, VideoRoute: item.VideoRoute, VideoDurations: durations, VideoCustomizable: item.VideoCustomizable, VideoCustomConfig: customConfig, SortOrder: item.SortOrder}, nil
}

func applyVideoCustomConfig(item *model.ChannelModel, config *model.CustomVideoConfig) error {
	if item.VideoRoute != "custom" {
		item.VideoCustomConfig = ""
		return nil
	}
	if config == nil {
		return errors.New("自定义视频路由必须提供 video_custom_config")
	}
	if err := model.NormalizeAndValidateCustomVideoConfig(config); err != nil {
		return fmt.Errorf("video_custom_config 无效: %w", err)
	}
	encoded, err := json.Marshal(config)
	if err != nil {
		return err
	}
	item.VideoCustomConfig = string(encoded)
	return nil
}

func buildChannelModelsURL(baseURL string) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if strings.HasSuffix(strings.ToLower(base), "/v1") {
		return base + "/models"
	}
	return base + "/v1/models"
}
