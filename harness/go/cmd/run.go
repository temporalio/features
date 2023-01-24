package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v2"
	"go.temporal.io/features/harness/go/harness"
	"go.temporal.io/sdk/log"
	"go.uber.org/zap"
)

func runCmd() *cli.Command {
	var config RunConfig
	return &cli.Command{
		Name:  "run",
		Usage: "run a test or set of tests for Go",
		Flags: config.flags(),
		Action: func(ctx *cli.Context) error {
			var run Run
			if err := run.FromArgs(ctx.Args().Slice()); err != nil {
				return err
			}
			runner := NewRunner(config)
			err := runner.Run(ctx.Context, &run)
			if config.StatsPath != "" {
				bytes, err := json.Marshal(runner.Stats)
				if err != nil {
					return err
				}
				if err := os.WriteFile(config.StatsPath, bytes, 0644); err != nil {
					return err
				}
			}
			return err
		},
	}
}

// Run represents a full set of features to run.
type Run struct {
	Features []RunFeature
}

// ToArgs converts this to a fixed string set of arguments.
func (r *Run) ToArgs() []string {
	ret := make([]string, len(r.Features))
	for i, feature := range r.Features {
		ret[i] = feature.Dir + ":" + feature.TaskQueue
	}
	return ret
}

// FromArgs converts the given arguments to features to run.
func (r *Run) FromArgs(args []string) error {
	for _, arg := range args {
		colonIndex := strings.Index(arg, ":")
		if colonIndex == -1 {
			return fmt.Errorf("feature %v missing task queue", arg)
		}
		r.Features = append(r.Features, RunFeature{Dir: arg[:colonIndex], TaskQueue: arg[colonIndex+1:]})
	}
	return nil
}

// RunFeature is a feature to run.
type RunFeature struct {
	Dir       string
	TaskQueue string
	Config    RunFeatureConfig
}

// RunFeatureConfig is config from .config.json.
type RunFeatureConfig struct {
	NoWorkflow bool               `json:"noWorkflow"`
	Go         RunFeatureConfigGo `json:"go"`
}

// RunFeatureConfigGo is go-specific configuration in the JSON file.
type RunFeatureConfigGo struct {
	MinVersion string `json:"minVersion"`
}

// RunConfig is configuration for NewRunner.
type RunConfig struct {
	Server         string
	Namespace      string
	ClientCertPath string
	ClientKeyPath  string
	StatsPath      string
}

func (r *RunConfig) flags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:        "server",
			Usage:       "The host:port of the server (default is to create ephemeral in-memory server)",
			Destination: &r.Server,
		},
		&cli.StringFlag{
			Name:        "namespace",
			Usage:       "The namespace to use (default is random)",
			Destination: &r.Namespace,
		},
		&cli.StringFlag{
			Name:        "client-cert-path",
			Usage:       "Path of TLS client cert to use (optional)",
			Destination: &r.ClientCertPath,
		},
		&cli.StringFlag{
			Name:        "client-key-path",
			Usage:       "Path of TLS client key to use (optional)",
			Destination: &r.ClientKeyPath,
		},
		&cli.StringFlag{
			Name:        "stats-output",
			Usage:       "Path to output run stats",
			Destination: &r.StatsPath,
		},
	}
}

// Stats is used to record run statistics
type Stats struct {
	// Skipped features
	Skipped []string `json:"skipped"`
	// Passed features
	Passed []string `json:"passed"`
	// Failed features
	Failed []string `json:"failed"`
}

// Runner is a runner that can run Go features.
type Runner struct {
	log    log.Logger
	config RunConfig
	Stats  Stats
}

// NewRunner creates a new runner from the given config.
func NewRunner(config RunConfig) *Runner {
	// TODO(cretz): Configurable logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	return &Runner{
		log:    harness.NewZapLogger(logger.Sugar()),
		config: config,
	}
}

// Run runs all the given features.
func (r *Runner) Run(ctx context.Context, run *Run) error {
	// Run the features
	// TODO(cretz): Concurrent with log capturing
	if len(run.Features) == 0 {
		return fmt.Errorf("no features to run")
	}
	allFeatures := harness.RegisteredFeatures()
	for _, runFeature := range run.Features {
		// Find the feature
		var feature *harness.PreparedFeature
		for _, maybeFeature := range allFeatures {
			if maybeFeature.Dir == runFeature.Dir {
				feature = maybeFeature
				break
			}
		}
		if feature == nil {
			return fmt.Errorf("feature %v not found, did you add it to features.go?", runFeature.Dir)
		} else if feature.SkipReason != "" {
			r.Stats.Skipped = append(r.Stats.Skipped, feature.Dir)
			r.log.Warn("Skipping feature", "Feature", feature.Dir, "Reason", feature.SkipReason)
			continue
		}
		runnerConfig := harness.RunnerConfig{
			ServerHostPort: r.config.Server,
			Namespace:      r.config.Namespace,
			ClientCertPath: r.config.ClientCertPath,
			ClientKeyPath:  r.config.ClientKeyPath,
			TaskQueue:      runFeature.TaskQueue,
			Log:            r.log,
		}
		if err := r.runFeature(ctx, runnerConfig, feature); err != nil {
			var skippedErr harness.SkippedError
			if errors.As(err, &skippedErr) {
				r.Stats.Skipped = append(r.Stats.Skipped, feature.Dir)
				r.log.Warn("Skipping feature", "Feature", feature.Dir, "Reason", skippedErr.Reason)
			} else {
				r.Stats.Failed = append(r.Stats.Failed, feature.Dir)
				r.log.Error("Feature failed", "Feature", feature.Dir, "error", err)
			}
		} else {
			r.Stats.Passed = append(r.Stats.Passed, feature.Dir)
		}

	}
	r.log.Info("Run completed", "passed", len(r.Stats.Passed), "skipped", len(r.Stats.Skipped), "failed",
		len(r.Stats.Failed))

	if len(r.Stats.Failed) > 0 {
		return fmt.Errorf("%v failure(s) reported: %v", len(r.Stats.Failed), r.Stats.Failed)
	}
	return nil
}

func (r *Runner) runFeature(
	ctx context.Context,
	config harness.RunnerConfig,
	feature *harness.PreparedFeature,
) error {
	// Create runner
	runner, err := harness.NewRunner(config, feature)
	if err != nil {
		return fmt.Errorf("failed starting runner: %w", err)
	}
	defer runner.Close()

	// Run
	return runner.Run(ctx)
}

// LoadFromDir loads the .config.json from the directory if present and
// unmarshals into the config.
func (r *RunFeatureConfig) LoadFromDir(dir string) error {
	b, err := os.ReadFile(filepath.Join(dir, ".config.json"))
	if err != nil {
		// We're ok w/ it not existing
		if os.IsNotExist(err) {
			err = nil
		}
	} else {
		err = json.Unmarshal(b, r)
	}
	return err
}
