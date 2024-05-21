package sdkbuild

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// BuildPhpProgramOptions are options for BuildPhpProgram.
type BuildPhpProgramOptions struct {
	// Directory that will have a temporary directory created underneath. This
	// should be a Poetry project with a pyproject.toml.
	BaseDir string
	// Required version. If it contains a slash it is assumed to be a path with
	// a single wheel in the dist directory. Otherwise it is a specific version
	// (with leading "v" is trimmed if present).
	Version string
	// If present, this directory is expected to exist beneath base dir. Otherwise
	// a temporary dir is created.
	DirName string
}

// PhpProgram is a PHP-specific implementation of Program.
type PhpProgram struct {
	dir string
}

var _ Program = (*PhpProgram)(nil)

// BuildPhpProgram builds a PHP program. If completed successfully, this
// can be stored and re-obtained via PhpProgramFromDir() with the Dir() value
// (but the entire BaseDir must be present too).
func BuildPhpProgram(ctx context.Context, options BuildPhpProgramOptions) (*PhpProgram, error) {
	if options.BaseDir == "" {
		return nil, fmt.Errorf("base dir required")
	} else if options.DirName == "" {
		return nil, fmt.Errorf("PHP dir required")
	} else if options.Version == "" {
		return nil, fmt.Errorf("version required")
	}

	// Working directory
	dir := filepath.Join(options.BaseDir, options.DirName)

	// Skip if installed
	// if st, err := os.Stat(filepath.Join(options.Version, "vendor")); err != nil || st.IsDir() {
	// 	return &PhpProgram{dir}, nil
	// }

	// Copy composer.json from options.BaseDir into dir
	data, err := os.ReadFile(filepath.Join(options.BaseDir, "composer.json"))
	if err != nil {
		return nil, fmt.Errorf("failed reading composer.json file: %w", err)
	}
	err = os.WriteFile(filepath.Join(dir, "composer.json"), data, 0755)
	if err != nil {
		return nil, fmt.Errorf("failed writing composer.json file: %w", err)
	}

	// Setup required SDK version
	cmd := exec.CommandContext(ctx, "composer", "req", "temporal/sdk", options.Version, "-W", "--no-install")
	cmd.Dir = dir
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed installing SDK deps: %w", err)
	}

	// Install dependencies via composer
	cmd = exec.CommandContext(ctx, "composer", "i", "-n", "-o", "-q", "--no-scripts")
	cmd.Dir = dir
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
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
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("failed downloading RoadRunner: %w", err)
		}
	}

	return &PhpProgram{dir}, nil
}

// PhpProgramFromDir recreates the Php program from a Dir() result of a
// BuildPhpProgram(). Note, the base directory of dir when it was built must
// also be present.
func PhpProgramFromDir(dir string) (*PhpProgram, error) {
	// Quick sanity check on the presence of package.json here
	if _, err := os.Stat(filepath.Join(dir, "composer.json")); err != nil {
		return nil, fmt.Errorf("failed finding composer.json in dir: %w", err)
	}
	return &PhpProgram{dir}, nil
}

// Dir is the directory to run in.
func (p *PhpProgram) Dir() string { return p.dir }

// NewCommand makes a new RoadRunner run command
func (p *PhpProgram) NewCommand(ctx context.Context, args ...string) (*exec.Cmd, error) {
	args = append([]string{"runner.php"}, args...)
	cmd := exec.CommandContext(ctx, "php", args...)
	cmd.Dir = p.dir
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd, nil
}
