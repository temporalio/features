package sdkbuild

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestRustManifestSelection(t *testing.T) {
	source, err := os.ReadFile("../harness/rust/Cargo.toml")
	if err != nil {
		t.Fatal(err)
	}
	var original map[string]any
	if err := toml.Unmarshal(source, &original); err != nil {
		t.Fatal(err)
	}
	baseline := strings.TrimPrefix(original["dependencies"].(map[string]any)["temporalio-sdk"].(string), "=")
	for _, version := range []string{"", baseline, "v" + baseline, "9.8.7"} {
		got, locked, err := rustManifest(source, BuildRustProgramOptions{Version: version})
		if err != nil {
			t.Fatal(err)
		}
		if locked != (version != "9.8.7") {
			t.Fatalf("version %q: locked=%v", version, locked)
		}
		if locked && !bytes.Equal(got, source) {
			t.Fatal("baseline manifest changed")
		}
		var manifest map[string]any
		if err := toml.Unmarshal(got, &manifest); err != nil {
			t.Fatal(err)
		}
		want := "=" + strings.TrimPrefix(version, "v")
		if version == "" {
			want = "=" + baseline
		}
		for _, crate := range rustSDKCrates {
			if crate.direct && manifest["dependencies"].(map[string]any)[crate.name] != want {
				t.Fatalf("incorrect selection for %s", crate.name)
			}
		}
	}
	sdk := t.TempDir()
	for _, crate := range rustSDKCrates {
		dir := filepath.Join(sdk, crate.path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "Cargo.toml"), nil, 0644); err != nil {
			t.Fatal(err)
		}
	}
	got, locked, err := rustManifest(source, BuildRustProgramOptions{Version: sdk})
	if err != nil || locked {
		t.Fatalf("local selection: locked=%v, %v", locked, err)
	}
	var manifest map[string]any
	if err := toml.Unmarshal(got, &manifest); err != nil {
		t.Fatal(err)
	}
	patches := manifest["patch"].(map[string]any)["crates-io"].(map[string]any)
	for _, crate := range rustSDKCrates {
		want := filepath.Join(sdk, crate.path)
		if patches[crate.name].(map[string]any)["path"] != want {
			t.Fatalf("missing local patch for %s", crate.name)
		}
		if crate.direct && manifest["dependencies"].(map[string]any)[crate.name].(map[string]any)["path"] != want {
			t.Fatalf("missing direct path for %s", crate.name)
		}
	}
}
