package handler

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"infinite-canvas-server/config"
	"infinite-canvas-server/crypto"
	"infinite-canvas-server/model"
	"infinite-canvas-server/repository"
	"infinite-canvas-server/service"
)

type ApiConfigHandler struct {
	apiConfigRepo  apiConfigRepository
	creditRepo     apiConfigPricingReader
	channelCatalog apiConfigChannelCatalog
	generateSvc    *service.GenerateService
	legacyConfig   legacyAPIConfigWriter
	cfg            *config.Config
}

type apiConfigRepository interface {
	FindByTenant(tenantID uint) (*model.TenantApiConfig, error)
}

type legacyAPIConfigWriter interface {
	Save(config *model.TenantApiConfig, actorUserID uint) error
}

type apiConfigPricingReader interface {
	FindPricingMap(tenantID uint) (map[string]map[uint]model.CreditPricing, error)
}

type apiConfigChannelCatalog interface {
	ListTenantCatalog(tenantID uint) ([]model.ChannelCatalogItem, error)
}

func NewApiConfigHandler(apiConfigRepo *repository.ApiConfigRepo, creditRepo apiConfigPricingReader, channelCatalog apiConfigChannelCatalog, generateSvc *service.GenerateService, legacyConfig *service.LegacyAPIConfigService, cfg *config.Config) *ApiConfigHandler {
	return &ApiConfigHandler{apiConfigRepo: apiConfigRepo, creditRepo: creditRepo, channelCatalog: channelCatalog, generateSvc: generateSvc, legacyConfig: legacyConfig, cfg: cfg}
}

type SaveApiConfigInput struct {
	BaseUrl                string            `json:"base_url"`
	ApiKey                 string            `json:"api_key"`
	Models                 []string          `json:"models"`
	ImageModels            []string          `json:"image_models"`
	VideoModels            []string          `json:"video_models"`
	TextModels             []string          `json:"text_models"`
	AudioModels            []string          `json:"audio_models"`
	ModelRoutes            map[string]string `json:"model_routes"`
	ModelVideoDurations    map[string][]int  `json:"model_video_durations"`
	ModelVideoCustomizable map[string]bool   `json:"model_video_customizable"`
}

func (h *ApiConfigHandler) Get(c *gin.Context) {
	claims := c.MustGet("claims").(*service.Claims)
	if h.apiConfigRepo != nil {
		cfg, err := h.apiConfigRepo.FindByTenant(claims.TenantID)
		if err == nil {
			models, _ := decodeStringList(cfg.Models)
			imageModels, _ := decodeStringList(cfg.ImageModels)
			videoModels, _ := decodeStringList(cfg.VideoModels)
			textModels, _ := decodeStringList(cfg.TextModels)
			audioModels, _ := decodeStringList(cfg.AudioModels)
			modelRoutes, _ := decodeStringMap(cfg.ModelRoutes)
			modelVideoDurations, _ := decodeIntListMap(cfg.ModelVideoDurations)
			modelVideoCustomizable, _ := decodeBoolMap(cfg.ModelVideoCustomizable)
			model.OK(c, gin.H{
				"base_url": cfg.BaseUrl, "has_key": len(cfg.ApiKey) > 0,
				"models": models, "image_models": imageModels, "video_models": videoModels,
				"text_models": textModels, "audio_models": audioModels, "model_routes": modelRoutes,
				"model_video_durations": modelVideoDurations, "model_video_customizable": modelVideoCustomizable,
				"deprecated": true, "deprecation": "旧版 API 配置将在 v0.6 变为只读，请迁移到模型服务",
			})
			return
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			model.Fail(c, 500, "读取旧版 API 配置失败")
			return
		}
	}
	channels, err := h.channelCatalog.ListTenantCatalog(claims.TenantID)
	if err != nil {
		model.Fail(c, 500, "读取渠道模型失败")
		return
	}
	summary := summarizeChannelCatalog(channels)
	model.OK(c, gin.H{
		"base_url": "", "has_key": false,
		"models": summary.models, "image_models": summary.byCapability["image"],
		"video_models": summary.byCapability["video"], "text_models": summary.byCapability["text"],
		"audio_models": summary.byCapability["audio"], "model_routes": map[string]string{},
		"model_video_durations": map[string][]int{}, "model_video_customizable": map[string]bool{},
		"deprecated": true, "deprecation": "旧版 API 配置将在 v0.6 变为只读，请迁移到模型服务",
	})
}

