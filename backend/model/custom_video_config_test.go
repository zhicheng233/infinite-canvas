package model

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func validCustomVideoConfig() CustomVideoConfig {
	return CustomVideoConfig{
		Seconds:           CustomVideoSecondsConfig{Enabled: true, Key: " seconds ", Mode: "range", Min: 3, Max: 10, Step: 1, Default: 6},
		Dimensions:        CustomVideoDimensionsConfig{Enabled: true, Mode: "size", Key: " size ", Options: []string{"720x1280", "1280x720", "1280x720"}, Default: "1280x720"},
		Images:            CustomVideoMediaConfig{Enabled: true, Key: "images", MaxCount: 1},
		InputReference:    CustomVideoMediaConfig{Enabled: false, Key: "input_reference", MaxCount: 1},
		StyleReferences:   CustomVideoMediaConfig{Enabled: true, Key: "style_references", MaxCount: 4},
		ElementReferences: CustomVideoMediaConfig{Enabled: true, Key: "element_references", MaxCount: 3},
		ReferenceImages:   CustomVideoMediaConfig{Enabled: true, Key: "reference_images", MaxCount: 1},
		ReferenceMode:     CustomVideoReferenceModeConfig{Enabled: true, Key: "reference_mode", Options: []string{"element", "frame", "style", "style"}, Default: "element"},
		InputVideo:        CustomVideoMediaConfig{Enabled: true, Key: "input_video", MaxCount: 1},
		Audio:             CustomVideoAudioConfig{Enabled: false, Key: "audio", Mode: "fixed", Value: false},
		N:                 CustomVideoNConfig{Enabled: true, Key: "n", Value: 1},
	}
}

func TestNormalizeAndValidateCustomVideoConfigCanonicalizesMediaRequired(t *testing.T) {
	tests := []struct {
		name       string
		selectRole func(*CustomVideoConfig) *CustomVideoMediaConfig
	}{
		{"images", func(config *CustomVideoConfig) *CustomVideoMediaConfig { return &config.Images }},
		{"input_reference", func(config *CustomVideoConfig) *CustomVideoMediaConfig { return &config.InputReference }},
		{"style_references", func(config *CustomVideoConfig) *CustomVideoMediaConfig { return &config.StyleReferences }},
		{"element_references", func(config *CustomVideoConfig) *CustomVideoMediaConfig { return &config.ElementReferences }},
		{"reference_images", func(config *CustomVideoConfig) *CustomVideoMediaConfig { return &config.ReferenceImages }},
		{"input_video", func(config *CustomVideoConfig) *CustomVideoMediaConfig { return &config.InputVideo }},
	}
	semantics := []struct {
		name         string
		enabled      bool
		required     bool
		wantRequired bool
	}{
		{"disabled", false, true, false},
		{"optional", true, false, false},
		{"required", true, true, true},
	}
	for _, test := range tests {
		for _, semantic := range semantics {
			t.Run(test.name+"/"+semantic.name, func(t *testing.T) {
				config := validCustomVideoConfig()
				role := test.selectRole(&config)
				role.Enabled = semantic.enabled
				role.Required = semantic.required
				if test.name == "reference_images" && !semantic.enabled {
					config.ReferenceMode.Enabled = false
				}

				if err := NormalizeAndValidateCustomVideoConfig(&config); err != nil {
					t.Fatalf("normalize %s %s: %v", test.name, semantic.name, err)
				}
				if role.Required != semantic.wantRequired {
					t.Fatalf("required=%t, want %t", role.Required, semantic.wantRequired)
				}
			})
		}
	}
}

func TestCustomVideoConfigJSONRoundTripPreservesMediaRequired(t *testing.T) {
	config := validCustomVideoConfig()
	config.Images.Required = false
	config.InputReference.Enabled = true
	config.InputReference.Required = true
	config.StyleReferences.Required = false
	config.ElementReferences.Required = true
	config.ReferenceImages.Required = false
	config.InputVideo.Required = true
	if err := NormalizeAndValidateCustomVideoConfig(&config); err != nil {
		t.Fatalf("normalize fixture: %v", err)
	}

	payload, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	var decoded CustomVideoConfig
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	if err := NormalizeAndValidateCustomVideoConfig(&decoded); err != nil {
		t.Fatalf("normalize decoded config: %v", err)
	}
	want := []bool{false, true, false, true, false, true}
	got := []bool{decoded.Images.Required, decoded.InputReference.Required, decoded.StyleReferences.Required, decoded.ElementReferences.Required, decoded.ReferenceImages.Required, decoded.InputVideo.Required}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("required values=%v, want %v; JSON=%s", got, want, payload)
	}
}

func TestNormalizeAndValidateCustomVideoConfig(t *testing.T) {
	config := validCustomVideoConfig()
	if err := NormalizeAndValidateCustomVideoConfig(&config); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	if config.Seconds.Key != "seconds" || config.Dimensions.Key != "size" {
		t.Fatalf("keys were not trimmed: seconds=%q dimensions=%q", config.Seconds.Key, config.Dimensions.Key)
	}
	if want := []string{"1280x720", "720x1280"}; !reflect.DeepEqual(config.Dimensions.Options, want) {
		t.Fatalf("dimensions options=%v, want %v", config.Dimensions.Options, want)
	}
	if want := []string{"frame", "style", "element"}; !reflect.DeepEqual(config.ReferenceMode.Options, want) {
		t.Fatalf("reference mode options=%v, want %v", config.ReferenceMode.Options, want)
	}
}

func TestNormalizeAndValidateCustomVideoConfigRejectsInvalidCatalog(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*CustomVideoConfig)
		message string
	}{
		{
			name: "duplicate alias",
			mutate: func(config *CustomVideoConfig) {
				config.Dimensions.Key = " seconds "
			},
			message: "重复",
		},
		{
			name: "forbidden model alias",
			mutate: func(config *CustomVideoConfig) {
				config.Seconds.Key = " model "
			},
			message: "不能是 model",
		},
		{
			name: "empty options",
			mutate: func(config *CustomVideoConfig) {
				config.Dimensions.Options = nil
			},
			message: "1-50",
		},
		{
			name: "invalid default",
			mutate: func(config *CustomVideoConfig) {
				config.Dimensions.Default = "1920x1080"
			},
			message: "default",
		},
		{
			name: "reference mode without reference images",
			mutate: func(config *CustomVideoConfig) {
				config.ReferenceImages.Enabled = false
			},
			message: "reference_images",
		},
		{
			name: "media above hard cap",
			mutate: func(config *CustomVideoConfig) {
				config.StyleReferences.MaxCount = 5
			},
			message: "1-4",
		},
		{
			name: "forbidden prompt alias",
			mutate: func(config *CustomVideoConfig) {
				config.N.Key = "prompt"
			},
			message: "不能是 prompt",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := validCustomVideoConfig()
			test.mutate(&config)
			err := NormalizeAndValidateCustomVideoConfig(&config)
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("error=%v, want message containing %q", err, test.message)
			}
		})
	}
}
