package cmd

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/temporalio/features/harness/go/cmd"
	"github.com/temporalio/features/sdkbuild"
)

// BuildRustProgram prepares a Rust run without running it.
func (p *Preparer) BuildRustProgram(ctx context.Context) (sdkbuild.Program, error) {
	p.log.Info("Building Rust project", "DirName", p.config.DirName)
	featuresDir := filepath.Join(p.rootDir, "features")
	var features []sdkbuild.RustFeature
	err := filepath.WalkDir(featuresDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || entry.Name() != "feature.rs" {
			return nil
		}
		relDir, err := filepath.Rel(featuresDir, filepath.Dir(path))
		if err != nil {
			return err
		}
		features = append(features, sdkbuild.RustFeature{
			Name: filepath.ToSlash(relDir),
			Path: path,
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed finding Rust features: %w", err)
	}

	prog, err := sdkbuild.BuildRustProgram(ctx, sdkbuild.BuildRustProgramOptions{
		BaseDir:    p.rootDir,
		HarnessDir: filepath.Join(p.rootDir, "harness", "rust"),
		DirName:    p.config.DirName,
		Version:    p.config.Version,
		Features:   features,
	})
	if err != nil {
		return nil, fmt.Errorf("failed preparing: %w", err)
	}
	return prog, nil
}

// RunRustExternal runs the Rust harness in an external process.
func (r *Runner) RunRustExternal(ctx context.Context, run *cmd.Run) error {
	if r.program == nil {
		var err error
		if r.program, err = NewPreparer(r.config.PrepareConfig).BuildRustProgram(ctx); err != nil {
			return err
		}
	}

	args := []string{"--server", r.config.Server, "--namespace", r.config.Namespace}
	if r.config.ClientCertPath != "" {
		args = append(args, "--client-cert-path", r.config.ClientCertPath)
	}
	if r.config.ClientKeyPath != "" {
		args = append(args, "--client-key-path", r.config.ClientKeyPath)
	}
	if r.config.CACertPath != "" {
		args = append(args, "--ca-cert-path", r.config.CACertPath)
	}
	if r.config.TLSServerName != "" {
		args = append(args, "--tls-server-name", r.config.TLSServerName)
	}
	if r.config.HTTPProxyURL != "" {
		args = append(args, "--http-proxy-url", r.config.HTTPProxyURL)
	}
	if r.config.SummaryURI != "" {
		args = append(args, "--summary-uri", r.config.SummaryURI)
	}
	args = append(args, run.ToArgs()...)

	command, err := r.program.NewCommand(ctx, args...)
	if err == nil {
		applyNamespaceCapabilitiesEnv(command, r.config.NamespaceCapabilitiesJSON)
		r.log.Debug("Running Rust separately", "Args", command.Args)
		err = command.Run()
	}
	if err != nil {
		return fmt.Errorf("failed running: %w", err)
	}
	return nil
}
