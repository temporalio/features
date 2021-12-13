package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"go.temporal.io/sdk-features/harness/go/cmd"
	"go.temporal.io/server/common/log/tag"
)

// RunGoExternal runs the given run details in an external Go project. This
// expects the server to already be started.
func (r *Runner) RunGoExternal(ctx context.Context, run *cmd.Run) error {
	// To do this, we are going to create a separate project with the proper SDK
	// version included and a simple main.go file that executes the local runner

	// Create base dir
	tempDir, err := os.MkdirTemp(r.rootDir, "sdk-features-go-test-")
	if err != nil {
		return fmt.Errorf("failed creating temp dir: %w", err)
	}
	r.log.Info("Building temporary Go project", tag.NewStringTag("Path", tempDir))
	// Remove when done if configured to do so
	if !r.config.RetainTempDir {
		defer os.RemoveAll(tempDir)
	}

	// Create go.mod
	goMod := `module go.temporal.io/sdk-features-test

	go 1.17
	
	require go.temporal.io/sdk-features/features v1.0.0
	require go.temporal.io/sdk-features/harness/go v1.0.0
	
	replace go.temporal.io/sdk-features/features => ../features
	replace go.temporal.io/sdk-features/harness/go => ../harness/go
	
	replace github.com/cactus/go-statsd-client => github.com/cactus/go-statsd-client v3.2.1+incompatible`
	// If a version is specified, overwrite the SDK to use that
	if r.config.Version != "" {
		goMod += "\nreplace go.temporal.io/sdk => go.temporal.io/sdk " + r.config.Version
	}
	if err := os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte(goMod), 0644); err != nil {
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
	if err := os.WriteFile(filepath.Join(tempDir, "main.go"), []byte(mainGo), 0644); err != nil {
		return fmt.Errorf("failed writing main.go: %w", err)
	}

	// Tidy it
	goCmd := exec.CommandContext(ctx, "go", "mod", "tidy")
	goCmd.Dir = tempDir
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
	for _, tag := range cmd.GoBuildTags(r.config.Version) {
		goCmdArgs = append(goCmdArgs, "-tags", tag)
	}
	goCmd = exec.CommandContext(ctx, "go", goCmdArgs...)
	goCmd.Dir = tempDir
	goCmd.Stdin, goCmd.Stdout, goCmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := goCmd.Run(); err != nil {
		return fmt.Errorf("failed go build: %w", err)
	}

	// Run it with args and features appended.
	runArgs := append([]string{
		"run",
		"--server", r.config.Server,
		"--namespace", r.config.Namespace,
	}, run.ToArgs()...)
	r.log.Debug("Running Go separately", tag.NewStringsTag("Args", runArgs))
	goCmd = exec.CommandContext(ctx, filepath.Join(tempDir, exe), runArgs...)
	goCmd.Dir = tempDir
	goCmd.Stdin, goCmd.Stdout, goCmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := goCmd.Run(); err != nil {
		return fmt.Errorf("failed running: %w", err)
	}
	return nil
}
