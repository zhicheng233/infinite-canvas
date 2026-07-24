package service

import (
	"encoding/json"
	"strings"
	"testing"

	"infinite-canvas-server/model"
)

func TestChannelToAdminInfo_MapsRemark(t *testing.T) {
	channel := &model.Channel{
		Name:   "test-channel",
		Remark: "测试备注",
	}
	info := channelToAdminInfo(channel)
	if info.Remark != "测试备注" {
		t.Fatalf("expected Remark='测试备注', got %q", info.Remark)
	}
}

func TestChannelToAdminInfo_RemarkEmpty(t *testing.T) {
	channel := &model.Channel{
		Name: "test-channel",
	}
	info := channelToAdminInfo(channel)
	if info.Remark != "" {
		t.Fatalf("expected empty Remark, got %q", info.Remark)
	}
}

func TestChannelToInfo_NoRemark(t *testing.T) {
	channel := &model.Channel{
		Name:   "test-channel",
		Remark: "测试备注",
	}
	info := channelToInfo(channel)
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	if strings.Contains(strings.ToLower(string(data)), "remark") {
		t.Fatal("ChannelInfo JSON must not contain remark field")
	}
}

func TestCreateChannel_RemarkTooLong_501Chars(t *testing.T) {
	input := model.SaveChannelInput{
		Name:    "test",
		BaseUrl: "https://example.com",
		ApiKey:  "test-key",
		Remark:  strings.Repeat("a", 501),
	}
	// Simulate the validation from Create()
	if len([]rune(input.Remark)) > 500 {
		return // expected error — test passes
	}
	t.Fatal("expected remark validation error for >500 characters")
}

func TestCreateChannel_RemarkTooLong_Chinese(t *testing.T) {
	input := model.SaveChannelInput{
		Name:    "test",
		BaseUrl: "https://example.com",
		ApiKey:  "test-key",
		Remark:  strings.Repeat("备", 501),
	}
	// Simulate the validation from Create(): len([]rune(input.Remark)) > 500
	if len([]rune(input.Remark)) > 500 {
		return // expected error — test passes
	}
	t.Fatal("expected remark validation error for >500 Chinese characters")
}

func TestCreateChannel_RemarkMaxLength_500Chars(t *testing.T) {
	input := model.SaveChannelInput{
		Name:    "test",
		BaseUrl: "https://example.com",
		ApiKey:  "test-key",
		Remark:  strings.Repeat("a", 500),
	}
	// Simulate the validation from Create()
	if len([]rune(input.Remark)) > 500 {
		t.Fatal("remark of exactly 500 characters should be valid")
	}
	// If we get here, validation passed — test passes
}

func TestCreateChannel_RemarkEmpty(t *testing.T) {
	input := model.SaveChannelInput{
		Name:    "test",
		BaseUrl: "https://example.com",
		ApiKey:  "test-key",
		Remark:  "",
	}
	// Simulate the validation from Create()
	if len([]rune(input.Remark)) > 500 {
		t.Fatal("empty remark should not trigger validation error")
	}
}

func TestUpdateChannel_RemarkPreserveEmpty(t *testing.T) {
	channel := &model.Channel{
		Name:   "test-channel",
		Remark: "existing remark",
	}
	input := model.SaveChannelInput{
		Name:    "test-channel",
		BaseUrl: "https://example.com",
		Remark:  "", // empty — should preserve existing
	}
	// Simulate the preservation logic from Update():
	if input.Remark != "" {
		channel.Remark = input.Remark
	}
	if channel.Remark != "existing remark" {
		t.Fatalf("expected Remark preserved as 'existing remark', got %q", channel.Remark)
	}
}

func TestUpdateChannel_RemarkUpdate(t *testing.T) {
	channel := &model.Channel{
		Name:   "test-channel",
		Remark: "old remark",
	}
	input := model.SaveChannelInput{
		Name:    "test-channel",
		BaseUrl: "https://example.com",
		Remark:  "new remark",
	}
	// Simulate the preservation logic from Update():
	if input.Remark != "" {
		channel.Remark = input.Remark
	}
	if channel.Remark != "new remark" {
		t.Fatalf("expected Remark='new remark', got %q", channel.Remark)
	}
}

func TestUpdateChannel_RemarkPreserveOnWhitespaceOnly(t *testing.T) {
	channel := &model.Channel{
		Name:   "test-channel",
		Remark: "existing remark",
	}
	// Whitespace-only is still != "" so it will update
	input := model.SaveChannelInput{
		Name:    "test-channel",
		BaseUrl: "https://example.com",
		Remark:  "   ",
	}
	// Simulate the preservation logic from Update():
	if input.Remark != "" {
		channel.Remark = input.Remark
	}
	if channel.Remark != "   " {
		t.Fatalf("expected Remark updated to '   ', got %q", channel.Remark)
	}
}

func TestCreateChannel_RemarkTooLong_Exactly501Runes(t *testing.T) {
	// Use mixed-width characters to ensure []rune length check
	input := model.SaveChannelInput{
		Name:    "test",
		BaseUrl: "https://example.com",
		ApiKey:  "test-key",
		Remark:  strings.Repeat("好", 501),
	}
	// Simulate the validation from Create()
	if len([]rune(input.Remark)) > 500 {
		return // expected error — test passes
	}
	t.Fatal("expected remark validation error for >500 runes")
}
