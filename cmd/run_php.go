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

// PreparePhpExternal prepares a PHP run without running it. The preparer
// config directory if present is expected to be a subdirectory name just
// beneath the root directory.
func (p *Preparer) BuildPhpProgram(ctx context.Context) (sdkbuild.Program, error) {
	p.log.Info("Building PHP project", "DirName", p.config.DirName)

	if p.config.DirName == "" {
		p.config.DirName = filepath.Join(p.config.DirName, "harness", "php")
	}

	// Get version from composer.json if not present
	version := p.config.Version
	if version == "" {
		verStruct := struct {
			Dependencies map[string]string `json:"require"`
		}{}
		if b, err := os.ReadFile(filepath.Join(p.rootDir, "composer.json")); err != nil {
			return nil, fmt.Errorf("failed reading composer.json: %w", err)
		} else if err := json.Unmarshal(b, &verStruct); err != nil {
			return nil, fmt.Errorf("failed read top level composer.json: %w", err)
		} else if version = verStruct.Dependencies["temporal/sdk"]; version == "" {
			return nil, fmt.Errorf("version not found in composer.json")
		}
	}

	prog, err := sdkbuild.BuildPhpProgram(ctx, sdkbuild.BuildPhpProgramOptions{
		BaseDir: p.rootDir,
		DirName: p.config.DirName,
		Version: version,
	})
	if err != nil {
		p.log.Error("failed preparing: %w", err)
		return nil, fmt.Errorf("failed preparing: %w", err)
	}
	return prog, nil
}

// RunPhpExternal runs the PHP run in an external process. This expects
// the server to already be started.
// todo
func (r *Runner) RunPhpExternal(ctx context.Context, run *cmd.Run) error {
	// If program not built, build it
	if r.program == nil {
		var err error
		if r.program, err = NewPreparer(r.config.PrepareConfig).BuildPhpProgram(ctx); err != nil {
			return err
		}
	}

	// Build args
	args := []string{"harness.Php.main", "--server", r.config.Server, "--namespace", r.config.Namespace}
	if r.config.ClientCertPath != "" {
		clientCertPath, err := filepath.Abs(r.config.ClientCertPath)
		if err != nil {
			return err
		}
		args = append(args, "--client-cert-path", clientCertPath)
	}
	if r.config.ClientKeyPath != "" {
		clientKeyPath, err := filepath.Abs(r.config.ClientKeyPath)
		if err != nil {
			return err
		}
		args = append(args, "--client-key-path", clientKeyPath)
	}
	args = append(args, run.ToArgs()...)

	// Run
	cmd, err := r.program.NewCommand(ctx, args...)
	if err == nil {
		r.log.Debug("Running PHP separately", "Args", cmd.Args)
		err = cmd.Run()
	}
	if err != nil {
		return fmt.Errorf("failed running: %w", err)
	}
	return nil
}
