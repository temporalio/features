package sdkbuild

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"golang.org/x/mod/semver"
)

// BuildRustProgramOptions are options for BuildRustProgram.
type BuildRustProgramOptions struct {
	// PackageDir is the checked-in Cargo package. BaseDir must be beneath the
	// features repository so the copied build script can discover its features.
	PackageDir string
	// Directory that will have a temporary directory created underneath.
	BaseDir string
	// If present, this directory is expected to exist beneath base dir.
	// Otherwise a temporary directory is created.
	DirName string
	// Version selects an exact published SDK version, defaulting to Cargo.toml.
	// A value containing a slash or backslash selects a local SDK workspace.
	Version string
	// If present, custom writers that will capture stdout/stderr.
	Stdout io.Writer
	Stderr io.Writer
}

// RustProgram is a compiled, host-native Cargo program.
type RustProgram struct{ dir string }

var _ Program = (*RustProgram)(nil)

var rustSDKCrates = []struct {
	name, path string
	direct     bool
}{
	{"temporalio-sdk", "crates/sdk", true},
	{"temporalio-client", "crates/client", true},
	{"temporalio-workflow", "crates/workflow", true},
	{"temporalio-macros", "crates/macros", true},
	{"temporalio-common", "crates/common", true},
	{"temporalio-common-wasm", "crates/common-wasm", false},
	{"temporalio-sdk-core", "crates/sdk-core", false},
	{"temporalio-protos", "crates/protos", false},
}

// BuildRustProgram copies a Cargo package and builds its binary. SDK source
// workspaces remain read-only. Failed temporary projects are removed; named
// projects are retained for diagnosis.
func BuildRustProgram(ctx context.Context, options BuildRustProgramOptions) (*RustProgram, error) {
	if options.BaseDir == "" || options.PackageDir == "" {
		return nil, fmt.Errorf("base dir and package dir required")
	}
	source, err := os.ReadFile(filepath.Join(options.PackageDir, "Cargo.toml"))
	if err != nil {
		return nil, err
	}
	manifest, locked, err := rustManifest(source, options)
	if err != nil {
		return nil, err
	}
	base, err := filepath.Abs(options.BaseDir)
	if err != nil {
		return nil, err
	}
	var dir string
	if options.DirName == "" {
		dir, err = os.MkdirTemp(base, "program-rust-")
	} else {
		dir = filepath.Join(base, options.DirName)
	}
	if err != nil {
		return nil, fmt.Errorf("create Rust project directory: %w", err)
	}
	success := false
	defer func() {
		if !success && options.DirName == "" {
			_ = os.RemoveAll(dir)
		}
	}()
	if err := os.CopyFS(filepath.Join(dir, "src"), os.DirFS(filepath.Join(options.PackageDir, "src"))); err != nil {
		return nil, fmt.Errorf("copy Rust sources: %w", err)
	}
	// Seed every selection's dependency graph; only the baseline is locked.
	for _, name := range []string{"Cargo.toml", "build.rs", "rust-toolchain.toml", "Cargo.lock"} {
		contents := manifest
		if name != "Cargo.toml" {
			contents, err = os.ReadFile(filepath.Join(options.PackageDir, name))
			if err != nil {
				return nil, err
			}
		}
		if err := os.WriteFile(filepath.Join(dir, name), contents, 0644); err != nil {
			return nil, fmt.Errorf("copy %s: %w", name, err)
		}
	}
	target, err := filepath.Abs(filepath.Join(options.PackageDir, "target"))
	if err != nil {
		return nil, err
	}
	if configured := os.Getenv("CARGO_TARGET_DIR"); configured != "" {
		target = configured
		if !filepath.IsAbs(target) {
			target = filepath.Join(dir, target)
		}
	}
	args := []string{"build", "--release", "--bin", "program", "--target-dir", target}
	if locked {
		args = append(args, "--locked")
	}
	cmd := exec.CommandContext(ctx, "cargo", args...)
	cmd.Dir = dir
	setupCommandIO(cmd, options.Stdout, options.Stderr)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("cargo build: %w", err)
	}
	// Keep Cargo's output intact when the target directory is shared.
	binary, err := os.ReadFile(filepath.Join(target, "release", rustExecutable()))
	if err != nil {
		return nil, fmt.Errorf("read Rust executable: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, rustExecutable()), binary, 0755); err != nil {
		return nil, fmt.Errorf("retain Rust executable: %w", err)
	}
	success = true
	return &RustProgram{dir: dir}, nil
}

