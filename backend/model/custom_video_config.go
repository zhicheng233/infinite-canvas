package model

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

const maxSafeJSONInteger int64 = 9_007_199_254_740_991

type CustomVideoConfig struct {
	Seconds           CustomVideoSecondsConfig       `json:"seconds"`
	Dimensions        CustomVideoDimensionsConfig    `json:"dimensions"`
	Images            CustomVideoMediaConfig         `json:"images"`
	InputReference    CustomVideoMediaConfig         `json:"input_reference"`
	StyleReferences   CustomVideoMediaConfig         `json:"style_references"`
	ElementReferences CustomVideoMediaConfig         `json:"element_references"`
	ReferenceImages   CustomVideoMediaConfig         `json:"reference_images"`
	ReferenceMode     CustomVideoReferenceModeConfig `json:"reference_mode"`
	InputVideo        CustomVideoMediaConfig         `json:"input_video"`
	Audio             CustomVideoAudioConfig         `json:"audio"`
	N                 CustomVideoNConfig             `json:"n"`
}

type CustomVideoSecondsConfig struct {
	Enabled bool   `json:"enabled"`
	Key     string `json:"key"`
	Mode    string `json:"mode"`
	Min     int    `json:"min,omitempty"`
	Max     int    `json:"max,omitempty"`
	Step    int    `json:"step,omitempty"`
	Options []int  `json:"options,omitempty"`
	Default int    `json:"default"`
}

type CustomVideoDimensionsConfig struct {
	Enabled bool     `json:"enabled"`
	Mode    string   `json:"mode"`
	Key     string   `json:"key"`
	Options []string `json:"options"`
	Default string   `json:"default"`
}

type CustomVideoMediaConfig struct {
	Enabled  bool   `json:"enabled"`
	Required bool   `json:"required"`
	Key      string `json:"key"`
	MaxCount int64  `json:"max_count"`
}

type CustomVideoReferenceModeConfig struct {
	Enabled bool     `json:"enabled"`
	Key     string   `json:"key"`
	Options []string `json:"options"`
	Default string   `json:"default"`
}

type CustomVideoAudioConfig struct {
	Enabled bool   `json:"enabled"`
	Key     string `json:"key"`
	Mode    string `json:"mode"`
	Value   bool   `json:"value"`
}

type CustomVideoNConfig struct {
	Enabled bool   `json:"enabled"`
	Key     string `json:"key"`
	Value   int    `json:"value"`
}

func NormalizeAndValidateCustomVideoConfig(config *CustomVideoConfig) error {
	if config == nil {
		return errors.New("video_custom_config 不能为空")
	}
	if err := normalizeSeconds(&config.Seconds); err != nil {
		return err
	}
	if err := normalizeDimensions(&config.Dimensions); err != nil {
		return err
	}
	media := []struct {
		name  string
		value *CustomVideoMediaConfig
	}{
		{"images", &config.Images},
		{"input_reference", &config.InputReference},
		{"style_references", &config.StyleReferences},
		{"element_references", &config.ElementReferences},
		{"reference_images", &config.ReferenceImages},
		{"input_video", &config.InputVideo},
	}
	for _, item := range media {
		item.value.Key = strings.TrimSpace(item.value.Key)
		if !item.value.Enabled {
			item.value.Required = false
		}
		if item.value.Enabled && (item.value.MaxCount < 1 || item.value.MaxCount > maxSafeJSONInteger) {
			return fmt.Errorf("%s.max_count 必须在 1-%d 之间", item.name, maxSafeJSONInteger)
		}
	}
	if err := normalizeReferenceMode(&config.ReferenceMode, config.ReferenceImages.Enabled); err != nil {
		return err
	}
	config.Audio.Key = strings.TrimSpace(config.Audio.Key)
	if config.Audio.Enabled && config.Audio.Mode != "fixed" && config.Audio.Mode != "user" {
		return errors.New("audio.mode 必须是 fixed 或 user")
	}
	config.N.Key = strings.TrimSpace(config.N.Key)
	if config.N.Enabled && (config.N.Value < 1 || config.N.Value > 16) {
		return errors.New("n.value 必须在 1-16 之间")
	}

	keys := []struct {
		name    string
		enabled bool
		key     string
	}{
		{"seconds", config.Seconds.Enabled, config.Seconds.Key},
		{"dimensions", config.Dimensions.Enabled, config.Dimensions.Key},
		{"images", config.Images.Enabled, config.Images.Key},
		{"input_reference", config.InputReference.Enabled, config.InputReference.Key},
		{"style_references", config.StyleReferences.Enabled, config.StyleReferences.Key},
		{"element_references", config.ElementReferences.Enabled, config.ElementReferences.Key},
		{"reference_images", config.ReferenceImages.Enabled, config.ReferenceImages.Key},
		{"reference_mode", config.ReferenceMode.Enabled, config.ReferenceMode.Key},
		{"input_video", config.InputVideo.Enabled, config.InputVideo.Key},
		{"audio", config.Audio.Enabled, config.Audio.Key},
		{"n", config.N.Enabled, config.N.Key},
	}
	seen := make(map[string]string, len(keys))
	for _, item := range keys {
		if !item.enabled {
			continue
		}
		if item.key == "" {
			return fmt.Errorf("%s.key 不能为空", item.name)
		}
		if item.key == "model" || item.key == "prompt" {
			return fmt.Errorf("%s.key 不能是 %s", item.name, item.key)
		}
		if previous, exists := seen[item.key]; exists {
			return fmt.Errorf("%s.key 与 %s.key 重复", item.name, previous)
		}
		seen[item.key] = item.name
	}
	return nil
}

