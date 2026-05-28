package cmd

import "testing"

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
			name: "invalid name",
			config: RunFeatureConfig{RunVariants: []RunVariantConfig{
				{Name: "enabled/invalid"},
			}},
			wantErr: true,
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
