package service

import "infinite-canvas-server/model"

const serviceTestMaxSafeJSONInteger int64 = (1 << 53) - 1

func serviceTestCustomVideoConfig() *model.CustomVideoConfig {
	return &model.CustomVideoConfig{
		Seconds:    model.CustomVideoSecondsConfig{Enabled: true, Key: "seconds", Mode: "range", Min: 3, Max: 10, Step: 1, Default: 6},
		Dimensions: model.CustomVideoDimensionsConfig{Enabled: true, Mode: "size", Key: "size", Options: []string{"1280x720", "720x1280"}, Default: "1280x720"},
		Images:     model.CustomVideoMediaConfig{Enabled: true, Required: false, Key: "images", MaxCount: 1},
		InputVideo: model.CustomVideoMediaConfig{Enabled: true, Required: true, Key: "input_video", MaxCount: 1},
		N:          model.CustomVideoNConfig{Enabled: true, Key: "n", Value: 1},
	}
}

type serviceTestMediaRole struct {
	name           string
	maxCount       int64
	legacyMaxCount int64
	selectRole     func(*model.CustomVideoConfig) *model.CustomVideoMediaConfig
}

var serviceTestMediaRoles = []serviceTestMediaRole{
	{name: "images", maxCount: 2, legacyMaxCount: 1, selectRole: func(config *model.CustomVideoConfig) *model.CustomVideoMediaConfig { return &config.Images }},
	{name: "input_reference", maxCount: 2, legacyMaxCount: 1, selectRole: func(config *model.CustomVideoConfig) *model.CustomVideoMediaConfig { return &config.InputReference }},
	{name: "style_references", maxCount: 5, legacyMaxCount: 4, selectRole: func(config *model.CustomVideoConfig) *model.CustomVideoMediaConfig { return &config.StyleReferences }},
	{name: "element_references", maxCount: 4, legacyMaxCount: 3, selectRole: func(config *model.CustomVideoConfig) *model.CustomVideoMediaConfig { return &config.ElementReferences }},
	{name: "reference_images", maxCount: 5, legacyMaxCount: 4, selectRole: func(config *model.CustomVideoConfig) *model.CustomVideoMediaConfig { return &config.ReferenceImages }},
	{name: "input_video", maxCount: 2, legacyMaxCount: 1, selectRole: func(config *model.CustomVideoConfig) *model.CustomVideoMediaConfig { return &config.InputVideo }},
}

func serviceTestAboveFormerCapCustomVideoConfig() *model.CustomVideoConfig {
	config := serviceTestCustomVideoConfig()
	for _, media := range serviceTestMediaRoles {
		role := media.selectRole(config)
		role.Enabled = true
		role.Key = media.name
		role.MaxCount = media.maxCount
	}
	return config
}

func serviceTestFormerCapCustomVideoConfig() *model.CustomVideoConfig {
	config := serviceTestCustomVideoConfig()
	for _, media := range serviceTestMediaRoles {
		role := media.selectRole(config)
		role.Enabled = true
		role.Key = media.name
		role.MaxCount = media.legacyMaxCount
	}
	return config
}

func serviceTestMediaMaxCounts(config *model.CustomVideoConfig) map[string]int64 {
	counts := make(map[string]int64, len(serviceTestMediaRoles))
	for _, media := range serviceTestMediaRoles {
		counts[media.name] = media.selectRole(config).MaxCount
	}
	return counts
}

func serviceTestExpectedAboveFormerCapMediaCounts() map[string]int64 {
	return map[string]int64{
		"images":             2,
		"input_reference":    2,
		"style_references":   5,
		"element_references": 4,
		"reference_images":   5,
		"input_video":        2,
	}
}

func serviceTestExpectedFormerCapMediaCounts() map[string]int64 {
	return map[string]int64{
		"images":             1,
		"input_reference":    1,
		"style_references":   4,
		"element_references": 3,
		"reference_images":   4,
		"input_video":        1,
	}
}
