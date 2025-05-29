package cmd

import (
	"context"
	"fmt"
	"github.com/temporalio/features/harness/go/cmd"
	"github.com/temporalio/features/sdkbuild"
)

// BuildDotNetProgram prepares a .NET run without running it. The preparer
// config directory if present is expected to be a subdirectory name just
// beneath the root directory.
func (p *Preparer) BuildDotNetProgram(ctx context.Context) (sdkbuild.Program, error) {
	p.log.Info("Building .NET project", "DirName", p.config.DirName)
	prog, err := sdkbuild.BuildDotNetProgram(ctx, sdkbuild.BuildDotNetProgramOptions{
		BaseDir:         p.rootDir,
		DirName:         p.config.DirName,
		Version:         p.config.Version,
		ProgramContents: `await Temporalio.Features.Harness.App.RunAsync(args);`,
		CsprojContents: `<Project Sdk="Microsoft.NET.Sdk">
			<PropertyGroup>
				<OutputType>Exe</OutputType>
				<TargetFramework>net8.0</TargetFramework>
			</PropertyGroup>
			<ItemGroup>
				<ProjectReference Include="..\dotnet.csproj" />
			</ItemGroup>
		</Project>`,
	})
	if err != nil {
		return nil, fmt.Errorf("failed preparing: %w", err)
	}
	return prog, nil
}

func (r *Runner) RunDotNetExternal(ctx context.Context, run *cmd.Run) error {
	// If program not built, build it
	if r.program == nil {
		var err error
		if r.program, err = NewPreparer(r.config.PrepareConfig).BuildDotNetProgram(ctx); err != nil {
			return err
		}
	}

	args := []string{"--server", r.config.Server, "--namespace", r.config.Namespace}
	if r.config.ClientCertPath != "" {
		args = append(args, "--client-cert-path", r.config.ClientCertPath, "--client-key-path", r.config.ClientKeyPath)
	}
	if r.config.HTTPProxyURL != "" {
		args = append(args, "--http-proxy-url", r.config.HTTPProxyURL)
	}
	args = append(args, run.ToArgs()...)
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
