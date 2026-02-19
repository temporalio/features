package sdkbuild

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// BuildPythonProgramOptions are options for BuildPythonProgram.
type BuildPythonProgramOptions struct {
	// Directory that will have a temporary directory created underneath. This
	// should be a Poetry project with a pyproject.toml.
	BaseDir string
	// Required version. If it contains a slash it is assumed to be a path with
	// a single wheel in the dist directory. Otherwise it is a specific version
	// (with leading "v" is trimmed if present).
	Version string
	// If specified, takes precedence over Version. Is a PEP 508 requirement string, like
	// `temporalio>=1.13.0,<2`.
	VersionFromPyProj string
	// If present, this directory is expected to exist beneath base dir. Otherwise
	// a temporary dir is created.
	DirName string
	// If present, applied to build commands before run. May be called multiple
	// times for a single build.
	ApplyToCommand func(context.Context, *exec.Cmd) error
	// If present, custom writers that will capture stdout/stderr.
	Stdout io.Writer
	Stderr io.Writer
}

// PythonProgram is a Python-specific implementation of Program.
type PythonProgram struct {
	dir string
}

var _ Program = (*PythonProgram)(nil)

// BuildPythonProgram builds a Python program. If completed successfully, this
// can be stored and re-obtained via PythonProgramFromDir() with the Dir() value
// (but the entire BaseDir must be present too).
func BuildPythonProgram(ctx context.Context, options BuildPythonProgramOptions) (*PythonProgram, error) {
	if options.BaseDir == "" {
		return nil, fmt.Errorf("base dir required")
	} else if options.Version == "" {
		return nil, fmt.Errorf("version required")
	} else if _, err := os.Stat(filepath.Join(options.BaseDir, "pyproject.toml")); err != nil {
		return nil, fmt.Errorf("failed finding pyproject.toml in base dir: %w", err)
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

	executeCommand := func(name string, args ...string) error {
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Dir = dir
		setupCommandIO(cmd, options.Stdout, options.Stderr)
		if options.ApplyToCommand != nil {
			if err := options.ApplyToCommand(ctx, cmd); err != nil {
				return err
			}
		}
		return cmd.Run()
	}

	pyProjectTOML := `
[project]
name = "python-program-` + filepath.Base(dir) + `"
version = "0.1.0"
description = "Temporal SDK Python Test"
authors = [{ name = "Temporal Technologies Inc", email = "sdk@temporal.io" }]
requires-python = "~=3.10"
`
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(pyProjectTOML), 0644); err != nil {
		return nil, fmt.Errorf("failed writing pyproject.toml: %w", err)
	}

	if options.VersionFromPyProj != "" {
		executeCommand("uv", "add", options.VersionFromPyProj)
	} else if strings.ContainsAny(options.Version, `/\`) {
		// It's a path; install from wheel
		wheel, err := getWheel(ctx, options.Version, options.Stdout, options.Stderr)
		if err != nil {
			return nil, err
		}
		executeCommand("uv", "add", wheel)
	} else {
		executeCommand("uv", "add", fmt.Sprintf("temporalio==%s", strings.TrimPrefix(options.Version, "v")))
	}
	// Add the `features` python package
	executeCommand("uv", "add", "--editable", "../")

	if err := executeCommand("uv", "sync"); err != nil {
		return nil, fmt.Errorf("failed installing: %w", err)
	}
	// Install mypy for type checking
	if err := executeCommand("uv", "add", "--dev", "mypy"); err != nil {
		return nil, fmt.Errorf("failed installing mypy: %w", err)
	}
	if err := executeCommand("uv", "run", "mypy", "--explicit-package-bases", "--namespace-packages", "../"); err != nil {
		return nil, fmt.Errorf("failed type checking: %w", err)
	}

	success = true
	return &PythonProgram{dir}, nil
}

func getWheel(ctx context.Context, version string, stdout, stderr io.Writer) (string, error) {
	// We expect a dist/ directory with a single whl file present
	sdkPath, err := filepath.Abs(version)
	if err != nil {
		return "", fmt.Errorf("unable to make sdk path absolute: %w", err)
	}
	triedBuilding := false

getWheels:
	wheels, err := filepath.Glob(filepath.Join(sdkPath, "dist/*.whl"))
	if err != nil {
		return "", fmt.Errorf("failed glob wheel lookup: %w", err)
	} else if len(wheels) == 0 && !triedBuilding {
		triedBuilding = true
		// Try to build the project
		cmd := exec.CommandContext(ctx, "uv", "sync")
		cmd.Dir = sdkPath
		setupCommandIO(cmd, stdout, stderr)
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("problem installing deps when building sdk by path: %w", err)
		}
		cmd = exec.CommandContext(ctx, "uv", "build")
		cmd.Dir = sdkPath
		setupCommandIO(cmd, stdout, stderr)
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("problem building sdk by path: %w", err)
		}
		goto getWheels
	} else if len(wheels) != 1 {
		return "", fmt.Errorf("expected single dist wheel, found %v - consider cleaning dist dir", wheels)
	}

	absWheel, err := filepath.Abs(wheels[0])
	if err != nil {
		return "", fmt.Errorf("unable to make wheel path absolute: %w", err)
	}
	// There's a strange bug in Poetry or somewhere deeper where, on Windows,
	// the single drive letter has to be capitalized
	if runtime.GOOS == "windows" && absWheel[1] == ':' {
		absWheel = strings.ToUpper(absWheel[:1]) + absWheel[1:]
	}
	return absWheel, nil
}

// PythonProgramFromDir recreates the Python program from a Dir() result of a
// BuildPythonProgram(). Note, the base directory of dir when it was built must
// also be present.
func PythonProgramFromDir(dir string) (*PythonProgram, error) {
	// Quick sanity check on the presence of pyproject.toml here _and_ in base
	if _, err := os.Stat(filepath.Join(dir, "pyproject.toml")); err != nil {
		return nil, fmt.Errorf("failed finding pyproject.toml in dir: %w", err)
	} else if _, err := os.Stat(filepath.Join(dir, "../pyproject.toml")); err != nil {
		return nil, fmt.Errorf("failed finding pyproject.toml in base dir: %w", err)
	}
	return &PythonProgram{dir}, nil
}

// Dir is the directory to run in.
func (p *PythonProgram) Dir() string { return p.dir }

// NewCommand makes a new uv command. The first argument needs to be the
// name of the module.
func (p *PythonProgram) NewCommand(ctx context.Context, args ...string) (*exec.Cmd, error) {
	args = append([]string{"run", "python", "-m"}, args...)
	cmd := exec.CommandContext(ctx, "uv", args...)
	cmd.Dir = p.dir
	setupCommandIO(cmd, nil, nil)
	return cmd, nil
}
