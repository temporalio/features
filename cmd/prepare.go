package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v2"
	"go.temporal.io/server/common/log"
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
	Dir     string
	Lang    string
	Version string
}

func (p *PrepareConfig) flags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name: "dir",
			Usage: "Directory name to prepare SDK for run at. " +
				"This will be relative to the SDK features directory and cannot exist yet.",
			Required:    true,
			Destination: &p.Dir,
		},
		&cli.StringFlag{
			Name:        "lang",
			Usage:       "SDK language to run ('go' or 'java' or 'ts' or 'py')",
			Required:    true,
			Destination: &p.Lang,
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
		log:     log.NewCLILogger(),
		config:  config,
		rootDir: rootDir(),
	}
}

func (p *Preparer) Prepare(ctx context.Context) error {
	var err error
	if p.config.Lang, err = normalizeLangName(p.config.Lang); err != nil {
		return err
	} else if p.config.Dir == "" {
		return fmt.Errorf("directory required")
	} else if strings.ContainsAny(p.config.Dir, `\/`) {
		return fmt.Errorf("directory must not have path separators, it is always relative to the SDK features root")
	} else if p.config.Version == "" {
		return fmt.Errorf("version required")
	}

	// Qualify the directory, error if it already exists. Since we're setting
	// things up in this temporary directory, it is unreasonable to auto-clean or
	// merge, so we just error if already there.
	p.config.Dir = filepath.Join(p.rootDir, p.config.Dir)
	if err := os.Mkdir(p.config.Dir, 0755); err != nil {
		return fmt.Errorf("failed creating directory: %w", err)
	}

	// Go
	switch p.config.Lang {
	case "go":
		return p.PrepareGoExternal(ctx)
	case "java":
		// Prepare and build
		return p.PrepareJavaExternal(ctx, true)
	case "ts":
		return p.PrepareTypeScriptExternal(ctx)
	case "py":
		return p.PreparePythonExternal(ctx)
	default:
		return fmt.Errorf("unrecognized language")
	}
}
