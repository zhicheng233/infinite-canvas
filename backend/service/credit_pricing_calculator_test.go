package service

import (
	"encoding/json"
	"testing"

	"infinite-canvas-server/model"
)

func TestCalculateCreditCostUsesPerVideoUnit(t *testing.T) {
	body := []byte(`{"model":"grok-video","duration":10,"resolution":"1080p"}`)
	pricing := &model.CreditPricing{Model: "grok-video", CreditsPerUnit: 18, UnitType: model.UnitPerVideo}

	result, err := CalculateCreditCost(pricing, "video", "application/json", body)
	if err != nil {
		t.Fatalf("CalculateCreditCost returned error: %v", err)
	}
	if result.TotalCost != 18 || result.UnitCost != 18 || result.Units != 1 || result.UnitType != model.UnitPerVideo {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestCalculateCreditCostMultipliesImageCount(t *testing.T) {
	body := []byte(`{"model":"gpt-image-2","n":"3"}`)
	pricing := &model.CreditPricing{Model: "gpt-image-2", CreditsPerUnit: 2, UnitType: model.UnitPerImage}

	result, err := CalculateCreditCost(pricing, "image", "application/json", body)
	if err != nil {
		t.Fatalf("CalculateCreditCost returned error: %v", err)
	}
	if result.TotalCost != 6 || result.UnitCost != 2 || result.Units != 3 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestCalculateCreditCostUsesVideoDurationResolutionRule(t *testing.T) {
	rule := model.VideoPricingRule{
		BaseCredits: 3,
		ResolutionSecondRates: map[string]int{
			"720p":  1,
			"1080p": 2,
		},
	}
	raw, _ := json.Marshal(rule)
	body := []byte(`{"model":"veo-omni-flash","duration":10,"resolution":"1080p"}`)
	pricing := &model.CreditPricing{Model: "veo-omni-flash", CreditsPerUnit: 1, UnitType: model.UnitPerVideoSecond, PricingMode: model.PricingModeVideoDynamic, PricingRule: string(raw)}

	result, err := CalculateCreditCost(pricing, "video", "application/json", body)
	if err != nil {
		t.Fatalf("CalculateCreditCost returned error: %v", err)
	}
	if result.TotalCost != 23 || result.UnitCost != 2 || result.Units != 10 || result.Resolution != "1080p" || result.Seconds != 10 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.Formula != "基础 3 + 1080p 2 × 10秒" {
		t.Fatalf("formula = %q", result.Formula)
	}
}

func TestCalculateCreditCostAllowsDynamicVideoWithoutFallbackUnitCost(t *testing.T) {
	rule := model.VideoPricingRule{
		BaseCredits: 1,
		ResolutionSecondRates: map[string]int{
			"720p": 2,
		},
	}
	raw, _ := json.Marshal(rule)
	body := []byte(`{"model":"veo-omni-flash","duration":10,"resolution":"720p"}`)
	pricing := &model.CreditPricing{Model: "veo-omni-flash", CreditsPerUnit: 0, UnitType: model.UnitPerVideoSecond, PricingMode: model.PricingModeVideoDynamic, PricingRule: string(raw)}

	result, err := CalculateCreditCost(pricing, "video", "application/json", body)
	if err != nil {
		t.Fatalf("CalculateCreditCost returned error: %v", err)
	}
	if result.TotalCost != 21 || result.UnitCost != 2 || result.Units != 10 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestCalculateCreditCostNormalizesVideoSizeToResolution(t *testing.T) {
	rule := model.VideoPricingRule{
		BaseCredits: 0,
		ResolutionSecondRates: map[string]int{
			"720p":  1,
			"1080p": 2,
		},
	}
	raw, _ := json.Marshal(rule)
	body := []byte(`{"model":"sora_video2","seconds":"8","size":"1920x1080"}`)
	pricing := &model.CreditPricing{Model: "sora_video2", CreditsPerUnit: 1, UnitType: model.UnitPerVideoSecond, PricingMode: model.PricingModeVideoDynamic, PricingRule: string(raw)}

	result, err := CalculateCreditCost(pricing, "video", "application/json", body)
	if err != nil {
		t.Fatalf("CalculateCreditCost returned error: %v", err)
	}
	if result.TotalCost != 16 || result.Resolution != "1080p" || result.Seconds != 8 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestCalculateCreditCostRejectsMissingResolutionRate(t *testing.T) {
	rule := model.VideoPricingRule{ResolutionSecondRates: map[string]int{"720p": 1}}
	raw, _ := json.Marshal(rule)
	body := []byte(`{"model":"veo-omni-flash","duration":10,"resolution":"4K"}`)
	pricing := &model.CreditPricing{Model: "veo-omni-flash", CreditsPerUnit: 1, UnitType: model.UnitPerVideoSecond, PricingMode: model.PricingModeVideoDynamic, PricingRule: string(raw)}

	_, err := CalculateCreditCost(pricing, "video", "application/json", body)
	if err == nil || err.Error() != "模型 veo-omni-flash 未配置 4K 分辨率计费" {
		t.Fatalf("unexpected error: %v", err)
	}
}
