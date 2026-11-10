package cmd

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/temporalio/features/harness/go/cmd"
	"github.com/temporalio/features/sdkbuild"
)

// BuildRustProgram copies and builds the Rust harness package.
func (p *Preparer) BuildRustProgram(ctx context.Context) (sdkbuild.Program, error) {
	p.log.Info("Building Rust project", "DirName", p.config.DirName)
	program, err := sdkbuild.BuildRustProgram(ctx, sdkbuild.BuildRustProgramOptions{
		BaseDir:    p.rootDir,
		DirName:    p.config.DirName,
		Version:    p.config.Version,
		PackageDir: filepath.Join(p.rootDir, "harness", "rust"),
	})
	if err != nil {
		return nil, fmt.Errorf("failed preparing Rust: %w", err)
	}
	return program, nil
}

// RunRustExternal runs a prepared native executable against the selected server.
func (r *Runner) RunRustExternal(ctx context.Context, run *cmd.Run) error {
	if r.program == nil {
		preparer := NewPreparer(r.config.PrepareConfig)
		var err error
		if r.program, err = preparer.BuildRustProgram(ctx); err != nil {
			return err
		}
	}
	args := []string{"--server", r.config.Server, "--namespace", r.config.Namespace}
	for _, flag := range []struct{ name, value string }{
		{"--client-cert-path", r.config.ClientCertPath},
		{"--client-key-path", r.config.ClientKeyPath},
		{"--ca-cert-path", r.config.CACertPath},
	} {
		if flag.value != "" {
			path, err := filepath.Abs(flag.value)
			if err != nil {
				return err
			}
			args = append(args, flag.name, path)
		}
	}
	for _, flag := range []struct{ name, value string }{
		{"--tls-server-name", r.config.TLSServerName},
		{"--http-proxy-url", r.config.HTTPProxyURL},
		{"--summary-uri", r.config.SummaryURI},
	} {
		if flag.value != "" {
			args = append(args, flag.name, flag.value)
		}
	}
	if r.config.DisableHistoryCheck {
		args = append(args, "--no-history-check")
	}
	args = append(args, run.ToArgs()...)
	command, err := r.program.NewCommand(ctx, args...)
	if err == nil {
		applyNamespaceCapabilitiesEnv(command, r.config.NamespaceCapabilitiesJSON)
		r.log.Debug("Running Rust separately", "Args", command.Args)
		err = command.Run()
	}
	if err != nil {
		return fmt.Errorf("failed running Rust: %w", err)
	}
	return nil
}
