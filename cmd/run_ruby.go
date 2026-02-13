package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/temporalio/features/harness/go/cmd"
	"github.com/temporalio/features/sdkbuild"
)

// BuildRubyProgram prepares a Ruby run without running it. The preparer
// config directory if present is expected to be a subdirectory name just
// beneath the root directory.
func (p *Preparer) BuildRubyProgram(ctx context.Context) (sdkbuild.Program, error) {
	p.log.Info("Building Ruby project", "DirName", p.config.DirName)

	// Get version from harness/ruby/Gemfile if not present
	version := p.config.Version
	if version == "" {
		b, err := os.ReadFile(filepath.Join(p.rootDir, "harness", "ruby", "Gemfile"))
		if err != nil {
			return nil, fmt.Errorf("failed reading harness/ruby/Gemfile: %w", err)
		}
		for _, line := range strings.Split(string(b), "\n") {
			line = strings.TrimSpace(line)
			if strings.Contains(line, `"temporalio"`) {
				// Extract version from: gem "temporalio", "~> 1.2"
				parts := strings.Split(line, ",")
				if len(parts) >= 2 {
					version = strings.TrimSpace(parts[1])
					version = strings.Trim(version, `"'`)
				}
				break
			}
		}
		if version == "" {
			return nil, fmt.Errorf("version not found in harness/ruby/Gemfile")
		}
	}

	prog, err := sdkbuild.BuildRubyProgram(ctx, sdkbuild.BuildRubyProgramOptions{
		BaseDir: p.rootDir,
		DirName: p.config.DirName,
		Version: version,
	})
	if err != nil {
		return nil, fmt.Errorf("failed preparing: %w", err)
	}
	return prog, nil
}

// RunRubyExternal runs the Ruby run in an external process. This expects
// the server to already be started.
func (r *Runner) RunRubyExternal(ctx context.Context, run *cmd.Run) error {
	// If program not built, build it
	if r.program == nil {
		var err error
		if r.program, err = NewPreparer(r.config.PrepareConfig).BuildRubyProgram(ctx); err != nil {
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
	if r.config.CACertPath != "" {
		caCertPath, err := filepath.Abs(r.config.CACertPath)
		if err != nil {
			return err
		}
		args = append(args, "--ca-cert-path", caCertPath)
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

	// Run
	cmd, err := r.program.NewCommand(ctx, args...)
	if err == nil {
		r.log.Debug("Running Ruby separately", "Args", cmd.Args)
		err = cmd.Run()
	}
	if err != nil {
		return fmt.Errorf("failed running: %w", err)
	}
	return nil
}
