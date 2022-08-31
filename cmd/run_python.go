package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"go.temporal.io/sdk-features/harness/go/cmd"
	"go.temporal.io/server/common/log/tag"
)

// PreparePythonExternal prepares a Python run without running it. The preparer
// config directory is expected to be an absolute subdirectory just beneath the
// root directory.
func (p *Preparer) PreparePythonExternal(ctx context.Context) error {
	p.log.Info("Building Python project", tag.NewStringTag("Path", p.config.Dir))
	// Use semantic version or path if it's a path
	versionStr := strconv.Quote(p.config.Version)
	if strings.ContainsAny(p.config.Version, "/\\") {
		// We expect a dist/ directory with a single whl file present
		wheels, err := filepath.Glob(filepath.Join(p.config.Version, "dist/*.whl"))
		if err != nil {
			return fmt.Errorf("failed glob wheel lookup: %w", err)
		} else if len(wheels) != 1 {
			return fmt.Errorf("expected single dist wheel, found %v", len(wheels))
		}
		absWheel, err := filepath.Abs(wheels[0])
		if err != nil {
			return fmt.Errorf("unable to make wheel path absolute: %w", err)
		}
		// There's a strange bug in Poetry or somewhere deeper where, on Windows,
		// the single drive letter has to be capitalized
		if runtime.GOOS == "windows" && absWheel[1] == ':' {
			absWheel = strings.ToUpper(absWheel[:1]) + absWheel[1:]
		}
		versionStr = "{ path = " + strconv.Quote(absWheel) + " }"
	}
	pyProjectTOML := `
[tool.poetry]
name = "sdk-features-python-test-` + filepath.Base(p.config.Dir) + `"
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
	if err := os.WriteFile(filepath.Join(p.config.Dir, "pyproject.toml"), []byte(pyProjectTOML), 0644); err != nil {
		return fmt.Errorf("failed writing pyproject.toml: %w", err)
	}

	// Install
	cmd := exec.CommandContext(ctx, "poetry", "install", "--no-dev", "--no-root", "-v")
	cmd.Dir = p.config.Dir
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed installing: %w", err)
	}
	return nil
}

// RunPythonExternal runs the Python run in an external process. This expects
// the server to already be started.
func (r *Runner) RunPythonExternal(ctx context.Context, run *cmd.Run) error {
	// Create temp directory and prepare if using specific version
	if r.config.Version != "" && r.config.Dir == "" {
		// Create a temp directory and prepare
		var err error
		if r.config.Dir, err = os.MkdirTemp(r.rootDir, "sdk-features-python-test-"); err != nil {
			return fmt.Errorf("failed creating temp dir: %w", err)
		}
		r.createdTempDir = &r.config.Dir
		if err := NewPreparer(r.config.PrepareConfig).PreparePythonExternal(ctx); err != nil {
			return err
		}
	}

	// Run in this directory unless we have prepared directory already
	runDir := r.rootDir
	if r.config.Dir != "" {
		runDir = r.config.Dir
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
