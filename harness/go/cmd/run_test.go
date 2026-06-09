package cmd

import (
	"reflect"
	"testing"
)

func TestRunToArgsAndFromArgsRoundTrip(t *testing.T) {
	run := Run{Features: []RunFeature{
		{Dir: "activity/basic", TaskQueue: "tq-basic"},
		{Dir: "nexus/sync_success", TaskQueue: "tq-nexus", NexusEndpoint: "endpoint-name"},
	}}

	args := run.ToArgs()
	var got Run
	if err := got.FromArgs(args); err != nil {
		t.Fatal(err)
	}
	if len(got.Features) != len(run.Features) {
		t.Fatalf("got %d features, want %d", len(got.Features), len(run.Features))
	}
	for i, want := range run.Features {
		if !reflect.DeepEqual(got.Features[i], want) {
			t.Fatalf("feature %d = %+v, want %+v; args=%v", i, got.Features[i], want, args)
		}
	}
}

func TestRunFeatureConfigValidateRunVariants(t *testing.T) {
	tests := []struct {
		name    string
		config  RunFeatureConfig
		wantErr bool
	}{
		{
			name: "valid",
			config: RunFeatureConfig{RunVariants: []RunVariantConfig{
				{Name: "enabled"},
				{Name: "disabled_1"},
			}},
		},
		{
			name: "empty name",
			config: RunFeatureConfig{RunVariants: []RunVariantConfig{
				{Name: ""},
			}},
			wantErr: true,
		},
		{
			name: "duplicate name",
			config: RunFeatureConfig{RunVariants: []RunVariantConfig{
				{Name: "enabled"},
				{Name: "enabled"},
			}},
			wantErr: true,
		},
		{
			name: "punctuation allowed",
			config: RunFeatureConfig{RunVariants: []RunVariantConfig{
				{Name: "enabled/invalid"},
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
