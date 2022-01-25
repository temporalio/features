package cmd

import (
	"bytes"
	"context"
	"fmt"
	"go.temporal.io/sdk-features/harness/go/cmd"
	"go.temporal.io/server/common/log/tag"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"text/template"
)

type packageJSONDetails struct {
	PathToMainTS   string
	MetaPkgVersion string
	LocalSDK       string
}

// RunTypeScriptExternal runs the TS harness in an external process. This expects the
// server to already be started.
func (r *Runner) RunTypeScriptExternal(ctx context.Context, run *cmd.Run) error {
	// Create base dir
	tempDir, err := os.MkdirTemp(r.rootDir, "sdk-features-typescript-test-")
	if err != nil {
		return fmt.Errorf("failed creating temp dir: %w", err)
	}
	r.log.Info("Building temporary Typescript project", tag.NewStringTag("Path", tempDir))

	// Remove when done if configured to do so
	// TODO: Dedupe
	if !r.config.RetainTempDir {
		c := make(chan os.Signal)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-c
			_ = os.RemoveAll(tempDir)
			os.Exit(1)
		}()
		defer os.RemoveAll(tempDir)
	}

	harnessPath, err := filepath.Abs(filepath.Join(r.rootDir, "harness", "ts"))
	if err != nil {
		return fmt.Errorf("failed to make absolute path for TS harness: %w", err)
	}
	packageJSON, err := template.ParseFiles(filepath.Join(harnessPath, "package.json.tmpl"))
	if err != nil {
		return fmt.Errorf("failed to load package.json template: %w", err)
	}

	// Create package.json from template
	var packageJSONEvaluated bytes.Buffer
	localSDK := ""
	MetaPkgVersion := ""
	if strings.HasPrefix(r.config.Version, "/") {
		localSDK = r.config.Version
	} else {
		MetaPkgVersion = r.config.Version
	}
	err = packageJSON.Execute(&packageJSONEvaluated, packageJSONDetails{
		LocalSDK:       "file:" + localSDK,
		PathToMainTS:   "../harness/ts/main.ts",
		MetaPkgVersion: MetaPkgVersion,
	})
	if err != nil {
		return fmt.Errorf("failed build package.json template: %w", err)
	}
	err = os.WriteFile(filepath.Join(tempDir, "package.json"), packageJSONEvaluated.Bytes(), 0644)
	if err != nil {
		return fmt.Errorf("failed to write to package.json in harness: %w", err)
	}

	// Copy tsconfig
	tsConfigSrc, err := os.ReadFile(filepath.Join(harnessPath, "tsconfig.json.tmpl"))
	if err != nil {
		return fmt.Errorf("failed open tsconfig.json template: %w", err)
	}
	err = os.WriteFile(filepath.Join(tempDir, "tsconfig.json"), tsConfigSrc, 0644)
	if err != nil {
		return fmt.Errorf("failed create tsconfig.json in harness: %w", err)
	}

	// TODO: Make callback for "done with initting" to avoid timing out too early?

	// Run npm install
	npmCmd := exec.CommandContext(ctx, "npm", "install")
	npmCmd.Dir = tempDir
	npmCmd.Stdin, npmCmd.Stdout, npmCmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := npmCmd.Run(); err != nil {
		return fmt.Errorf("failed running npm install: %w", err)
	}

	// Run the harness
	runArgs := []string{"run", "start", "--",
		"--server", r.config.Server, "--namespace", r.config.Namespace}
	if localSDK != "" {
		runArgs = append(runArgs, "--node-modules-path", filepath.Join(localSDK, "node_modules"))
	}
	runArgs = append(runArgs, run.ToArgs()...)
	npmRun := exec.CommandContext(ctx, "npm", runArgs...)
	npmRun.Dir = tempDir
	npmRun.Stdin, npmRun.Stdout, npmRun.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := npmRun.Run(); err != nil {
		return fmt.Errorf("failed running ts harness: %w", err)
	}

	return nil
}
