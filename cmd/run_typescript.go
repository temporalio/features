package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"go.temporal.io/sdk-features/harness/go/cmd"
	"go.temporal.io/server/common/log/tag"
)

type packageJSONDetails struct {
	PathToMainJS   string
	MetaPkgVersion string
	LocalSDK       string
}

type packageJsonTemporalVersion struct {
	Dependencies struct{ Temporalio string }
}

// PrepareTypeScriptExternal prepares a TypeScript run without running it. The
// preparer config directory is expected to be an absolute subdirectory just
// beneath the root directory.
func (p *Preparer) PrepareTypeScriptExternal(ctx context.Context) error {
	p.log.Info("Building Typescript project", tag.NewStringTag("Path", p.config.Dir))

	harnessPath, err := filepath.Abs(filepath.Join(p.rootDir, "harness", "ts"))
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
	if strings.HasPrefix(p.config.Version, "/") {
		localSDK = p.config.Version
	} else {
		if p.config.Version == "" {
			// Default to version from top-level package.json
			packageJsonBytes, err := os.ReadFile(filepath.Join(p.rootDir, "package.json"))
			if err != nil {
				return fmt.Errorf("failed reading package.json: %w", err)
			}
			verStruct := packageJsonTemporalVersion{}
			err = json.Unmarshal(packageJsonBytes, &verStruct)
			if err != nil {
				return fmt.Errorf("failed read top level package.json: %w", err)
			}
			p.config.Version = verStruct.Dependencies.Temporalio
		}
		MetaPkgVersion = p.config.Version
	}
	var maybeLocalSDK string
	if localSDK != "" {
		maybeLocalSDK = "file:" + localSDK
	}
	err = packageJSON.Execute(&packageJSONEvaluated, packageJSONDetails{
		LocalSDK:       maybeLocalSDK,
		PathToMainJS:   "./tslib/harness/ts/main.js",
		MetaPkgVersion: MetaPkgVersion,
	})
	if err != nil {
		return fmt.Errorf("failed build package.json template: %w", err)
	}
	err = os.WriteFile(filepath.Join(p.config.Dir, "package.json"), packageJSONEvaluated.Bytes(), 0644)
	if err != nil {
		return fmt.Errorf("failed to write to package.json in harness: %w", err)
	}

	// Copy tsconfig
	tsConfigSrc, err := os.ReadFile(filepath.Join(harnessPath, "tsconfig.json.tmpl"))
	if err != nil {
		return fmt.Errorf("failed open tsconfig.json template: %w", err)
	}
	err = os.WriteFile(filepath.Join(p.config.Dir, "tsconfig.json"), tsConfigSrc, 0644)
	if err != nil {
		return fmt.Errorf("failed create tsconfig.json in harness: %w", err)
	}

	// TODO: Make callback for "done with initting" to avoid timing out too early?

	// Run npm install
	npmCmd := exec.CommandContext(ctx, "npm", "install")
	npmCmd.Dir = p.config.Dir
	npmCmd.Stdin, npmCmd.Stdout, npmCmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := npmCmd.Run(); err != nil {
		return fmt.Errorf("failed running npm install: %w", err)
	}

	// Compile typescript
	npmCmd = exec.CommandContext(ctx, "npm", "run", "build")
	npmCmd.Dir = p.config.Dir
	npmCmd.Stdin, npmCmd.Stdout, npmCmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := npmCmd.Run(); err != nil {
		return fmt.Errorf("failed running npm run build: %w", err)
	}
	return nil
}

// RunTypeScriptExternal runs the TS harness in an external process. This expects the
// server to already be started.
func (r *Runner) RunTypeScriptExternal(ctx context.Context, run *cmd.Run) error {
	// If there is not a prepared directory, create a temp directory and prepare
	if r.config.Dir == "" {
		var err error
		if r.config.Dir, err = os.MkdirTemp(r.rootDir, "sdk-features-typescript-test-"); err != nil {
			return fmt.Errorf("failed creating temp dir: %w", err)
		}
		r.createdTempDir = &r.config.Dir

		// Prepare the project
		if err := NewPreparer(r.config.PrepareConfig).PrepareTypeScriptExternal(ctx); err != nil {
			return err
		}
	}

	// Run the harness
	runArgs := []string{"run", "start", "--",
		"--server", r.config.Server, "--namespace", r.config.Namespace}
	runArgs = append(runArgs, run.ToArgs()...)
	npmRun := exec.CommandContext(ctx, "npm", runArgs...)
	npmRun.Dir = r.config.Dir
	npmRun.Stdin, npmRun.Stdout, npmRun.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := npmRun.Run(); err != nil {
		return fmt.Errorf("failed running ts harness: %w", err)
	}

	return nil
}
