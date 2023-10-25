package cmd

import (
	"context"
	"fmt"

	"github.com/temporalio/features/harness/go/cmd"
	"github.com/temporalio/features/sdkbuild"
)

// BuildGoProgram prepares a Go run without running it. The preparer config
// directory if present is expected to be a subdirectory name just beneath the
// root directory.
func (p *Preparer) BuildGoProgram(ctx context.Context) (sdkbuild.Program, error) {
	p.log.Info("Building Go project", "DirName", p.config.DirName)
	prog, err := sdkbuild.BuildGoProgram(ctx, sdkbuild.BuildGoProgramOptions{
		BaseDir: p.rootDir,
		DirName: p.config.DirName,
		Version: p.config.Version,
		GoModContents: `module go.temporal.io/features-test

go 1.20

require github.com/temporalio/features/features v1.0.0
require github.com/temporalio/features/harness/go v1.0.0

replace github.com/temporalio/features/features => ../features
replace github.com/temporalio/features/harness/go => ../harness/go
replace go.temporal.io/api => github.com/tdeebswihart/temporal-api-go nomo-gogo

replace github.com/cactus/go-statsd-client => github.com/cactus/go-statsd-client v3.2.1+incompatible`,
		GoMainContents: `package main

import (
	"github.com/temporalio/features/harness/go/cmd"
	_ "github.com/temporalio/features/features"
)

func main() {
	cmd.Execute()
}`,
		GoBuildTags: cmd.GoBuildTags(p.config.Version),
	})
	if err != nil {
		return nil, fmt.Errorf("failed preparing: %w", err)
	}
	return prog, nil
}

// RunGoExternal runs the given run details in an external Go project. This
// expects the server to already be started.
func (r *Runner) RunGoExternal(ctx context.Context, run *cmd.Run) error {
	// If program not built, build it
	if r.program == nil {
		var err error
		if r.program, err = NewPreparer(r.config.PrepareConfig).BuildGoProgram(ctx); err != nil {
			return err
		}
	}

	args := append([]string{
		"run",
		"--server", r.config.Server,
		"--namespace", r.config.Namespace,
		"--client-cert-path", r.config.ClientCertPath,
		"--client-key-path", r.config.ClientKeyPath,
		"--summary-uri", r.config.SummaryURI,
	}, run.ToArgs()...)
	cmd, err := r.program.NewCommand(ctx, args...)
	if err == nil {
		r.log.Debug("Running Go separately", "Args", cmd.Args)
		err = cmd.Run()
	}
	if err != nil {
		return fmt.Errorf("failed running: %w", err)
	}
	return nil
}
