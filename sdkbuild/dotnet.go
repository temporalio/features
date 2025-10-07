package sdkbuild

import (
	"context"
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"io"
)

// BuildDotNetProgramOptions are options for BuildDotNetProgram.
type BuildDotNetProgramOptions struct {
	// Directory that will have a temporary directory created underneath.
	BaseDir string
	// If not set, uses default defined in dotnet.csproj. If set and contains a slash,
	// it is assumed to be a path to the base of the repo (and will have
	// a src/Temporalio/Temporalio.csproj child). Otherwise it is a NuGet version.
	Version string
	// If present, this directory is expected to exist beneath base dir. Otherwise
	// a temporary dir is created.
	DirName string
	// Required Program.cs content. If not set, no Program.cs is created (so it)
	ProgramContents string
	// Required csproj content. This should not contain a dependency on Temporalio
	// because this adds a package/project reference near the end.
	CsprojContents string
	// If present, custom writers that will capture stdout/stderr.
	Stdout io.Writer
	Stderr io.Writer
}

// DotNetProgram is a .NET-specific implementation of Program.
type DotNetProgram struct {
	dir string
}

var _ Program = (*DotNetProgram)(nil)

func BuildDotNetProgram(ctx context.Context, options BuildDotNetProgramOptions) (*DotNetProgram, error) {
	if options.BaseDir == "" {
		return nil, fmt.Errorf("base dir required")
	} else if options.ProgramContents == "" {
		return nil, fmt.Errorf("program contents required")
	} else if options.CsprojContents == "" {
		return nil, fmt.Errorf("csproj contents required")
	}

	// Create temp dir if needed that we will remove if creating is unsuccessful
	success := false
	var dir string
	if options.DirName != "" {
		dir = filepath.Join(options.BaseDir, options.DirName)
	} else {
		var err error
		dir, err = os.MkdirTemp(options.BaseDir, "program-")
		if err != nil {
			return nil, fmt.Errorf("failed making temp dir: %w", err)
		}
		defer func() {
			if !success {
				// Intentionally swallow error
				_ = os.RemoveAll(dir)
			}
		}()
	}

	versionArg := ""
	// Slash means it is a path
	if strings.ContainsAny(options.Version, `/\`) {
		// Get absolute path of csproj file
		absCsproj, err := filepath.Abs(filepath.Join(options.Version, "src/Temporalio/Temporalio.csproj"))
		if err != nil {
			return nil, fmt.Errorf("cannot make absolute path from version: %w", err)
		} else if _, err := os.Stat(absCsproj); err != nil {
			return nil, fmt.Errorf("cannot find version path of %v: %w", absCsproj, err)
		}
		// Need to build this csproj first
		cmd := exec.CommandContext(ctx, "dotnet", "build", absCsproj)
		cmd.Dir = dir
		setupCommandIO(cmd, options.Stdout, options.Stderr)
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("failed dotnet build of csproj in version: %w", err)
		}
		versionArg = `-property:TemporalioProjectReference="` + html.EscapeString(absCsproj) + `"`
	} else if options.Version != "" {
		versionArg = `-property:TemporalioVersion="` + html.EscapeString(strings.TrimPrefix(options.Version, "v")) + `"`
	}

	// Create program.csproj
	if err := os.WriteFile(filepath.Join(dir, "program.csproj"), []byte(options.CsprojContents), 0644); err != nil {
		return nil, fmt.Errorf("failed writing program.csproj: %w", err)
	}

	// Create Program.cs
	if err := os.WriteFile(filepath.Join(dir, "Program.cs"), []byte(options.ProgramContents), 0644); err != nil {
		return nil, fmt.Errorf("failed writing Program.cs: %w", err)
	}

	// Build it into build folder
	cmdArgs := []string{"build", "--output", "build"}
	if versionArg != "" {
		cmdArgs = append(cmdArgs, versionArg)
	}
	cmd := exec.CommandContext(ctx, "dotnet", cmdArgs...)
	cmd.Dir = dir
	setupCommandIO(cmd, options.Stdout, options.Stderr)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed dotnet build: %w", err)
	}

	// All good
	success = true
	return &DotNetProgram{dir}, nil
}

// DotNetProgramFromDir recreates the Go program from a Dir() result of a
// BuildDotNetProgram().
func DotNetProgramFromDir(dir string) (*DotNetProgram, error) {
	// Quick sanity check on the presence of program.csproj
	if _, err := os.Stat(filepath.Join(dir, "program.csproj")); err != nil {
		return nil, fmt.Errorf("failed finding program.csproj in dir: %w", err)
	}
	return &DotNetProgram{dir}, nil
}

// Dir is the directory to run in.
func (d *DotNetProgram) Dir() string { return d.dir }

// NewCommand makes a new command for the given args.
func (d *DotNetProgram) NewCommand(ctx context.Context, args ...string) (*exec.Cmd, error) {
	exe := "./build/program"
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Dir = d.dir
	setupCommandIO(cmd, nil, nil)
	return cmd, nil
}
