package model

import (
	"fmt"
	"strings"
)

const (
	ImageRouteAuto        = "auto"
	ImageRouteGenerations = "generations"
	ImageRouteEdits       = "edits"
	ImageRouteChat        = "chat"
	ImageRouteBanana      = "banana"
)

var videoRouteValues = map[string]struct{}{
	"auto": {}, "openai": {}, "veo_json": {}, "waninter": {}, "yijia": {}, "xai": {}, "newapi": {}, "seedance": {}, "binghuo": {}, "custom": {},
}

func NormalizeImageGenerateRoute(value string) (string, error) {
	return normalizeRoute(value, map[string]struct{}{ImageRouteAuto: {}, ImageRouteGenerations: {}, ImageRouteChat: {}, ImageRouteBanana: {}}, "图片生成")
}

func NormalizeImageEditRoute(value string) (string, error) {
	return normalizeRoute(value, map[string]struct{}{ImageRouteAuto: {}, ImageRouteGenerations: {}, ImageRouteEdits: {}, ImageRouteChat: {}, ImageRouteBanana: {}}, "图片编辑")
}

func NormalizeVideoRoute(value string) (string, error) {
	return normalizeRoute(value, videoRouteValues, "视频")
}

func normalizeRoute(value string, allowed map[string]struct{}, label string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		normalized = "auto"
	}
	if _, ok := allowed[normalized]; !ok {
		return "", fmt.Errorf("不支持的%s路由：%s", label, value)
	}
	return normalized, nil
}
