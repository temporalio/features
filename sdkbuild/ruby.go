package sdkbuild

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// BuildRubyProgramOptions are options for BuildRubyProgram.
type BuildRubyProgramOptions struct {
	// Directory that will have a temporary directory created underneath.
	BaseDir string
	// Directory containing the Ruby source (gemspec + runner.rb).
	SourceDir string
	// If not set, no version constraint is applied and the package manager
	// resolves to the latest release. If it contains a slash it is assumed
	// to be a path to the Ruby SDK repo. Otherwise it is a specific version
	// (with leading "v" trimmed if present).
	Version string
	// If present, this directory is expected to exist beneath base dir. Otherwise
	// a temporary dir is created.
	DirName string
	// If present, additional gems to add to the generated Gemfile.
	MoreDependencies []RubyDependency
	// If present, custom writers that will capture stdout/stderr.
	Stdout io.Writer
	Stderr io.Writer
}

// RubyDependency is an additional gem dependency for the generated Gemfile.
type RubyDependency struct {
	Name    string
	Version string
	Path    string
}

// RubyProgram is a Ruby-specific implementation of Program.
type RubyProgram struct {
	dir    string
	source string
}

var _ Program = (*RubyProgram)(nil)

// BuildRubyProgram builds a Ruby program. If completed successfully, this
// can be stored and re-obtained via RubyProgramFromDir() with the Dir() value.
func BuildRubyProgram(ctx context.Context, options BuildRubyProgramOptions) (*RubyProgram, error) {
	if options.BaseDir == "" {
		return nil, fmt.Errorf("base dir required")
	}

	sourceDir := options.SourceDir

	// Create temp dir if needed that we will remove if creating is unsuccessful
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

	// Skip if already installed
	if st, err := os.Stat(filepath.Join(dir, "vendor")); err == nil && st.IsDir() {
		return &RubyProgram{dir: dir, source: sourceDir}, nil
	}

	executeCommand := func(name string, args ...string) error {
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Dir = dir
		setupCommandIO(cmd, options.Stdout, options.Stderr)
		return cmd.Run()
	}

	// Build the Gemfile content. We use Bundler's `gemspec` directive to
	// auto-discover the gemspec in the source directory (via path: option).
	// This works for any gem name (harness, omes, etc.).
	gemfileLines := []string{`source "https://rubygems.org"`, ""}
	if strings.ContainsAny(options.Version, `/\`) {
		// It's a path to a local SDK repo
		sdkPath, err := filepath.Abs(options.Version)
		if err != nil {
			return nil, fmt.Errorf("unable to make sdk path absolute: %w", err)
		}
		// The gem is in the temporalio/ subdirectory of the SDK repo
		gemPath := filepath.Join(sdkPath, "temporalio")
		if _, err := os.Stat(filepath.Join(gemPath, "temporalio.gemspec")); err != nil {
			// Try the path directly if no temporalio/ subdirectory
			gemPath = sdkPath
			if _, err := os.Stat(filepath.Join(gemPath, "temporalio.gemspec")); err != nil {
				return nil, fmt.Errorf("failed finding temporalio.gemspec in version dir: %w", err)
			}
		}
		gemfileLines = append(gemfileLines, fmt.Sprintf(`gem "temporalio", path: %q`, gemPath))
	} else if options.Version != "" {
		version := strings.TrimPrefix(options.Version, "v")
		gemfileLines = append(gemfileLines, fmt.Sprintf(`gem "temporalio", %q`, version))
	} else {
		// No version constraint — keep relying on the source gemspec.
	}
	moreDependencyLines, err := renderRubyDependencies(options.MoreDependencies)
	if err != nil {
		return nil, err
	}
	gemfileLines = append(gemfileLines, moreDependencyLines...)
	if len(moreDependencyLines) > 0 {
		gemfileLines = append(gemfileLines, "")
	}
	gemfileLines = append(gemfileLines, fmt.Sprintf(`gemspec path: %q`, sourceDir), "")
	gemfileContent := strings.Join(gemfileLines, "\n")

	if err := os.WriteFile(filepath.Join(dir, "Gemfile"), []byte(gemfileContent), 0644); err != nil {
		return nil, fmt.Errorf("failed writing Gemfile: %w", err)
	}

	// Install dependencies via Bundler into a local vendor directory so the
	// prepared dir is self-contained (important for Docker multi-stage builds).
	// We write .bundle/config directly because the Ruby Docker image sets
	// BUNDLE_APP_CONFIG to /usr/local/bundle, which would cause bundle config
	// to write outside the prepared directory.
	bundleDir := filepath.Join(dir, ".bundle")
	if err := os.MkdirAll(bundleDir, 0755); err != nil {
		return nil, fmt.Errorf("failed creating .bundle dir: %w", err)
	}
	bundleConfig := "---\nBUNDLE_PATH: \"vendor/bundle\"\n"
	if err := os.WriteFile(filepath.Join(bundleDir, "config"), []byte(bundleConfig), 0644); err != nil {
		return nil, fmt.Errorf("failed writing .bundle/config: %w", err)
	}
	if err := executeCommand("bundle", "install"); err != nil {
		return nil, fmt.Errorf("failed installing dependencies: %w", err)
	}

	// When using a local SDK path, compile the native Rust extension
	if strings.ContainsAny(options.Version, `/\`) {
		sdkPath, _ := filepath.Abs(options.Version)
		gemPath := filepath.Join(sdkPath, "temporalio")
		if _, err := os.Stat(filepath.Join(gemPath, "Rakefile")); err != nil {
			gemPath = sdkPath
		}
		if _, err := os.Stat(filepath.Join(gemPath, "Rakefile")); err == nil {
			compileCmd := exec.CommandContext(ctx, "bundle", "exec", "rake", "compile")
			compileCmd.Dir = gemPath
			setupCommandIO(compileCmd, options.Stdout, options.Stderr)
			if err := compileCmd.Run(); err != nil {
				return nil, fmt.Errorf("failed compiling native extension: %w", err)
			}
		}
	}

	success = true
	return &RubyProgram{dir: dir, source: sourceDir}, nil
}

func renderRubyDependencies(dependencies []RubyDependency) ([]string, error) {
	lines := make([]string, 0, len(dependencies))
	for _, dependency := range dependencies {
		if dependency.Name == "" {
			return nil, fmt.Errorf("ruby dependency name required")
		}
		if dependency.Path != "" && dependency.Version != "" {
			return nil, fmt.Errorf("ruby dependency %q cannot have both path and version", dependency.Name)
		}
		if dependency.Path != "" {
			lines = append(lines, fmt.Sprintf(`gem %q, path: %q`, dependency.Name, dependency.Path))
		} else if dependency.Version != "" {
			lines = append(lines, fmt.Sprintf(`gem %q, %q`, dependency.Name, dependency.Version))
		} else {
			lines = append(lines, fmt.Sprintf(`gem %q`, dependency.Name))
		}
	}
	return lines, nil
}

// RubyProgramFromDir recreates the Ruby program from a Dir() result of a
// BuildRubyProgram(). The sourceDir should point to the directory containing
// the gemspec and runner.rb.
func RubyProgramFromDir(dir string, sourceDir string) (*RubyProgram, error) {
	if _, err := os.Stat(filepath.Join(dir, "Gemfile")); err != nil {
		return nil, fmt.Errorf("failed finding Gemfile in dir: %w", err)
	}
	return &RubyProgram{dir: dir, source: sourceDir}, nil
}

// Dir is the directory to run in.
func (r *RubyProgram) Dir() string { return r.dir }

// NewCommand makes a new Ruby command via Bundler.
func (r *RubyProgram) NewCommand(ctx context.Context, args ...string) (*exec.Cmd, error) {
	args = append([]string{"exec", "ruby", filepath.Join(r.source, "runner.rb")}, args...)
	cmd := exec.CommandContext(ctx, "bundle", args...)
	cmd.Dir = r.dir
	setupCommandIO(cmd, nil, nil)
	return cmd, nil
}
