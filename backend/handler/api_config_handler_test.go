package handler

import "testing"

func TestEncodeDecodeIntListMap(t *testing.T) {
	input := map[string][]int{
		"veo-omni-flash":         {10},
		"grok-imagine-video-1.5": {5, 10, 10, 0, -1},
		"":                       {6},
	}

	encoded, err := encodeIntListMap(input)
	if err != nil {
		t.Fatalf("encodeIntListMap returned error: %v", err)
	}

	decoded, err := decodeIntListMap(encoded)
	if err != nil {
		t.Fatalf("decodeIntListMap returned error: %v", err)
	}

	if len(decoded) != 2 {
		t.Fatalf("expected 2 models after cleanup, got %d: %#v", len(decoded), decoded)
	}
	if got := decoded["veo-omni-flash"]; len(got) != 1 || got[0] != 10 {
		t.Fatalf("unexpected veo durations: %#v", got)
	}
	if got := decoded["grok-imagine-video-1.5"]; len(got) != 2 || got[0] != 5 || got[1] != 10 {
		t.Fatalf("unexpected grok durations: %#v", got)
	}
}

func TestDecodeIntListMapEmpty(t *testing.T) {
	decoded, err := decodeIntListMap("")
	if err != nil {
		t.Fatalf("decodeIntListMap returned error: %v", err)
	}
	if len(decoded) != 0 {
		t.Fatalf("expected empty map, got %#v", decoded)
	}
}

func TestEncodeDecodeBoolMap(t *testing.T) {
	input := map[string]bool{
		"veo-omni-flash": true,
		"grok-imagine-video-1.5": false,
		"": true,
	}

	encoded, err := encodeBoolMap(input)
	if err != nil {
		t.Fatalf("encodeBoolMap returned error: %v", err)
	}

	decoded, err := decodeBoolMap(encoded)
	if err != nil {
		t.Fatalf("decodeBoolMap returned error: %v", err)
	}

	if len(decoded) != 1 || !decoded["veo-omni-flash"] {
		t.Fatalf("unexpected decoded bool map: %#v", decoded)
	}
}
