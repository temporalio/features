package sdkbuild

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// BuildTypeScriptProgramOptions are options for BuildTypeScriptProgram.
type BuildTypeScriptProgramOptions struct {
	// Directory that will have a temporary directory created underneath.
	BaseDir string
	// Required version. If it contains a "/" it is assumed to be a path with a
	// package.json. Otherwise it is a specific version (with leading "v" is
	// trimmed if present).
	Version string
	// Required set of paths to include in tsconfig.json paths for the harness.
	// The paths should be relative to one-directory beneath BaseDir.
	HarnessPaths map[string][]string
	// If present, this directory is expected to exist beneath base dir. Otherwise
	// a temporary dir is created.
	DirName string
	// If present, applied to build commands before run. May be called multiple
	// times for a single build.
	ApplyToCommand func(context.Context, *exec.Cmd) error
}

// TypeScriptProgram is a TypeScript-specific implementation of Program.
type TypeScriptProgram struct {
	dir string
}

var _ Program = (*TypeScriptProgram)(nil)

// BuildTypeScriptProgram builds a TypeScript program. If completed
// successfully, this can be stored and re-obtained via
// TypeScriptProgramFromDir() with the Dir() value (but the entire BaseDir must
// be present too).
func BuildTypeScriptProgram(ctx context.Context, options BuildTypeScriptProgramOptions) (*TypeScriptProgram, error) {
	if options.BaseDir == "" {
		return nil, fmt.Errorf("base dir required")
	} else if options.Version == "" {
		return nil, fmt.Errorf("version required")
	} else if len(options.HarnessPaths) == 0 {
		return nil, fmt.Errorf("at least one harness path required")
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

	// Create package JSON
	var packageJSONDepStr string
	if strings.Contains(options.Version, "/") {
		if _, err := os.Stat(filepath.Join(options.Version, "package.json")); err != nil {
			return nil, fmt.Errorf("failed finding package.json in version dir: %w", err)
		}
		localPath := "file:" + options.Version
		pkgs := []string{"activity", "client", "common", "internal-workflow-common",
			"internal-non-workflow-common", "proto", "worker", "workflow"}
		for _, pkg := range pkgs {
			packageJSONDepStr += fmt.Sprintf(`"@temporalio/%v": "%v/packages/%v",`, pkg, localPath, pkg)
			packageJSONDepStr += "\n    "
		}
	} else {
		packageJSONDepStr = `"temporalio": "` + strings.TrimPrefix(options.Version, "v") + "\",\n    "
	}
	packageJSON := `{
  "name": "program",
  "private": true,
  "scripts": {
    "build": "tsc --build"
  },
  "dependencies": {
    ` + packageJSONDepStr + `
    "commander": "^8.3.0",
    "uuid": "^8.3.2"
  },
  "devDependencies": {
    "@tsconfig/node16": "^1.0.0",
    "@types/node": "^16.11.59",
    "@types/uuid": "^8.3.4",
    "tsconfig-paths": "^3.12.0",
    "typescript": "^4.4.2"
  }
}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(packageJSON), 0644); err != nil {
		return nil, fmt.Errorf("failed writing package.json: %w", err)
	}

	// Create tsconfig
	var tsConfigPathStr string
	for name, paths := range options.HarnessPaths {
		if len(paths) == 0 {
			return nil, fmt.Errorf("harness path slice is empty")
		}
		tsConfigPathStr += fmt.Sprintf("%q: [", name)
		for i, path := range paths {
			if i > 0 {
				tsConfigPathStr += ", "
			}
			tsConfigPathStr += strconv.Quote(path)
		}
		tsConfigPathStr += "],\n      "
	}
	tsConfig := `{
  "extends": "@tsconfig/node16/tsconfig.json",
  "version": "4.4.2",
  "compilerOptions": {
    "baseUrl": ".",
    "outDir": "./tslib",
    "rootDirs": ["../", "."],
    "paths": {
      ` + tsConfigPathStr + `
      "*": ["node_modules/*", "node_modules/@types/*"]
    },
    "typeRoots": ["node_modules/@types"],
    "module": "commonjs",
    "moduleResolution": "node",
    "sourceMap": true,
    "resolveJsonModule": true
  },
  "include": ["../features/**/*.ts", "../harness/ts/**/*.ts"],
  "exclude": ["../node_modules", "../harness/go", "../harness/java"],
}`
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(tsConfig), 0644); err != nil {
		return nil, fmt.Errorf("failed writing tsconfig.json: %w", err)
	}

	// Install
	cmd := exec.CommandContext(ctx, "npm", "install")
	cmd.Dir = dir
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if options.ApplyToCommand != nil {
		if err := options.ApplyToCommand(ctx, cmd); err != nil {
			return nil, err
		}
	}
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed installing: %w", err)
	}

	// Compile
	cmd = exec.CommandContext(ctx, "npm", "run", "build")
	cmd.Dir = dir
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if options.ApplyToCommand != nil {
		if err := options.ApplyToCommand(ctx, cmd); err != nil {
			return nil, err
		}
	}
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed compiling: %w", err)
	}

	success = true
	return &TypeScriptProgram{dir}, nil
}

// TypeScriptProgramFromDir recreates the TypeScript program from a Dir() result
// of a BuildTypeScriptProgram(). Note, the base directory of dir when it was
// built must also be present.
func TypeScriptProgramFromDir(dir string) (*TypeScriptProgram, error) {
	// Quick sanity check on the presence of package.json here
	if _, err := os.Stat(filepath.Join(dir, "package.json")); err != nil {
		return nil, fmt.Errorf("failed finding package.json in dir: %w", err)
	}
	return &TypeScriptProgram{dir}, nil
}

// Dir is the directory to run in.
func (t *TypeScriptProgram) Dir() string { return t.dir }

// NewCommand makes a new Node command. The first argument needs to be the name
// of the script.
func (t *TypeScriptProgram) NewCommand(ctx context.Context, args ...string) (*exec.Cmd, error) {
	args = append([]string{"-r", "tsconfig-paths/register"}, args...)
	cmd := exec.CommandContext(ctx, "node", args...)
	cmd.Dir = t.dir
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd, nil
}