// rustManifest changes only SDK dependency selection. The baseline version and
// all non-SDK settings belong to the checked-in Cargo manifest.
func rustManifest(source []byte, options BuildRustProgramOptions) ([]byte, bool, error) {
	var manifest map[string]any
	if err := toml.Unmarshal(source, &manifest); err != nil {
		return nil, false, err
	}
	deps, ok := manifest["dependencies"].(map[string]any)
	if !ok {
		return nil, false, fmt.Errorf("Rust package requires dependencies")
	}
	baseline, ok := deps["temporalio-sdk"].(string)
	if !ok || !strings.HasPrefix(baseline, "=") {
		return nil, false, fmt.Errorf("Rust package requires an exact temporalio-sdk version")
	}
	version, sdkPath := options.Version, ""
	if strings.ContainsAny(version, `/\`) {
		sdkPath, version = version, ""
	}
	version = strings.TrimPrefix(version, "v")
	if version == "" {
		version = strings.TrimPrefix(baseline, "=")
	}
	if sdkPath == "" {
		if !semver.IsValid("v"+version) ||
			!strings.HasPrefix("v"+version, semver.MajorMinor("v"+version)+".") {
			return nil, false, fmt.Errorf("Rust SDK version must be a complete published version, got %q", version)
		}
		if "="+version == baseline {
			return source, true, nil
		}
	} else {
		var err error
		sdkPath, err = filepath.Abs(sdkPath)
		if err != nil {
			return nil, false, err
		}
	}
	patches := make(map[string]any)
	for _, crate := range rustSDKCrates {
		if sdkPath == "" && !crate.direct {
			continue
		}
		var dep any = "=" + version
		if sdkPath != "" {
			path := filepath.Join(sdkPath, crate.path)
			if info, err := os.Stat(filepath.Join(path, "Cargo.toml")); err != nil || info.IsDir() {
				return nil, false, fmt.Errorf("SDK workspace requires manifest %s", filepath.Join(path, "Cargo.toml"))
			}
			dep = map[string]any{"path": path}
			patches[crate.name] = dep
		}
		if crate.direct {
			deps[crate.name] = dep
		}
	}
	if sdkPath != "" {
		manifest["patch"] = map[string]any{"crates-io": patches}
	}
	contents, err := toml.Marshal(manifest)
	return contents, false, err
}

func rustExecutable() string {
	if runtime.GOOS == "windows" {
		return "program.exe"
	}
	return "program"
}

// RustProgramFromDir opens a prepared directory without invoking Cargo.
func RustProgramFromDir(dir string) (*RustProgram, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(filepath.Join(abs, rustExecutable()))
	if err != nil {
		return nil, fmt.Errorf("find prepared Rust executable: %w", err)
	}
	if !info.Mode().IsRegular() || (runtime.GOOS != "windows" && info.Mode().Perm()&0111 == 0) {
		return nil, fmt.Errorf("prepared Rust executable is not executable")
	}
	return &RustProgram{dir: abs}, nil
}

// Dir is the prepared program directory.
func (r *RustProgram) Dir() string { return r.dir }

// NewCommand executes the prepared binary directly, without Cargo or sources.
func (r *RustProgram) NewCommand(ctx context.Context, args ...string) (*exec.Cmd, error) {
	cmd := exec.CommandContext(ctx, filepath.Join(r.dir, rustExecutable()), args...)
	cmd.Dir = r.dir
	setupCommandIO(cmd, nil, nil)
	return cmd, nil
}
