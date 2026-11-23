package cmd

import "testing"

func TestLatestJavaSDKVersion(t *testing.T) {
	for _, test := range []struct {
		name, metadata, release string
	}{
		{
			name: "release rather than latest snapshot",
			metadata: `<metadata><versioning>
				<latest>1.42.0-SNAPSHOT</latest>
				<release>1.41.1</release>
			</versioning></metadata>`,
			release: "1.41.1",
		},
		{
			name:     "missing release does not select a default SDK",
			metadata: `<metadata><versioning><latest>1.42.0-SNAPSHOT</latest></versioning></metadata>`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			version, err := registryQueries["java"].extract([]byte(test.metadata))
			if test.release == "" {
				if err == nil {
					t.Fatalf("expected missing-release error, got %q", version)
				}
			} else if err != nil || version != test.release {
				t.Fatalf("got version %q, error %v; want %q", version, err, test.release)
			}
		})
	}
}
