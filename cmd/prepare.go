package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/temporalio/features/harness/go/harness"
	"github.com/urfave/cli/v2"
	"go.temporal.io/sdk/log"
)

func prepareCmd() *cli.Command {
	var config PrepareConfig
	return &cli.Command{
		Name:  "prepare",
		Usage: "prepare an SDK for execution",
		Flags: config.flags(),
		Action: func(ctx *cli.Context) error {
			return NewPreparer(config).Prepare(ctx.Context)
		},
	}
}

// PrepareConfig is configuration for NewPreparer.
type PrepareConfig struct {
	DirName string
	Lang    string
	Version string
}

func (p *PrepareConfig) flags() []cli.Flag {
	return []cli.Flag{
		langFlag(&p.Lang),
		&cli.StringFlag{
			Name: "dir",
			Usage: "Directory name to prepare SDK for run at. " +
				"This will be relative to the SDK features directory and cannot exist yet.",
			Required:    true,
			Destination: &p.DirName,
		},
		&cli.StringFlag{
			Name:        "version",
			Usage:       "SDK language version to run. Most languages support versions as paths.",
			Required:    true,
			Destination: &p.Version,
		},
	}
}

type Preparer struct {
	log     log.Logger
	config  PrepareConfig
	rootDir string
}

func NewPreparer(config PrepareConfig) *Preparer {
	return &Preparer{
		// TODO(cretz): Configurable logger
		log:     harness.NewCLILogger(),
		config:  config,
		rootDir: rootDir(),
	}
}

func (p *Preparer) Prepare(ctx context.Context) error {
	var err error
	if p.config.Lang, err = normalizeLangName(p.config.Lang); err != nil {
		return err
	} else if p.config.DirName == "" {
		return fmt.Errorf("directory required")
	} else if strings.ContainsAny(p.config.DirName, `\/`) {
		return fmt.Errorf("directory must not have path separators, it is always relative to the SDK features root")
	} else if p.config.Version == "" {
		return fmt.Errorf("version required")
	}

	// Try to create dir or error if already exists.
	if err := os.Mkdir(filepath.Join(p.rootDir, p.config.DirName), 0755); err != nil {
		return fmt.Errorf("failed creating directory: %w", err)
	}

	// Go
	switch p.config.Lang {
	case "go":
		_, err = p.BuildGoProgram(ctx)
	case "java":
		_, err = p.BuildJavaProgram(ctx, true)
	case "ts":
		_, err = p.BuildTypeScriptProgram(ctx)
	case "py":
		_, err = p.BuildPythonProgram(ctx)
	default:
		err = fmt.Errorf("unrecognized language")
	}
	return err
}
