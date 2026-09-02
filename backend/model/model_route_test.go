package model

import "testing"

func TestModelRoutesNormalizeKnownValuesAndRejectUnknownValues(t *testing.T) {
	tests := []struct {
		name      string
		normalize func(string) (string, error)
		input     string
		want      string
		wantError bool
	}{
		{name: "blank image generation defaults to auto", normalize: NormalizeImageGenerateRoute, input: " ", want: ImageRouteAuto},
		{name: "image edit generations", normalize: NormalizeImageEditRoute, input: " Generations ", want: ImageRouteGenerations},
		{name: "custom video", normalize: NormalizeVideoRoute, input: "CUSTOM", want: "custom"},
		{name: "edits cannot generate images", normalize: NormalizeImageGenerateRoute, input: "edits", wantError: true},
		{name: "unknown image edit route", normalize: NormalizeImageEditRoute, input: "vendor-image", wantError: true},
		{name: "unknown video route", normalize: NormalizeVideoRoute, input: "vendor-video", wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := test.normalize(test.input)
			if test.wantError {
				if err == nil {
					t.Fatalf("normalize(%q) = %q, want error", test.input, got)
				}
				return
			}
			if err != nil || got != test.want {
				t.Fatalf("normalize(%q) = %q, %v; want %q", test.input, got, err, test.want)
			}
		})
	}
}
