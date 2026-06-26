package handler

import (
	"encoding/json"
	"strings"

	"github.com/gin-gonic/gin"
	"infinite-canvas-server/crypto"
	"infinite-canvas-server/config"
	"infinite-canvas-server/model"
	"infinite-canvas-server/repository"
	"infinite-canvas-server/service"
)

type ApiConfigHandler struct {
	apiConfigRepo *repository.ApiConfigRepo
	creditRepo    *repository.CreditRepo
	cfg           *config.Config
}

func NewApiConfigHandler(apiConfigRepo *repository.ApiConfigRepo, creditRepo *repository.CreditRepo, cfg *config.Config) *ApiConfigHandler {
	return &ApiConfigHandler{apiConfigRepo: apiConfigRepo, creditRepo: creditRepo, cfg: cfg}
}

type SaveApiConfigInput struct {
	BaseUrl     string            `json:"base_url"`
	ApiKey      string            `json:"api_key"`
	Models      []string          `json:"models"`
	ImageModels []string          `json:"image_models"`
	VideoModels []string          `json:"video_models"`
	TextModels  []string          `json:"text_models"`
	AudioModels []string          `json:"audio_models"`
	ModelRoutes map[string]string `json:"model_routes"`
}

func (h *ApiConfigHandler) Get(c *gin.Context) {
	claims := c.MustGet("claims").(*service.Claims)
	cfg, err := h.apiConfigRepo.FindByTenant(claims.TenantID)
	if err != nil {
		model.Fail(c, 404, "未配置 API")
		return
	}
	models, _ := decodeStringList(cfg.Models)
	imageModels, _ := decodeStringList(cfg.ImageModels)
	videoModels, _ := decodeStringList(cfg.VideoModels)
	textModels, _ := decodeStringList(cfg.TextModels)
	audioModels, _ := decodeStringList(cfg.AudioModels)
	modelRoutes, _ := decodeStringMap(cfg.ModelRoutes)
	model.OK(c, gin.H{
		"base_url":     cfg.BaseUrl,
		"has_key":      len(cfg.ApiKey) > 0,
		"models":       models,
		"image_models": imageModels,
		"video_models": videoModels,
		"text_models":  textModels,
		"audio_models": audioModels,
		"model_routes": modelRoutes,
	})
}

func (h *ApiConfigHandler) Catalog(c *gin.Context) {
	claims := c.MustGet("claims").(*service.Claims)
	cfg, err := h.apiConfigRepo.FindByTenant(claims.TenantID)
	if err != nil {
		model.Fail(c, 404, "未配置 API")
		return
	}

	models, _ := decodeStringList(cfg.Models)
	imageModels, _ := decodeStringList(cfg.ImageModels)
	videoModels, _ := decodeStringList(cfg.VideoModels)
	textModels, _ := decodeStringList(cfg.TextModels)
	audioModels, _ := decodeStringList(cfg.AudioModels)
	modelRoutes, _ := decodeStringMap(cfg.ModelRoutes)
	pricingMap, err := h.creditRepo.FindPricingMap(claims.TenantID)
	if err != nil {
		model.Fail(c, 500, "读取定价配置失败")
		return
	}

	enabledModels := filterModelsByPricing(models, pricingMap)
	model.OK(c, gin.H{
		"models":          enabledModels,
		"image_models":    filterModelsByPricing(imageModels, pricingMap),
		"video_models":    filterModelsByPricing(videoModels, pricingMap),
		"text_models":     filterModelsByPricing(textModels, pricingMap),
		"audio_models":    filterModelsByPricing(audioModels, pricingMap),
		"priced_models":   enabledModels,
		"pricing_map":     pricingMap,
		"model_routes":    modelRoutes,
		"total_models":    len(models),
		"enabled_count":   len(enabledModels),
		"disabled_models": collectDisabledModels(models, pricingMap),
	})
}

func (h *ApiConfigHandler) Save(c *gin.Context) {
	claims := c.MustGet("claims").(*service.Claims)
	var input SaveApiConfigInput
	if err := c.ShouldBindJSON(&input); err != nil {
		model.Fail(c, 400, "无效的请求参数")
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

	existingCfg, _ := h.apiConfigRepo.FindByTenant(claims.TenantID)
	encryptedKey := ""
	if strings.TrimSpace(input.ApiKey) == "" {
		if existingCfg == nil || strings.TrimSpace(existingCfg.ApiKey) == "" {
			model.Fail(c, 400, "请填写 API Key")
			return
		}
		encryptedKey = existingCfg.ApiKey
	} else {
		var err error
		encryptedKey, err = crypto.Encrypt(h.cfg.ApiKeyEncryptKey, input.ApiKey)
		if err != nil {
			model.Fail(c, 500, "加密 API Key 失败")
			return
		}
	}

	cfg := &model.TenantApiConfig{
		TenantID:    claims.TenantID,
		BaseUrl:     input.BaseUrl,
		ApiKey:      encryptedKey,
		Models:      models,
		ImageModels: imageModels,
		VideoModels: videoModels,
		TextModels:  textModels,
		AudioModels: audioModels,
		ModelRoutes: modelRoutes,
	}
	if err := h.apiConfigRepo.Save(cfg); err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.OK(c, gin.H{"saved": true})
}

func filterModelsByPricing(models []string, pricingMap map[string]model.CreditPricing) []string {
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
		if _, exists := pricingMap[name]; !exists {
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

func collectDisabledModels(models []string, pricingMap map[string]model.CreditPricing) []string {
	items := make([]string, 0)
	seen := make(map[string]struct{})
	for _, item := range models {
		name := strings.TrimSpace(item)
		if name == "" {
			continue
		}
		if _, ok := pricingMap[name]; ok {
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
