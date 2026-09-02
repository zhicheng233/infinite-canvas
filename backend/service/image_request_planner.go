package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"infinite-canvas-server/model"
)

func validateResolvedRequestProtocol(route *channelRouteContext, capability, method, path, contentType string, body []byte) error {
	if capability != "image" || strings.ToUpper(strings.TrimSpace(method)) != http.MethodPost || route == nil || route.ChannelModel == nil {
		return nil
	}
	clean := cleanPath(path)
	payload := map[string]interface{}{}
	isJSON := strings.HasPrefix(strings.ToLower(strings.TrimSpace(contentType)), "application/json")
	if isJSON && len(body) > 0 && json.Unmarshal(body, &payload) != nil {
		return errors.New("图片请求 JSON 无效")
	}
	_, hasImage := payload["image"]
	isEdit := strings.HasSuffix(clean, "/images/edits") || hasImage
	configured := route.ChannelModel.ImageGenerateRoute
	if isEdit {
		configured = route.ChannelModel.ImageEditRoute
	}
	configured, err := normalizePlannedImageRoute(configured, isEdit, route.ChannelModel.ModelName, clean)
	if err != nil {
		return err
	}

	switch configured {
	case model.ImageRouteGenerations:
		if !strings.HasSuffix(clean, "/images/generations") || !isJSON {
			return errors.New("当前图片路由要求 POST /images/generations 并使用 JSON")
		}
		if hasImage && !validGenerationImageValue(payload["image"]) {
			return errors.New("generations 路由的 image 必须是字符串或非空字符串数组")
		}
	case model.ImageRouteEdits:
		if !strings.HasSuffix(clean, "/images/edits") || !strings.HasPrefix(strings.ToLower(contentType), "multipart/form-data") {
			return errors.New("当前图片编辑路由要求 POST /images/edits 并使用 multipart/form-data")
		}
	case model.ImageRouteChat, model.ImageRouteBanana:
		if !strings.HasSuffix(clean, "/chat/completions") || !isJSON {
			return fmt.Errorf("当前图片路由 %s 要求 POST /chat/completions 并使用 JSON", configured)
		}
	}
	return nil
}

func normalizePlannedImageRoute(configured string, isEdit bool, modelName, path string) (string, error) {
	normalized, err := model.NormalizeImageGenerateRoute(configured)
	if isEdit {
		normalized, err = model.NormalizeImageEditRoute(configured)
	}
	if err != nil {
		return "", err
	}
	if normalized != model.ImageRouteAuto {
		return normalized, nil
	}
	switch {
	case strings.HasSuffix(path, "/images/edits"):
		return model.ImageRouteEdits, nil
	case strings.HasSuffix(path, "/chat/completions") && isBananaModelName(modelName):
		return model.ImageRouteBanana, nil
	case strings.HasSuffix(path, "/chat/completions"):
		return model.ImageRouteChat, nil
	default:
		return model.ImageRouteGenerations, nil
	}
}

func validGenerationImageValue(value interface{}) bool {
	if item, ok := value.(string); ok {
		return strings.TrimSpace(item) != ""
	}
	items, ok := value.([]interface{})
	if !ok || len(items) == 0 {
		return false
	}
	for _, item := range items {
		text, ok := item.(string)
		if !ok || strings.TrimSpace(text) == "" {
			return false
		}
	}
	return true
}
