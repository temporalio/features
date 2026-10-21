package sdkbuild

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

const rustProgramName = "temporal-features-rust"

// RustFeature identifies a Rust feature source file to compile into the harness.
type RustFeature struct {
	Name string
	Path string
}

// BuildRustProgramOptions are options for BuildRustProgram.
type BuildRustProgramOptions struct {
	// Directory that will have a temporary directory created underneath.
	BaseDir string
	// Directory containing the Rust harness crate.
	HarnessDir string
	// If not set, Cargo resolves the latest SDK version compatible with the
	// harness. A directory is treated as a local sdk-rust checkout; otherwise
	// this is an exact crate version (with a leading "v" trimmed if present).
	Version string
	// Rust feature files to compile into the program.
	Features []RustFeature
	// If present, this directory is expected to exist beneath base dir. Otherwise
	// a temporary dir is created.
	DirName string
	// If present, applied to build commands before run.
	ApplyToCommand func(context.Context, *exec.Cmd) error
	// If present, custom writers that will capture stdout/stderr.
	Stdout io.Writer
	Stderr io.Writer
}

// RustProgram is a Rust-specific implementation of Program.
type RustProgram struct {
	dir string
}

var _ Program = (*RustProgram)(nil)

// BuildRustProgram builds a Rust program. If completed successfully, this can
// be stored and re-obtained via RustProgramFromDir() with the Dir() value.
func BuildRustProgram(ctx context.Context, options BuildRustProgramOptions) (*RustProgram, error) {
	if options.BaseDir == "" {
		return nil, fmt.Errorf("base dir required")
	} else if options.HarnessDir == "" {
		return nil, fmt.Errorf("harness dir required")
	} else if len(options.Features) == 0 {
		return nil, fmt.Errorf("at least one Rust feature required")
	}

	success := false
	var dir string
	if options.DirName != "" {
		dir = filepath.Join(options.BaseDir, options.DirName)
	} else {
		var err error
		dir, err = os.MkdirTemp(options.BaseDir, "program-")
		if err != nil {
			return nil, fmt.Errorf("failed making temp dir: %w", err)
		}
		defer func() {
			if !success {
				_ = os.RemoveAll(dir)
			}
		}()
	}

	manifest, localSDKDir, err := rustManifest(dir, options)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte(manifest), 0644); err != nil {
		return nil, fmt.Errorf("failed writing Cargo.toml: %w", err)
	}

	srcDir := filepath.Join(dir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		return nil, fmt.Errorf("failed creating src dir: %w", err)
	}
	mainContents, err := rustMain(srcDir, options.Features)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(srcDir, "main.rs"), []byte(mainContents), 0644); err != nil {
		return nil, fmt.Errorf("failed writing main.rs: %w", err)
	}

	if localSDKDir != "" {
		toolchainPath := filepath.Join(localSDKDir, "rust-toolchain.toml")
		if contents, err := os.ReadFile(toolchainPath); err == nil {
			if err := os.WriteFile(filepath.Join(dir, "rust-toolchain.toml"), contents, 0644); err != nil {
				return nil, fmt.Errorf("failed writing rust-toolchain.toml: %w", err)
			}
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed reading rust-toolchain.toml: %w", err)
		}
	}

	cmd := exec.CommandContext(ctx, "cargo", "build", "--target-dir", "target")
	cmd.Dir = dir
	setupCommandIO(cmd, options.Stdout, options.Stderr)
	if options.ApplyToCommand != nil {
		if err := options.ApplyToCommand(ctx, cmd); err != nil {
			return nil, err
		}
	}
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed cargo build: %w", err)
	}

	success = true
	return &RustProgram{dir: dir}, nil
}

