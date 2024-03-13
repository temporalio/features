package cmd

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/temporalio/features/harness/go/cmd"
	"github.com/temporalio/features/sdkbuild"
)

// BuildJavaProgram prepares a Java run without running it. The preparer config
// directory if present is expected to be a subdirectory name just beneath the
// root directory.
func (p *Preparer) BuildJavaProgram(ctx context.Context, build bool) (sdkbuild.Program, error) {
	p.log.Info("Building Java project", "DirName", p.config.DirName)
	prog, err := sdkbuild.BuildJavaProgram(ctx, sdkbuild.BuildJavaProgramOptions{
		BaseDir:           p.rootDir,
		DirName:           p.config.DirName,
		Version:           p.config.Version,
		HarnessDependency: "io.temporal:features:0.1.0",
		MainClass:         "io.temporal.sdkfeatures.Main",
		Build:             build,
	})
	if err != nil {
		return nil, fmt.Errorf("failed preparing: %w", err)
	}
	return prog, nil
}

// RunJavaExternal runs the Java run in an external process. This expects the
// server to already be started.
func (r *Runner) RunJavaExternal(ctx context.Context, run *cmd.Run) error {
	// If program not built, build it
	if r.program == nil {
		var err error
		if r.program, err = NewPreparer(r.config.PrepareConfig).BuildJavaProgram(ctx, false); err != nil {
			return err
		}
	}

	// Build args
	args := []string{"--server", r.config.Server, "--namespace", r.config.Namespace}
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
	if r.config.SummaryURI != "" {
		args = append(args, "--summary-uri", r.config.SummaryURI)
	}
	if proxyControlURI := r.config.ProxyControlURI(); proxyControlURI != "" {
		args = append(args, "--proxy-control-uri", proxyControlURI)
	}
	args = append(args, run.ToArgs()...)

	// Run
	cmd, err := r.program.NewCommand(ctx, args...)
	if err == nil {
		r.log.Debug("Running Java separately", "Args", cmd.Args)
		err = cmd.Run()
	}
	if err != nil {
		return fmt.Errorf("failed running: %w", err)
	}
	return nil
}
