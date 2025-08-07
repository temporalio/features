package sdkbuild

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"io"
)

// BuildPhpProgramOptions are options for BuildPhpProgram.
type BuildPhpProgramOptions struct {
	// If not set, the default version from composer.json is used.
	Version string
	// If present, this directory is expected to exist beneath base dir. Otherwise
	// a temporary dir is created.
	DirName string
	RootDir string
	// If present, custom writers that will capture stdout/stderr.
	Stdout io.Writer
	Stderr io.Writer
}

// PhpProgram is a PHP-specific implementation of Program.
type PhpProgram struct {
	dir    string
	source string
}

var _ Program = (*PhpProgram)(nil)

// BuildPhpProgram builds a PHP program. If completed successfully, this
// can be stored and re-obtained via PhpProgramFromDir() with the Dir() value
func BuildPhpProgram(ctx context.Context, options BuildPhpProgramOptions) (*PhpProgram, error) {
	// Working directory
	// Create temp dir if needed that we will remove if creating is unsuccessful
	var dir string
	if options.DirName != "" {
		dir = filepath.Join(options.RootDir, options.DirName)
	} else {
		var err error
		dir, err = os.MkdirTemp(options.RootDir, "program-")
		if err != nil {
			return nil, fmt.Errorf("failed making temp dir: %w", err)
		}
	}

	sourceDir := GetSourceDir(options.RootDir)

	// Skip if installed
	if st, err := os.Stat(filepath.Join(dir, "vendor")); err == nil && st.IsDir() {
		return &PhpProgram{dir: dir, source: sourceDir}, nil
	}

	// Copy composer.json from sourceDir into dir
	data, err := os.ReadFile(filepath.Join(sourceDir, "composer.json"))
	if err != nil {
		return nil, fmt.Errorf("failed reading composer.json file: %w", err)
	}
	err = os.WriteFile(filepath.Join(dir, "composer.json"), data, 0755)
	if err != nil {
		return nil, fmt.Errorf("failed writing composer.json file: %w", err)
	}

	// Copy .rr.yaml from sourceDir into dir
	data, err = os.ReadFile(filepath.Join(sourceDir, ".rr.yaml"))
	if err != nil {
		return nil, fmt.Errorf("failed reading .rr.yaml file: %w", err)
	}
	err = os.WriteFile(filepath.Join(dir, ".rr.yaml"), data, 0755)
	if err != nil {
		return nil, fmt.Errorf("failed writing .rr.yaml file: %w", err)
	}

	var cmd *exec.Cmd
	// Setup required SDK version if specified
	if options.Version != "" {
		cmd = exec.CommandContext(ctx, "composer", "req", "temporal/sdk", options.Version, "-W", "--no-install", "--ignore-platform-reqs")
		cmd.Dir = dir
		setupCommandIO(cmd, options.Stdout, options.Stderr)
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("failed installing SDK deps: %w", err)
		}
	}

	// Install dependencies via composer
	cmd = exec.CommandContext(ctx, "composer", "i", "-n", "-o", "-q", "--no-scripts", "--ignore-platform-reqs")
	cmd.Dir = dir
	setupCommandIO(cmd, options.Stdout, options.Stderr)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed installing SDK deps: %w", err)
	}

	// Download RoadRunner
	rrExe := filepath.Join(dir, "rr")
	if runtime.GOOS == "windows" {
		rrExe += ".exe"
	}
	_, err = os.Stat(rrExe)
	if os.IsNotExist(err) {
		cmd = exec.CommandContext(ctx, "composer", "run", "rr-get")
		cmd.Dir = dir
		setupCommandIO(cmd, options.Stdout, options.Stderr)
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("failed downloading RoadRunner: %w", err)
		}
	}

	return &PhpProgram{dir, sourceDir}, nil
}

// PhpProgramFromDir recreates the Php program from a Dir() result of a
// BuildPhpProgram(). Note, the base directory of dir when it was built must
// also be present.
func PhpProgramFromDir(dir string, rootDir string) (*PhpProgram, error) {
	// Quick sanity check on the presence of package.json here
	if _, err := os.Stat(filepath.Join(dir, "composer.json")); err != nil {
		return nil, fmt.Errorf("failed finding composer.json in dir: %w", err)
	}
	return &PhpProgram{dir, GetSourceDir(rootDir)}, nil
}

func GetSourceDir(rootDir string) string {
	return filepath.Join(rootDir, "harness", "php")
}

// Dir is the directory to run in.
func (p *PhpProgram) Dir() string { return p.dir }

// NewCommand makes a new RoadRunner run command
func (p *PhpProgram) NewCommand(ctx context.Context, args ...string) (*exec.Cmd, error) {
	args = append([]string{filepath.Join(p.source, "runner.php")}, args...)
	cmd := exec.CommandContext(ctx, "php", args...)
	cmd.Dir = p.dir
	setupCommandIO(cmd, nil, nil)
	return cmd, nil
}
