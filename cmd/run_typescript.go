package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/temporalio/features/harness/go/cmd"
	"github.com/temporalio/features/sdkbuild"
)

// BuildTypeScriptProgram prepares a TypeScript run without running it. The
// preparer config directory if present is expected to be a subdirectory name
// just beneath the root directory.
func (p *Preparer) BuildTypeScriptProgram(ctx context.Context) (sdkbuild.Program, error) {
	p.log.Info("Building TypeScript project", "DirName", p.config.DirName)

	// Get version from package.json if not present
	version := p.config.Version
	if version == "" {
		verStruct := struct {
			Dependencies map[string]string `json:"dependencies"`
		}{}
		if b, err := os.ReadFile(filepath.Join(p.rootDir, "package.json")); err != nil {
			return nil, fmt.Errorf("failed reading package.json: %w", err)
		} else if err := json.Unmarshal(b, &verStruct); err != nil {
			return nil, fmt.Errorf("failed read top level package.json: %w", err)
		} else if version = verStruct.Dependencies["@temporalio/client"]; version == "" {
			return nil, fmt.Errorf("version not found in package.json")
		}
	}

	prog, err := sdkbuild.BuildTypeScriptProgram(ctx, sdkbuild.BuildTypeScriptProgramOptions{
		BaseDir: p.rootDir,
		DirName: p.config.DirName,
		Version: version,
		TSConfigPaths: map[string][]string{
			"@temporalio/harness": {"./tslib/harness/ts/harness.js", "../harness/ts/harness.ts"},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed preparing: %w", err)
	}
	return prog, nil
}

// RunTypeScriptExternal runs the TS harness in an external process. This expects the
// server to already be started.
func (r *Runner) RunTypeScriptExternal(ctx context.Context, run *cmd.Run) error {
	// If program not built, build it
	if r.program == nil {
		var err error
		if r.program, err = NewPreparer(r.config.PrepareConfig).BuildTypeScriptProgram(ctx); err != nil {
			return err
		}
	}

	// Build args
	args := make([]string, 0, 64)
	args = append(args, "./tslib/harness/ts/main.js")
	args, err := r.config.appendFlags(args)
	if err != nil {
		return err
	}
	args = append(args, run.ToArgs()...)

	// Run
	cmd, err := r.program.NewCommand(ctx, args...)
	if err == nil {
		r.log.Debug("Running TypeScript separately", "Args", cmd.Args)
		err = cmd.Run()
	}
	if err != nil {
		return fmt.Errorf("failed running: %w", err)
	}
	return nil
}
