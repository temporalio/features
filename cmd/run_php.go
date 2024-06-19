package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/temporalio/features/harness/go/cmd"
	"github.com/temporalio/features/sdkbuild"
)

// PreparePhpExternal prepares a PHP run without running it. The preparer
// config directory if present is expected to be a subdirectory name just
// beneath the root directory.
func (p *Preparer) BuildPhpProgram(ctx context.Context) (sdkbuild.Program, error) {
	p.log.Info("Building PHP project", "DirName", p.config.DirName)

	prog, err := sdkbuild.BuildPhpProgram(ctx, sdkbuild.BuildPhpProgramOptions{
		DirName: p.config.DirName,
		Version: p.config.Version,
		RootDir: p.rootDir,
	})
	if err != nil {
		p.log.Error("failed preparing: %w", err)
		return nil, fmt.Errorf("failed preparing: %w", err)
	}
	return prog, nil
}

// RunPhpExternal runs the PHP run in an external process. This expects
// the server to already be started.
func (r *Runner) RunPhpExternal(ctx context.Context, run *cmd.Run) error {
	// If program not built, build it
	if r.program == nil {
		var err error
		if r.program, err = NewPreparer(r.config.PrepareConfig).BuildPhpProgram(ctx); err != nil {
			return err
		}
	}

	// Compose RoadRunner command options
	args := append(
		[]string{
			// Namespace
			"namespace=" + r.config.Namespace,
			// Server address
			"address=" + r.config.Server,
		},
		// Features
		run.ToArgs()...,
	)
	// TLS
	if r.config.ClientCertPath != "" {
		clientCertPath, err := filepath.Abs(r.config.ClientCertPath)
		if err != nil {
			return err
		}
		args = append(args, "tls.cert="+clientCertPath)
	}
	if r.config.ClientKeyPath != "" {
		clientKeyPath, err := filepath.Abs(r.config.ClientKeyPath)
		if err != nil {
			return err
		}
		args = append(args, "tls.key="+clientKeyPath)
	}

	// r.log.Debug("ARGS", "Args", args)
	r.log.Debug("ARGS", "Args", strings.Join(args, " "))

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
