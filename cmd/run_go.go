package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"go.temporal.io/sdk-features/harness/go/cmd"
	"go.temporal.io/server/common/log/tag"
)

// PrepareGoExternal prepares a Go run without running it. The preparer config
// directory is expected to be an absolute subdirectory just beneath the root
// directory.
func (p *Preparer) PrepareGoExternal(ctx context.Context) error {
	p.log.Info("Preparing Go project", tag.NewStringTag("Path", p.config.Dir))

	// Create go.mod
	goMod := `module go.temporal.io/sdk-features-test

	go 1.17
	
	require go.temporal.io/sdk-features/features v1.0.0
	require go.temporal.io/sdk-features/harness/go v1.0.0
	
	replace go.temporal.io/sdk-features/features => ../features
	replace go.temporal.io/sdk-features/harness/go => ../harness/go
	
	replace github.com/cactus/go-statsd-client => github.com/cactus/go-statsd-client v3.2.1+incompatible`
	// If a version is specified, overwrite the SDK to use that
	if p.config.Version != "" {
		// If version does not start with a "v" we assume path
		if strings.HasPrefix(p.config.Version, "v") {
			goMod += "\nreplace go.temporal.io/sdk => go.temporal.io/sdk " + p.config.Version
		} else {
			absVersion, err := filepath.Abs(p.config.Version)
			if err != nil {
				return fmt.Errorf("version does not start with 'v' and cannot get abs dir: %w", err)
			}
			relVersion, err := filepath.Rel(p.config.Dir, absVersion)
			if err != nil {
				return fmt.Errorf("version does not start with 'v' and unable to relativize: %w", err)
			}
			goMod += "\nreplace go.temporal.io/sdk => " + filepath.ToSlash(relVersion)
		}
	}
	if err := os.WriteFile(filepath.Join(p.config.Dir, "go.mod"), []byte(goMod), 0644); err != nil {
		return fmt.Errorf("failed writing go.mod: %w", err)
	}

	// Create main.go
	mainGo := `package main

import (
	"go.temporal.io/sdk-features/harness/go/cmd"
	_ "go.temporal.io/sdk-features/features"
)

func main() {
	cmd.Execute()
}`
	if err := os.WriteFile(filepath.Join(p.config.Dir, "main.go"), []byte(mainGo), 0644); err != nil {
		return fmt.Errorf("failed writing main.go: %w", err)
	}

	// Tidy it
	goCmd := exec.CommandContext(ctx, "go", "mod", "tidy")
	goCmd.Dir = p.config.Dir
	goCmd.Stdin, goCmd.Stdout, goCmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := goCmd.Run(); err != nil {
		return fmt.Errorf("failed go mod tidy: %w", err)
	}

	// Build it
	exe := "sdk-features-test"
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	goCmdArgs := []string{"build", "-o", exe}
	for _, tag := range cmd.GoBuildTags(p.config.Version) {
		goCmdArgs = append(goCmdArgs, "-tags", tag)
	}
	goCmd = exec.CommandContext(ctx, "go", goCmdArgs...)
	goCmd.Dir = p.config.Dir
	goCmd.Stdin, goCmd.Stdout, goCmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := goCmd.Run(); err != nil {
		return fmt.Errorf("failed go build: %w", err)
	}

	return nil
}

// RunGoExternal runs the given run details in an external Go project. This
// expects the server to already be started.
func (r *Runner) RunGoExternal(ctx context.Context, run *cmd.Run) error {
	// To do this, we are going to create a separate project with the proper SDK
	// version included and a simple main.go file that executes the local runner

	// If there is not a prepared directory, create a temp directory and prepare
	if r.config.Dir == "" {
		var err error
		if r.config.Dir, err = os.MkdirTemp(r.rootDir, "sdk-features-go-test-"); err != nil {
			return fmt.Errorf("failed creating temp dir: %w", err)
		}
		r.createdTempDir = &r.config.Dir

		// Prepare the project
		if err := NewPreparer(r.config.PrepareConfig).PrepareGoExternal(ctx); err != nil {
			return err
		}
	}

	// Run it with args and features appended.
	exe := "sdk-features-test"
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	runArgs := append([]string{
		"run",
		"--server", r.config.Server,
		"--namespace", r.config.Namespace,
	}, run.ToArgs()...)
	r.log.Debug("Running Go separately", tag.NewStringsTag("Args", runArgs))
	goCmd := exec.CommandContext(ctx, filepath.Join(r.config.Dir, exe), runArgs...)
	goCmd.Dir = r.config.Dir
	goCmd.Stdin, goCmd.Stdout, goCmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := goCmd.Run(); err != nil {
		return fmt.Errorf("failed running: %w", err)
	}
	return nil
}
