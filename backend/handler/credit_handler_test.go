package handler

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"infinite-canvas-server/model"
	"infinite-canvas-server/service"
)

type fakeEstimatePricingReader struct {
	items     map[string]map[uint]*model.CreditPricing
	seenModel string
	seen      uint
}

func (f *fakeEstimatePricingReader) FindPricing(_ uint, modelName string, channelID uint) (*model.CreditPricing, error) {
	f.seenModel = modelName
	f.seen = channelID
	return f.items[modelName][channelID], nil
}

type fakeEstimateRouteResolver struct {
	resolved service.ResolvedEstimateRoute
}

func (f fakeEstimateRouteResolver) ResolveChannelRouteForEstimate(_ uint, _ service.ChannelSelection, _, _, _ string) (service.ResolvedEstimateRoute, error) {
	return f.resolved, nil
}

func estimateContext(target string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("GET", target, nil)
	ctx.Set("claims", &service.Claims{TenantID: 1, UserID: 2})
	return ctx, recorder
}

func TestEstimateCostPricesResolvedChannelAndPrefersExactPricing(t *testing.T) {
	pricing := &fakeEstimatePricingReader{items: map[string]map[uint]*model.CreditPricing{
		"same-model": {
			0: {Model: "same-model", CreditsPerUnit: 2, UnitType: model.UnitPerImage},
			2: {Model: "same-model", ChannelID: 2, CreditsPerUnit: 7, UnitType: model.UnitPerImage},
		},
	}}
	handler := &CreditHandler{estimatePricingRepo: pricing, estimateRouteResolver: fakeEstimateRouteResolver{resolved: service.ResolvedEstimateRoute{Selection: service.ChannelSelection{ChannelID: 2, ChannelModelID: 22}, PricingModel: "same-model"}}}
	ctx, recorder := estimateContext("/credits/estimate?model=same-model&type=image&count=2&channel_id=0")

	handler.EstimateCost(ctx)

	var response struct {
		Code int `json:"code"`
		Data struct {
			TotalCost int `json:"total_cost"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Code != 0 || response.Data.TotalCost != 14 || pricing.seen != 2 || pricing.seenModel != "same-model" {
		t.Fatalf("unexpected response=%s pricing=%s/%d", recorder.Body.String(), pricing.seenModel, pricing.seen)
	}
}

func TestEstimateCostPricesResolvedMergeModelInsteadOfAlias(t *testing.T) {
	pricing := &fakeEstimatePricingReader{items: map[string]map[uint]*model.CreditPricing{
		"text-model": {1: {Model: "text-model", ChannelID: 1, CreditsPerUnit: 4, UnitType: model.UnitPerToken}},
	}}
	handler := &CreditHandler{
		estimatePricingRepo: pricing,
		estimateRouteResolver: fakeEstimateRouteResolver{resolved: service.ResolvedEstimateRoute{
			Selection:    service.ChannelSelection{ChannelID: 1, ChannelModelID: 44},
			PricingModel: "text-model",
		}},
	}
	ctx, recorder := estimateContext("/credits/estimate?model=gpt-4o&type=text&channel_id=1&fuzzy_group_name=gpt-4o")

	handler.EstimateCost(ctx)

	var response model.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Code != 0 || pricing.seenModel != "text-model" || pricing.seen != 1 {
		t.Fatalf("unexpected response=%s pricing=%s/%d", recorder.Body.String(), pricing.seenModel, pricing.seen)
	}
}

func TestEstimateCostMissingPricingReturnsStructuredError(t *testing.T) {
	handler := &CreditHandler{
		estimatePricingRepo: &fakeEstimatePricingReader{items: map[string]map[uint]*model.CreditPricing{}},
		estimateRouteResolver: fakeEstimateRouteResolver{resolved: service.ResolvedEstimateRoute{
			Selection: service.ChannelSelection{ChannelID: 2, ChannelModelID: 22}, PricingModel: "same-model",
		}},
	}
	ctx, recorder := estimateContext("/credits/estimate?model=same-model&type=image&channel_id=2&channel_model_id=22")

	handler.EstimateCost(ctx)

	var response model.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Code != 403 || response.Msg != "该模型未配置计费，暂不可用" {
		t.Fatalf("unexpected response: %s", recorder.Body.String())
	}
}

func TestEstimateCostRejectsMalformedOrPartialRouteIdentity(t *testing.T) {
	tests := []string{
		"/credits/estimate?model=same-model&type=image",
		"/credits/estimate?model=same-model&type=image&channel_id=abc&channel_model_id=22",
		"/credits/estimate?model=same-model&type=image&channel_id=-1&channel_model_id=22",
		"/credits/estimate?model=same-model&type=image&channel_id=2",
		"/credits/estimate?model=same-model&type=image&channel_id=0&channel_model_id=22",
		"/credits/estimate?model=gpt-4o&type=text&channel_id=0&fuzzy_group_name=gpt-4o",
	}
	for _, target := range tests {
		t.Run(target, func(t *testing.T) {
			handler := &CreditHandler{}
			ctx, recorder := estimateContext(target)

			handler.EstimateCost(ctx)

			var response model.Response
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if response.Code != 400 {
				t.Fatalf("expected route validation failure, got %s", recorder.Body.String())
			}
		})
	}
}
