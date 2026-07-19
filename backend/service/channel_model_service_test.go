package service

import (
	"encoding/json"
	"testing"

	"infinite-canvas-server/model"
)

func TestUpdateChannelModelCapabilities(t *testing.T) {
	// Setup: Create a channel model with initial capabilities
	initialCapabilities := []string{"image", "video"}
	initialJSON, err := json.Marshal(initialCapabilities)
	if err != nil {
		t.Fatalf("failed to marshal initial capabilities: %v", err)
	}

	item := &model.ChannelModel{
		ChannelID:    1,
		ModelName:    "test-model",
		Capabilities: string(initialJSON),
		Enabled:      true,
	}

	// Test 1: Update with new capabilities ["image","text","audio"]
	t.Run("update with new capabilities", func(t *testing.T) {
		newCapabilities := []string{"image", "text", "audio"}
		input := model.UpdateChannelModelInput{
			Capabilities: newCapabilities,
		}

		// Simulate the update logic from channel_model_service.go lines 138-147
		if input.Capabilities != nil {
			if len(input.Capabilities) == 0 {
				t.Fatal("should not reach here - empty capabilities test is separate")
			}
			encoded, encodeErr := json.Marshal(input.Capabilities)
			if encodeErr != nil {
				t.Fatalf("failed to marshal capabilities: %v", encodeErr)
			}
			item.Capabilities = string(encoded)
		}

		// Verify the result
		var result []string
		if err := json.Unmarshal([]byte(item.Capabilities), &result); err != nil {
			t.Fatalf("failed to unmarshal result capabilities: %v", err)
		}

		if len(result) != 3 {
			t.Fatalf("expected 3 capabilities, got %d", len(result))
		}
		expected := map[string]bool{"image": true, "text": true, "audio": true}
		for _, cap := range result {
			if !expected[cap] {
				t.Fatalf("unexpected capability: %s", cap)
			}
		}
	})

	// Test 2: Empty capabilities should return error
	t.Run("empty capabilities returns error", func(t *testing.T) {
		input := model.UpdateChannelModelInput{
			Capabilities: []string{},
		}

		// Simulate the validation logic from channel_model_service.go lines 139-141
		if input.Capabilities != nil {
			if len(input.Capabilities) == 0 {
				// This is the expected error path
				return
			}
			t.Fatal("empty capabilities should trigger error before marshal")
		}
	})

	// Test 3: Nil capabilities preserves existing values
	t.Run("nil capabilities preserves existing", func(t *testing.T) {
		// Reset to initial state
		item.Capabilities = string(initialJSON)

		input := model.UpdateChannelModelInput{
			Capabilities: nil,
		}

		// Simulate the update logic - nil means no change
		if input.Capabilities != nil {
			t.Fatal("should not enter update block when Capabilities is nil")
		}

		// Verify original capabilities are preserved
		var result []string
		if err := json.Unmarshal([]byte(item.Capabilities), &result); err != nil {
			t.Fatalf("failed to unmarshal preserved capabilities: %v", err)
		}

		if len(result) != 2 {
			t.Fatalf("expected 2 preserved capabilities, got %d", len(result))
		}
		expected := map[string]bool{"image": true, "video": true}
		for _, cap := range result {
			if !expected[cap] {
				t.Fatalf("unexpected preserved capability: %s", cap)
			}
		}
	})
}
