package service

import "testing"

func TestModelSelectionRequiresAnExplicitSmartRoutingPool(t *testing.T) {
	if kind := (ModelSelection{}).SelectionKind(); kind != "" {
		t.Fatalf("empty selection kind = %q, want empty", kind)
	}
	if kind := (ModelSelection{AutoRoutingPoolID: 9}).SelectionKind(); kind != ModelSelectionAuto {
		t.Fatalf("pool selection kind = %q, want %q", kind, ModelSelectionAuto)
	}
}

func TestModelSelectionRejectsMixedRoutingIdentities(t *testing.T) {
	tests := []ModelSelection{
		{},
		{Kind: ModelSelectionAuto, AutoRoutingPoolID: 9, ChannelID: 2, ChannelModelID: 22},
		{Kind: ModelSelectionPhysical, ChannelID: 2},
		{Kind: ModelSelectionMerge, ChannelID: 2, ChannelModelID: 22, MergeGroupName: "group"},
	}
	for _, selection := range tests {
		if err := validateModelSelection(selection); err == nil {
			t.Fatalf("mixed selection should fail: %#v", selection)
		}
	}
}

func TestGenerationTypeUsesStructuredCapabilityBeforeAmbiguousPath(t *testing.T) {
	for _, test := range []struct {
		name       string
		selection  ModelSelection
		path       string
		generation string
	}{
		{name: "image chat route", selection: ModelSelection{Capability: "image"}, path: "/v1/chat/completions", generation: "image"},
		{name: "video polling route", selection: ModelSelection{Capability: "video"}, path: "/v1/videos/task-1", generation: "video"},
		{name: "legacy chat fallback", path: "/v1/chat/completions", generation: "text"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := generationTypeForSelection(test.selection, test.path); got != test.generation {
				t.Fatalf("generation = %q, want %q", got, test.generation)
			}
		})
	}
}