func (h *ApiConfigHandler) Catalog(c *gin.Context) {
	claims := c.MustGet("claims").(*service.Claims)
	pricingMap, err := h.creditRepo.FindPricingMap(claims.TenantID)
	if err != nil {
		model.Fail(c, 500, "读取定价配置失败")
		return
	}
	channels, err := h.channelCatalog.ListTenantCatalog(claims.TenantID)
	if err != nil {
		model.Fail(c, 500, "读取渠道模型失败")
		return
	}
	summary := summarizeChannelCatalog(channels)
	model.OK(c, gin.H{
		"models":                   summary.models,
		"image_models":             summary.byCapability["image"],
		"video_models":             summary.byCapability["video"],
		"text_models":              summary.byCapability["text"],
		"audio_models":             summary.byCapability["audio"],
		"priced_models":            summary.models,
		"pricing_map":              pricingMap,
		"model_routes":             map[string]string{},
		"model_video_durations":    map[string][]int{},
		"model_video_customizable": map[string]bool{},
		"total_models":             len(summary.models),
		"enabled_count":            len(summary.models),
		"disabled_models":          []string{},
		"channels":                 channels,
		"deprecated":               true,
		"deprecation":              "旧版模型目录读取接口将在后续版本移除",
	})
}

func (h *ApiConfigHandler) Save(c *gin.Context) {
	claims := c.MustGet("claims").(*service.Claims)
	var input SaveApiConfigInput
	if err := c.ShouldBindJSON(&input); err != nil {
		model.Fail(c, 400, "无效的请求参数")
		return
	}
	if h.apiConfigRepo == nil || h.legacyConfig == nil || h.cfg == nil {
		model.Fail(c, 500, "旧版 API 配置兼容服务不可用")
		return
	}
	models, err := encodeStringList(input.Models)
	if err != nil {
		model.Fail(c, 400, "模型列表格式错误")
		return
	}
	imageModels, err := encodeStringList(input.ImageModels)
	if err != nil {
		model.Fail(c, 400, "图片模型列表格式错误")
		return
	}
	videoModels, err := encodeStringList(input.VideoModels)
	if err != nil {
		model.Fail(c, 400, "视频模型列表格式错误")
		return
	}
	textModels, err := encodeStringList(input.TextModels)
	if err != nil {
		model.Fail(c, 400, "文本模型列表格式错误")
		return
	}
	audioModels, err := encodeStringList(input.AudioModels)
	if err != nil {
		model.Fail(c, 400, "音频模型列表格式错误")
		return
	}
	modelRoutes, err := encodeStringMap(input.ModelRoutes)
	if err != nil {
		model.Fail(c, 400, "模型路由配置格式错误")
		return
	}
	modelVideoDurations, err := encodeIntListMap(input.ModelVideoDurations)
	if err != nil {
		model.Fail(c, 400, "视频时长配置格式错误")
		return
	}
	modelVideoCustomizable, err := encodeBoolMap(input.ModelVideoCustomizable)
	if err != nil {
		model.Fail(c, 400, "视频自定义配置格式错误")
		return
	}

	existing, _ := h.apiConfigRepo.FindByTenant(claims.TenantID)
	encryptedKey := ""
	if strings.TrimSpace(input.ApiKey) == "" {
		if existing == nil || strings.TrimSpace(existing.ApiKey) == "" {
			model.Fail(c, 400, "请填写 API Key")
			return
		}
		encryptedKey = existing.ApiKey
	} else {
		encryptedKey, err = crypto.Encrypt(h.cfg.ApiKeyEncryptKey, input.ApiKey)
		if err != nil {
			model.Fail(c, 500, "加密 API Key 失败")
			return
		}
	}
	if strings.TrimSpace(input.BaseUrl) == "" {
		model.Fail(c, 400, "请填写 Base URL")
		return
	}
	cfg := &model.TenantApiConfig{
		TenantID: claims.TenantID, BaseUrl: strings.TrimRight(strings.TrimSpace(input.BaseUrl), "/"), ApiKey: encryptedKey,
		Models: models, ImageModels: imageModels, VideoModels: videoModels, TextModels: textModels, AudioModels: audioModels,
		ModelRoutes: modelRoutes, ModelVideoDurations: modelVideoDurations, ModelVideoCustomizable: modelVideoCustomizable,
	}
	if err := h.legacyConfig.Save(cfg, claims.UserID); err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.OK(c, gin.H{"saved": true, "deprecated": true, "deprecation": "旧版 API 配置将在 v0.6 变为只读，请迁移到模型服务"})
}

