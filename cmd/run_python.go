package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"go.temporal.io/sdk-features/harness/go/cmd"
	"go.temporal.io/server/common/log/tag"
)

// RunPythonExternal runs the Python run in an external process. This expects
// the server to already be started.
func (r *Runner) RunPythonExternal(ctx context.Context, run *cmd.Run) error {
	// To do this, if they supplied a custom version we'll make a temporary
	// directory to run in with a custom pyproject and such. Otherwise we'll just
	// use this directory.

	runDir := r.rootDir
	if r.config.Version != "" {
		// Put pyproject.toml in temp dir
		var err error
		if runDir, err = os.MkdirTemp(r.rootDir, "sdk-features-python-test-"); err != nil {
			return fmt.Errorf("failed creating temp dir: %w", err)
		}
		r.createdTempDir = &runDir
		r.log.Info("Building temporary Python project", tag.NewStringTag("Path", runDir))
		// Use semantic version or path if it's a path
		versionStr := strconv.Quote(r.config.Version)
		if strings.ContainsAny(r.config.Version, "/\\") {
			// We expect a dist/ directory with a single whl file present
			wheels, err := filepath.Glob(filepath.Join(r.config.Version, "dist/*.whl"))
			if err != nil {
				return fmt.Errorf("failed glob wheel lookup: %w", err)
			} else if len(wheels) != 1 {
				return fmt.Errorf("expected single dist wheel, found %v", len(wheels))
			}
			versionStr = "{ path = " + strconv.Quote(wheels[0]) + " }"
		}
		pyProjectTOML := `
[tool.poetry]
name = "sdk-features-python-test-` + filepath.Base(runDir) + `"
version = "0.1.0"
description = "Temporal SDK Features Python Test"
authors = ["Temporal Technologies Inc <sdk@temporal.io>"]

[tool.poetry.dependencies]
python = "^3.7"
temporalio = ` + versionStr + `
sdk-features = { path = "../" }

[build-system]
requires = ["poetry-core>=1.0.0"]
build-backend = "poetry.core.masonry.api"`
		if err := os.WriteFile(filepath.Join(runDir, "pyproject.toml"), []byte(pyProjectTOML), 0644); err != nil {
			return fmt.Errorf("failed writing pyproject.toml: %w", err)
		}

		// Install
		cmd := exec.CommandContext(ctx, "poetry", "install", "--no-dev", "--no-root")
		cmd.Dir = runDir
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed installing: %w", err)
		}
	}

	// Run
	args := append([]string{"run", "python", "-m", "harness.python.main",
		"--server", r.config.Server, "--namespace", r.config.Namespace}, run.ToArgs()...)
	r.log.Debug("Running Poetry", tag.NewStringsTag("Args", args))
	cmd := exec.CommandContext(ctx, "poetry", args...)
	cmd.Dir = runDir
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed running: %w", err)
	}
	return nil
}
