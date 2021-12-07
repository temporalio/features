package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/urfave/cli/v2"
	"go.temporal.io/sdk-features/harness/go/harness"
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
			return NewRunner(config).Run(ctx.Context, &run)
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
}

// RunConfig is configuration for NewRunner.
type RunConfig struct {
	Server    string
	Namespace string
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
	}
}

// Runner is a runner that can run Go features.
type Runner struct {
	log    log.Logger
	config RunConfig
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
	var failureCount int
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
			return fmt.Errorf("feature %v not found", runFeature.Dir)
		} else if feature.SkipReason != "" {
			r.log.Warn("Skipping feature", "Feature", feature.Dir, "Reason", feature.SkipReason)
			continue
		}

		runnerConfig := harness.RunnerConfig{
			ServerHostPort: r.config.Server,
			Namespace:      r.config.Namespace,
			TaskQueue:      runFeature.TaskQueue,
			Log:            r.log,
		}
		if err := r.runFeature(ctx, runnerConfig, feature); err != nil {
			failureCount++
			r.log.Error("Feature failed", "Feature", feature.Dir, "error", err)
		}
	}
	if failureCount > 0 {
		return fmt.Errorf("%v failure(s) reported", failureCount)
	}
	r.log.Info("All features passed")
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
