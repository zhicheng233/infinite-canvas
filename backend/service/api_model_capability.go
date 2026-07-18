package service

import (
	"encoding/json"
	"strings"

	"infinite-canvas-server/model"
)

func resolveGenerationByApiConfig(cfg *model.TenantApiConfig, modelName, fallback string) string {
	name := canonicalModelName(modelName)
	if cfg == nil || name == "" {
		return strings.TrimSpace(fallback)
	}
	for _, item := range decodeApiModelList(cfg.ImageModels) {
		if canonicalModelName(item) == name {
			return "image"
		}
	}
	for _, item := range decodeApiModelList(cfg.VideoModels) {
		if canonicalModelName(item) == name {
			return "video"
		}
	}
	for _, item := range decodeApiModelList(cfg.TextModels) {
		if canonicalModelName(item) == name {
			return "text"
		}
	}
	for _, item := range decodeApiModelList(cfg.AudioModels) {
		if canonicalModelName(item) == name {
			return "audio"
		}
	}
	return strings.TrimSpace(fallback)
}

func canonicalModelName(value string) string {
	name := strings.ToLower(strings.TrimSpace(value))
	name = strings.ReplaceAll(name, "-", "")
	name = strings.ReplaceAll(name, "_", "")
	return name
}

func decodeApiModelList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var items []string
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil
	}
	result := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		result = append(result, name)
	}
	return result
}
