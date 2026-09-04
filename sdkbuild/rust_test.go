package sdkbuild

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRustCrateVersion(t *testing.T) {
	manifest := filepath.Join(t.TempDir(), "Cargo.toml")
	if err := os.WriteFile(manifest, []byte(`[package]
name = "temporalio-sdk"
version = "0.5.1"

[dependencies]
version = "ignored"
`), 0644); err != nil {
		t.Fatal(err)
	}

	version, err := rustCrateVersion(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if version != "0.5.1" {
		t.Fatalf("version = %q, want 0.5.1", version)
	}
}

func TestRustMainSortsFeatures(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.rs")
	second := filepath.Join(dir, "second.rs")
	for _, path := range []string{first, second} {
		if err := os.WriteFile(path, []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
	}

	main, err := rustMain(dir, []RustFeature{
		{Name: "z/last", Path: second},
		{Name: "a/first", Path: first},
	})
	if err != nil {
		t.Fatal(err)
	}
	firstIndex := strings.Index(main, `("a/first", Box::new(feature_0::feature()))`)
	secondIndex := strings.Index(main, `("z/last", Box::new(feature_1::feature()))`)
	if firstIndex == -1 || secondIndex == -1 || firstIndex >= secondIndex {
		t.Fatalf("features were not sorted in generated main:\n%s", main)
	}
}

func TestRustManifestUsesStandaloneWorkspace(t *testing.T) {
	dir := t.TempDir()
	harnessDir := filepath.Join(dir, "harness")
	if err := os.MkdirAll(harnessDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(harnessDir, "Cargo.toml"), []byte("[package]\n"), 0644); err != nil {
		t.Fatal(err)
	}

	manifest, _, err := rustManifest(filepath.Join(dir, "program"), BuildRustProgramOptions{
		HarnessDir: harnessDir,
		Version:    "0.5.0",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(manifest, "\n[workspace]\n") {
		t.Fatalf("generated manifest does not define a standalone workspace:\n%s", manifest)
	}
}

func TestRustProgramFromDir(t *testing.T) {
	dir := t.TempDir()
	for path, contents := range map[string]string{
		filepath.Join(dir, "Cargo.toml"): "[package]\nname = \"test\"\n",
		rustExecutablePath(dir):          "",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0755); err != nil {
			t.Fatal(err)
		}
	}

	program, err := RustProgramFromDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	command, err := program.NewCommand(context.Background(), "--server", "localhost:7233")
	if err != nil {
		t.Fatal(err)
	}
	if command.Dir != dir {
		t.Fatalf("command dir = %q, want %q", command.Dir, dir)
	}
	wantArgs := []string{rustExecutablePath(dir), "--server", "localhost:7233"}
	if !reflect.DeepEqual(command.Args, wantArgs) {
		t.Fatalf("command args = %q, want %q", command.Args, wantArgs)
	}
}