func rustManifest(dir string, options BuildRustProgramOptions) (string, string, error) {
	harnessDir, err := filepath.Abs(options.HarnessDir)
	if err != nil {
		return "", "", fmt.Errorf("failed resolving harness dir: %w", err)
	}
	if _, err := os.Stat(filepath.Join(harnessDir, "Cargo.toml")); err != nil {
		return "", "", fmt.Errorf("failed finding harness Cargo.toml: %w", err)
	}
	harnessRel, err := filepath.Rel(dir, harnessDir)
	if err != nil {
		return "", "", fmt.Errorf("failed relativizing harness dir: %w", err)
	}

	version := strings.TrimPrefix(options.Version, "v")
	localSDKDir := ""
	patches := ""
	isLocalPath := false
	if options.Version != "" {
		if stat, statErr := os.Stat(options.Version); statErr == nil && stat.IsDir() {
			isLocalPath = true
		} else if strings.ContainsAny(options.Version, `/\`) {
			isLocalPath = true
		}
	}
	if isLocalPath {
		localSDKDir, err = filepath.Abs(options.Version)
		if err != nil {
			return "", "", fmt.Errorf("failed resolving Rust SDK dir: %w", err)
		}
		version, err = rustCrateVersion(filepath.Join(localSDKDir, "crates", "sdk", "Cargo.toml"))
		if err != nil {
			return "", "", err
		}
		patches, err = rustLocalPatches(localSDKDir)
		if err != nil {
			return "", "", err
		}
	}
	if version == "" {
		version = "*"
	} else {
		version = "=" + version
	}

	dependencyNames := []string{
		"temporalio-client",
		"temporalio-common",
		"temporalio-macros",
		"temporalio-sdk",
		"temporalio-sdk-core",
		"temporalio-workflow",
	}
	var dependencies strings.Builder
	for _, name := range dependencyNames {
		fmt.Fprintf(&dependencies, "%s = %s\n", name, strconv.Quote(version))
	}

	manifest := fmt.Sprintf(`[package]
name = %q
version = "0.0.0"
edition = "2024"
publish = false

[dependencies]
anyhow = "1"
base64 = "0.22"
futures = "0.3"
prost = "0.14"
serde_json = "1"
temporalio-features-harness = { path = %s }
%stokio = { version = "1", features = ["macros", "rt-multi-thread"] }
uuid = { version = "1", features = ["v4"] }
%s
[workspace]
`, rustProgramName, strconv.Quote(filepath.ToSlash(harnessRel)), dependencies.String(), patches)
	return manifest, localSDKDir, nil
}

func rustLocalPatches(sdkDir string) (string, error) {
	cratePaths := map[string]string{
		"temporalio-client":      "client",
		"temporalio-common":      "common",
		"temporalio-common-wasm": "common-wasm",
		"temporalio-macros":      "macros",
		"temporalio-protos":      "protos",
		"temporalio-sdk":         "sdk",
		"temporalio-sdk-core":    "sdk-core",
		"temporalio-workflow":    "workflow",
	}
	names := make([]string, 0, len(cratePaths))
	for name := range cratePaths {
		names = append(names, name)
	}
	sort.Strings(names)

	var patches strings.Builder
	patches.WriteString("\n[patch.crates-io]\n")
	for _, name := range names {
		crateDir := filepath.Join(sdkDir, "crates", cratePaths[name])
		if _, err := os.Stat(filepath.Join(crateDir, "Cargo.toml")); err != nil {
			return "", fmt.Errorf("failed finding %s in local Rust SDK: %w", name, err)
		}
		fmt.Fprintf(&patches, "%s = { path = %s }\n", name, strconv.Quote(filepath.ToSlash(crateDir)))
	}
	return patches.String(), nil
}

func rustCrateVersion(manifestPath string) (string, error) {
	f, err := os.Open(manifestPath)
	if err != nil {
		return "", fmt.Errorf("failed opening Rust SDK manifest: %w", err)
	}
	defer f.Close()

	inPackage := false
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "[package]" {
			inPackage = true
			continue
		}
		if inPackage && strings.HasPrefix(line, "[") {
			break
		}
		if inPackage && strings.HasPrefix(line, "version") {
			_, value, found := strings.Cut(line, "=")
			if found {
				version := strings.Trim(strings.TrimSpace(value), `"`)
				if version != "" {
					return version, nil
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("failed reading Rust SDK manifest: %w", err)
	}
	return "", fmt.Errorf("failed finding package version in %s", manifestPath)
}

func rustMain(srcDir string, features []RustFeature) (string, error) {
	features = append([]RustFeature(nil), features...)
	sort.Slice(features, func(i, j int) bool { return features[i].Name < features[j].Name })

	var modules, registrations strings.Builder
	for i, feature := range features {
		if feature.Name == "" || feature.Path == "" {
			return "", fmt.Errorf("Rust feature name and path required")
		}
		absPath, err := filepath.Abs(feature.Path)
		if err != nil {
			return "", fmt.Errorf("failed resolving Rust feature %q: %w", feature.Name, err)
		}
		if _, err := os.Stat(absPath); err != nil {
			return "", fmt.Errorf("failed finding Rust feature %q: %w", feature.Name, err)
		}
		relPath, err := filepath.Rel(srcDir, absPath)
		if err != nil {
			return "", fmt.Errorf("failed relativizing Rust feature %q: %w", feature.Name, err)
		}
		moduleName := fmt.Sprintf("feature_%d", i)
		fmt.Fprintf(&modules, "#[path = %s]\nmod %s;\n", strconv.Quote(filepath.ToSlash(relPath)), moduleName)
		fmt.Fprintf(&registrations, "        (%s, Box::new(%s::feature())),\n", strconv.Quote(feature.Name), moduleName)
	}

	return fmt.Sprintf(`%s
fn main() -> anyhow::Result<()> {
    let features: Vec<(&'static str, Box<dyn temporalio_features_harness::Feature>)> = vec![
%s    ];
    temporalio_features_harness::run(features)
}
`, modules.String(), registrations.String()), nil
}

// RustProgramFromDir recreates a Rust program from a Dir() result of a
// successful BuildRustProgram().
func RustProgramFromDir(dir string) (*RustProgram, error) {
	if _, err := os.Stat(filepath.Join(dir, "Cargo.toml")); err != nil {
		return nil, fmt.Errorf("failed finding Cargo.toml in dir: %w", err)
	}
	if _, err := os.Stat(rustExecutablePath(dir)); err != nil {
		return nil, fmt.Errorf("failed finding Rust program in dir: %w", err)
	}
	return &RustProgram{dir: dir}, nil
}

func rustExecutablePath(dir string) string {
	exe := rustProgramName
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	return filepath.Join(dir, "target", "debug", exe)
}

// Dir is the directory to run in.
func (r *RustProgram) Dir() string { return r.dir }

// NewCommand makes a new command for the given args.
func (r *RustProgram) NewCommand(ctx context.Context, args ...string) (*exec.Cmd, error) {
	cmd := exec.CommandContext(ctx, rustExecutablePath(r.dir), args...)
	cmd.Dir = r.dir
	setupCommandIO(cmd, nil, nil)
	return cmd, nil
}
