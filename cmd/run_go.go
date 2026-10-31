package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/temporalio/features/harness/go/cmd"
	"github.com/temporalio/features/sdkbuild"
)

// BuildGoProgram prepares a Go run without running it. The preparer config
// directory if present is expected to be a subdirectory name just beneath the
// root directory.
func (p *Preparer) BuildGoProgram(ctx context.Context) (sdkbuild.Program, error) {
	p.log.Info("Building Go project", "DirName", p.config.DirName)
	options := sdkbuild.BuildGoProgramOptions{
		BaseDir: p.rootDir,
		DirName: p.config.DirName,
		Version: p.config.Version,
		GoModContents: `module go.temporal.io/features-test

go 1.20

require github.com/temporalio/features/features v1.0.0
require github.com/temporalio/features/harness/go v1.0.0

replace github.com/temporalio/features/features => ../features
replace github.com/temporalio/features/harness/go => ../harness/go

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
	}
	if p.config.crossLanguage {
		runner := NewRunner(RunConfig{})
		runner.rootDir = p.rootDir
		features, err := runner.GlobCrossLanguageFeatures(nil)
		if err != nil {
			return nil, err
		}
		options.GoMainContents = `package main
import (
"github.com/temporalio/features/harness/go/cmd"
"github.com/temporalio/features/harness/go/harness"
`
		var registrations []string
		for i, feature := range features {
			if matches, _ := filepath.Glob(filepath.Join(p.rootDir, "features", feature.Dir, "feature.go")); len(matches) != 0 {
				options.GoMainContents += fmt.Sprintf("f%d \"github.com/temporalio/features/features/%s\"\n", i, feature.Dir)
				registrations = append(registrations, fmt.Sprintf("f%d.Feature", i))
			}
		}
		options.GoMainContents += ")\nfunc main() { harness.MustRegisterFeatures(" + strings.Join(registrations, ",") + "); cmd.Execute() }\n"
		if strings.ContainsAny(p.config.Version, `/\`) {
			sdkPath, err := filepath.Abs(p.config.Version)
			if err != nil {
				return nil, err
			}
			options.GoModContents += "\nreplace go.temporal.io/sdk/contrib/opentelemetry => " + filepath.ToSlash(filepath.Join(sdkPath, "contrib", "opentelemetry")) + "\n"
		}
	}
	prog, err := sdkbuild.BuildGoProgram(ctx, options)
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

	args := []string{
		"run",
		"--server", r.config.Server,
		"--namespace", r.config.Namespace,
		"--client-cert-path", r.config.ClientCertPath,
		"--client-key-path", r.config.ClientKeyPath,
		"--summary-uri", r.config.SummaryURI,
	}
	if r.config.CACertPath != "" {
		args = append(args, "--ca-cert-path", r.config.CACertPath)
	}
	if r.config.HTTPProxyURL != "" {
		args = append(args, "--http-proxy-url", r.config.HTTPProxyURL)
	}
	if r.config.TLSServerName != "" {
		args = append(args, "--tls-server-name", r.config.TLSServerName)
	}
	args = append(args, run.ToArgs()...)
	cmd, err := r.program.NewCommand(ctx, args...)
	if err == nil {
		applyNamespaceCapabilitiesEnv(cmd, r.config.NamespaceCapabilitiesJSON)
		r.log.Debug("Running Go separately", "Args", cmd.Args)
		err = cmd.Run()
	}
	if err != nil {
		return fmt.Errorf("failed running: %w", err)
	}
	return nil
}