type channelCatalogSummary struct {
	models       []string
	byCapability map[string][]string
}

func summarizeChannelCatalog(channels []model.ChannelCatalogItem) channelCatalogSummary {
	result := channelCatalogSummary{models: []string{}, byCapability: map[string][]string{"image": {}, "video": {}, "text": {}, "audio": {}}}
	allSeen := make(map[string]struct{})
	capabilitySeen := map[string]map[string]struct{}{"image": {}, "video": {}, "text": {}, "audio": {}}
	for _, channel := range channels {
		for _, item := range channel.Models {
			name := strings.TrimSpace(item.ModelName)
			if name == "" {
				continue
			}
			if _, ok := allSeen[name]; !ok {
				allSeen[name] = struct{}{}
				result.models = append(result.models, name)
			}
			for _, capability := range item.Capabilities {
				if _, ok := result.byCapability[capability]; !ok {
					continue
				}
				if _, ok := capabilitySeen[capability][name]; ok {
					continue
				}
				capabilitySeen[capability][name] = struct{}{}
				result.byCapability[capability] = append(result.byCapability[capability], name)
			}
		}
	}
	return result
}

func (h *ApiConfigHandler) TestModel(c *gin.Context) {
	claims := c.MustGet("claims").(*service.Claims)
	var input service.ModelTestInput
	if err := c.ShouldBindJSON(&input); err != nil {
		model.Fail(c, 400, "无效的请求参数")
		return
	}
	result, err := h.generateSvc.TestModel(claims.TenantID, claims.UserID, input)
	if err != nil {
		model.Fail(c, 400, err.Error())
		return
	}
	model.OK(c, result)
}

func filterModelsByPricing(models []string, pricingMap map[string]map[uint]model.CreditPricing) []string {
	if len(models) == 0 {
		return []string{}
	}
	items := make([]string, 0, len(models))
	seen := make(map[string]struct{}, len(models))
	for _, item := range models {
		name := strings.TrimSpace(item)
		if name == "" {
			continue
		}
		pricingByChannel, exists := pricingMap[name]
		if !exists || len(pricingByChannel) == 0 || !hasPricingRule(pricingByChannel) {
			continue
		}
		if _, duplicated := seen[name]; duplicated {
			continue
		}
		seen[name] = struct{}{}
		items = append(items, name)
	}
	return items
}

func collectDisabledModels(models []string, pricingMap map[string]map[uint]model.CreditPricing) []string {
	items := make([]string, 0)
	seen := make(map[string]struct{})
	for _, item := range models {
		name := strings.TrimSpace(item)
		if name == "" {
			continue
		}
		if pricingByChannel, ok := pricingMap[name]; ok && len(pricingByChannel) > 0 && hasPricingRule(pricingByChannel) {
			continue
		}
		if _, duplicated := seen[name]; duplicated {
			continue
		}
		seen[name] = struct{}{}
		items = append(items, name)
	}
	return items
}

func filterModelDurationsByPricing(items map[string][]int, pricingMap map[string]map[uint]model.CreditPricing) map[string][]int {
	if len(items) == 0 {
		return map[string][]int{}
	}
	filtered := make(map[string][]int, len(items))
	for modelName, durations := range items {
		pricingByChannel, ok := pricingMap[modelName]
		if !ok || len(pricingByChannel) == 0 || !hasPricingRule(pricingByChannel) {
			continue
		}
		filtered[modelName] = append([]int(nil), durations...)
	}
	return filtered
}

