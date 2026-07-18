package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"mime/multipart"
	"strconv"
	"strings"

	"infinite-canvas-server/model"
)

type CreditCostResult struct {
	TotalCost  int
	UnitCost   int
	Units      int
	UnitType   model.CreditPricingUnit
	Seconds    int
	Resolution string
	Formula    string
}

func CalculateCreditCost(pricing *model.CreditPricing, genType, contentType string, body []byte) (CreditCostResult, error) {
	if pricing == nil {
		return CreditCostResult{}, fmt.Errorf("模型未配置有效计费")
	}
	if pricing.PricingMode == model.PricingModeVideoDynamic || pricing.UnitType == model.UnitPerVideoSecond {
		return calculateVideoDynamicCost(pricing, contentType, body)
	}
	if pricing.CreditsPerUnit <= 0 {
		return CreditCostResult{}, fmt.Errorf("模型未配置有效计费")
	}
	units := extractUsageCount(genType, contentType, body)
	if units <= 0 {
		units = 1
	}
	total := pricing.CreditsPerUnit
	if pricing.UnitType == model.UnitPerImage {
		total *= units
	}
	return CreditCostResult{
		TotalCost: total,
		UnitCost:  pricing.CreditsPerUnit,
		Units:     units,
		UnitType:  pricing.UnitType,
	}, nil
}

func calculateVideoDynamicCost(pricing *model.CreditPricing, contentType string, body []byte) (CreditCostResult, error) {
	var rule model.VideoPricingRule
	if strings.TrimSpace(pricing.PricingRule) != "" {
		if err := json.Unmarshal([]byte(pricing.PricingRule), &rule); err != nil {
			return CreditCostResult{}, fmt.Errorf("模型 %s 视频计费规则格式错误", pricing.Model)
		}
	}
	seconds := extractVideoSeconds(contentType, body)
	if seconds <= 0 {
		seconds = 1
	}
	resolution := extractVideoResolution(contentType, body)
	if resolution == "" {
		resolution = "720p"
	}
	rate := rule.ResolutionSecondRates[resolution]
	if rate <= 0 {
		return CreditCostResult{}, fmt.Errorf("模型 %s 未配置 %s 分辨率计费", pricing.Model, resolution)
	}
	total := rule.BaseCredits + rate*seconds
	formula := fmt.Sprintf("基础 %d + %s %d × %d秒", rule.BaseCredits, resolution, rate, seconds)
	return CreditCostResult{
		TotalCost:  total,
		UnitCost:   rate,
		Units:      seconds,
		UnitType:   model.UnitPerVideoSecond,
		Seconds:    seconds,
		Resolution: resolution,
		Formula:    formula,
	}, nil
}

func extractVideoSeconds(contentType string, body []byte) int {
	values := extractRequestFields(contentType, body)
	for _, key := range []string{"seconds", "duration", "videoSeconds"} {
		if value := intFromAny(values[key]); value > 0 {
			return value
		}
	}
	return 0
}

func extractVideoResolution(contentType string, body []byte) string {
	values := extractRequestFields(contentType, body)
	for _, key := range []string{"resolution", "resolution_name", "vquality", "quality"} {
		if value := normalizeResolutionLabel(stringFromAny(values[key])); value != "" {
			return value
		}
	}
	if value := normalizeResolutionFromSize(stringFromAny(values["size"])); value != "" {
		return value
	}
	return ""
}

func extractRequestFields(contentType string, body []byte) map[string]interface{} {
	values := map[string]interface{}{}
	if strings.HasPrefix(contentType, "application/json") {
		_ = json.Unmarshal(body, &values)
		return values
	}
	if strings.HasPrefix(contentType, "multipart/form-data") {
		boundary := extractBoundary(contentType)
		if boundary == "" {
			return values
		}
		reader := multipart.NewReader(bytes.NewReader(body), boundary)
		form, err := reader.ReadForm(32 << 20)
		if err != nil {
			return values
		}
		for key, items := range form.Value {
			if len(items) > 0 {
				values[key] = items[0]
			}
		}
		return values
	}
	return values
}

func intFromAny(value interface{}) int {
	switch typed := value.(type) {
	case float64:
		return int(math.Ceil(typed))
	case int:
		return typed
	case string:
		number, _ := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return int(math.Ceil(number))
	default:
		return 0
	}
}

func stringFromAny(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case float64:
		return strconv.Itoa(int(typed))
	case int:
		return strconv.Itoa(typed)
	default:
		return ""
	}
}

func normalizeResolutionLabel(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" || value == "auto" || value == "medium" || value == "high" {
		return ""
	}
	if value == "low" {
		return "480p"
	}
	value = strings.TrimSuffix(value, "p")
	switch value {
	case "480", "720", "1080":
		return value + "p"
	case "2k":
		return "2K"
	case "4k":
		return "4K"
	default:
		if n, err := strconv.Atoi(value); err == nil && n > 0 {
			return fmt.Sprintf("%dp", n)
		}
		return strings.ToUpper(value)
	}
}

func normalizeResolutionFromSize(value string) string {
	match := strings.Split(strings.ToLower(strings.TrimSpace(value)), "x")
	if len(match) != 2 {
		return ""
	}
	width, _ := strconv.Atoi(match[0])
	height, _ := strconv.Atoi(match[1])
	longSide := width
	if height > longSide {
		longSide = height
	}
	switch {
	case longSide >= 3840:
		return "4K"
	case longSide >= 2560:
		return "2K"
	case longSide >= 1920:
		return "1080p"
	case longSide >= 1280:
		return "720p"
	case longSide > 0:
		return "480p"
	default:
		return ""
	}
}