func normalizeSeconds(config *CustomVideoSecondsConfig) error {
	config.Key = strings.TrimSpace(config.Key)
	if !config.Enabled {
		return nil
	}
	switch config.Mode {
	case "range":
		config.Options = nil
		if config.Min < 1 || config.Min > config.Default || config.Default > config.Max || config.Max > 3600 {
			return errors.New("seconds range 必须满足 1 <= min <= default <= max <= 3600")
		}
		if config.Step < 1 {
			return errors.New("seconds.step 必须是正整数")
		}
		if (config.Default-config.Min)%config.Step != 0 {
			return errors.New("seconds.default 必须在步长网格上")
		}
	case "options":
		config.Min, config.Max, config.Step = 0, 0, 0
		config.Options = uniqueSortedInts(config.Options)
		if len(config.Options) < 1 || len(config.Options) > 100 {
			return errors.New("seconds.options 必须包含 1-100 项")
		}
		for _, value := range config.Options {
			if value < 1 || value > 3600 {
				return errors.New("seconds.options 每项必须在 1-3600 之间")
			}
		}
		if !containsInt(config.Options, config.Default) {
			return errors.New("seconds.default 必须在 options 中")
		}
	default:
		return errors.New("seconds.mode 必须是 range 或 options")
	}
	return nil
}

func normalizeDimensions(config *CustomVideoDimensionsConfig) error {
	config.Key = strings.TrimSpace(config.Key)
	if !config.Enabled {
		return nil
	}
	if config.Mode != "size" && config.Mode != "aspect_ratio" {
		return errors.New("dimensions.mode 必须是 size 或 aspect_ratio")
	}
	options, err := uniqueSortedStrings(config.Options, nil)
	if err != nil {
		return errors.New("dimensions.options 不能包含空字符串")
	}
	config.Options = options
	config.Default = strings.TrimSpace(config.Default)
	if len(config.Options) < 1 || len(config.Options) > 50 {
		return errors.New("dimensions.options 必须包含 1-50 项")
	}
	if !containsString(config.Options, config.Default) {
		return errors.New("dimensions.default 必须在 options 中")
	}
	return nil
}

func normalizeReferenceMode(config *CustomVideoReferenceModeConfig, referenceImagesEnabled bool) error {
	config.Key = strings.TrimSpace(config.Key)
	if !config.Enabled {
		return nil
	}
	if !referenceImagesEnabled {
		return errors.New("reference_mode 仅可在 reference_images 启用时启用")
	}
	order := map[string]int{"frame": 0, "style": 1, "element": 2}
	options, err := uniqueSortedStrings(config.Options, order)
	if err != nil || len(options) < 1 {
		return errors.New("reference_mode.options 必须是 frame/style/element 的非空子集")
	}
	for _, option := range options {
		if _, exists := order[option]; !exists {
			return errors.New("reference_mode.options 必须是 frame/style/element 的非空子集")
		}
	}
	config.Options = options
	config.Default = strings.TrimSpace(config.Default)
	if !containsString(config.Options, config.Default) {
		return errors.New("reference_mode.default 必须在 options 中")
	}
	return nil
}

func uniqueSortedInts(values []int) []int {
	seen := make(map[int]struct{}, len(values))
	result := make([]int, 0, len(values))
	for _, value := range values {
		if _, exists := seen[value]; !exists {
			seen[value] = struct{}{}
			result = append(result, value)
		}
	}
	sort.Ints(result)
	return result
}

func uniqueSortedStrings(values []string, order map[string]int) ([]string, error) {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, errors.New("empty option")
		}
		if _, exists := seen[value]; !exists {
			seen[value] = struct{}{}
			result = append(result, value)
		}
	}
	if order == nil {
		sort.Strings(result)
	} else {
		sort.Slice(result, func(i, j int) bool { return order[result[i]] < order[result[j]] })
	}
	return result, nil
}

func containsInt(values []int, target int) bool {
	index := sort.SearchInts(values, target)
	return index < len(values) && values[index] == target
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