func filterBoolMapByPricing(items map[string]bool, pricingMap map[string]map[uint]model.CreditPricing) map[string]bool {
	if len(items) == 0 {
		return map[string]bool{}
	}
	filtered := make(map[string]bool, len(items))
	for modelName, enabled := range items {
		if !enabled {
			continue
		}
		pricingByChannel, ok := pricingMap[modelName]
		if !ok || len(pricingByChannel) == 0 || !hasPricingRule(pricingByChannel) {
			continue
		}
		filtered[modelName] = true
	}
	return filtered
}

// hasPricingRule checks whether any channel in the nested pricing map has a valid pricing rule.
func hasPricingRule(pricingByChannel map[uint]model.CreditPricing) bool {
	for _, p := range pricingByChannel {
		if p.HasValidPricingRule() {
			return true
		}
	}
	return false
}

func encodeStringList(items []string) (string, error) {
	if len(items) == 0 {
		return "[]", nil
	}
	returnValue, err := json.Marshal(items)
	if err != nil {
		return "", err
	}
	return string(returnValue), nil
}

func decodeStringList(raw string) ([]string, error) {
	if raw == "" {
		return []string{}, nil
	}
	var items []string
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return []string{}, err
	}
	return items, nil
}

func encodeStringMap(items map[string]string) (string, error) {
	if len(items) == 0 {
		return "{}", nil
	}
	cleaned := make(map[string]string, len(items))
	for key, value := range items {
		model := strings.TrimSpace(key)
		route := strings.TrimSpace(value)
		if model == "" || route == "" || route == "auto" {
			continue
		}
		cleaned[model] = route
	}
	returnValue, err := json.Marshal(cleaned)
	if err != nil {
		return "", err
	}
	return string(returnValue), nil
}

func decodeStringMap(raw string) (map[string]string, error) {
	if raw == "" {
		return map[string]string{}, nil
	}
	var items map[string]string
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return map[string]string{}, err
	}
	if items == nil {
		items = map[string]string{}
	}
	return items, nil
}

func encodeIntListMap(items map[string][]int) (string, error) {
	if len(items) == 0 {
		return "{}", nil
	}
	cleaned := make(map[string][]int, len(items))
	for key, values := range items {
		modelName := strings.TrimSpace(key)
		if modelName == "" {
			continue
		}
		seen := make(map[int]struct{}, len(values))
		list := make([]int, 0, len(values))
		for _, value := range values {
			if value <= 0 {
				continue
			}
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			list = append(list, value)
		}
		sort.Ints(list)
		if len(list) == 0 {
			continue
		}
		cleaned[modelName] = list
	}
	returnValue, err := json.Marshal(cleaned)
	if err != nil {
		return "", err
	}
	return string(returnValue), nil
}

func decodeIntListMap(raw string) (map[string][]int, error) {
	if raw == "" {
		return map[string][]int{}, nil
	}
	var items map[string][]int
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return map[string][]int{}, err
	}
	if items == nil {
		items = map[string][]int{}
	}
	cleaned := make(map[string][]int, len(items))
	for key, values := range items {
		modelName := strings.TrimSpace(key)
		if modelName == "" {
			continue
		}
		seen := make(map[int]struct{}, len(values))
		list := make([]int, 0, len(values))
		for _, value := range values {
			if value <= 0 {
				continue
			}
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			list = append(list, value)
		}
		sort.Ints(list)
		if len(list) == 0 {
			continue
		}
		cleaned[modelName] = list
	}
	return cleaned, nil
}

func encodeBoolMap(items map[string]bool) (string, error) {
	if len(items) == 0 {
		return "{}", nil
	}
	cleaned := make(map[string]bool, len(items))
	for key, value := range items {
		modelName := strings.TrimSpace(key)
		if modelName == "" || !value {
			continue
		}
		cleaned[modelName] = true
	}
	returnValue, err := json.Marshal(cleaned)
	if err != nil {
		return "", err
	}
	return string(returnValue), nil
}

func decodeBoolMap(raw string) (map[string]bool, error) {
	if raw == "" {
		return map[string]bool{}, nil
	}
	var items map[string]bool
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return map[string]bool{}, err
	}
	if items == nil {
		items = map[string]bool{}
	}
	cleaned := make(map[string]bool, len(items))
	for key, value := range items {
		modelName := strings.TrimSpace(key)
		if modelName == "" || !value {
			continue
		}
		cleaned[modelName] = true
	}
	return cleaned, nil
}
