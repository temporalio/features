package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/temporalio/features/harness/go/cmd"
	"github.com/temporalio/features/sdkbuild"
)

// PreparePythonExternal prepares a Python run without running it. The preparer
// config directory if present is expected to be a subdirectory name just
// beneath the root directory.
func (p *Preparer) BuildPythonProgram(ctx context.Context) (sdkbuild.Program, error) {
	p.log.Info("Building Python project", "DirName", p.config.DirName)

	// Get version from pyproject.toml if not present
	version := p.config.Version
	if version == "" {
		b, err := os.ReadFile(filepath.Join(p.rootDir, "pyproject.toml"))
		if err != nil {
			return nil, fmt.Errorf("failed reading pyproject.toml: %w", err)
		}
		for _, line := range strings.Split(string(b), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "temporalio = ") {
				version = line[strings.Index(line, `"`)+1 : strings.LastIndex(line, `"`)]
				break
			}
		}
		if version == "" {
			return nil, fmt.Errorf("version not found in pyproject.toml")
		}
	}

	prog, err := sdkbuild.BuildPythonProgram(ctx, sdkbuild.BuildPythonProgramOptions{
		BaseDir:        p.rootDir,
		DirName:        p.config.DirName,
		Version:        version,
		DependencyName: "features",
	})
	if err != nil {
		return nil, fmt.Errorf("failed preparing: %w", err)
	}
	return prog, nil
}

// RunPythonExternal runs the Python run in an external process. This expects
// the server to already be started.
func (r *Runner) RunPythonExternal(ctx context.Context, run *cmd.Run) error {
	// If program not built, build it
	if r.program == nil {
		var err error
		if r.program, err = NewPreparer(r.config.PrepareConfig).BuildPythonProgram(ctx); err != nil {
			return err
		}
	}

	// Build args
	args := make([]string, 0, 64)
	args = append(args, "harness.python.main")
	args, err := r.config.appendFlags(args)
	if err != nil {
		return err
	}
	args = append(args, run.ToArgs()...)

	// Run
	cmd, err := r.program.NewCommand(ctx, args...)
	if err == nil {
		r.log.Debug("Running Python separately", "Args", cmd.Args)
		err = cmd.Run()
	}
	if err != nil {
		return fmt.Errorf("failed running: %w", err)
	}
	return nil
}
