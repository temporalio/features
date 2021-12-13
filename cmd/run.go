package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"runtime"
	"time"

	"github.com/DataDog/temporalite"
	"github.com/google/uuid"
	"github.com/pmezard/go-difflib/difflib"
	"github.com/urfave/cli/v2"
	"go.temporal.io/sdk-features/harness/go/cmd"
	"go.temporal.io/sdk-features/harness/go/history"
	"go.temporal.io/sdk/client"
	"go.temporal.io/server/common/log"
	"go.temporal.io/server/common/log/tag"
)

func runCmd() *cli.Command {
	var config RunConfig
	return &cli.Command{
		Name:  "run",
		Usage: "run a test or set of tests",
		Flags: config.flags(),
		Action: func(ctx *cli.Context) error {
			return NewRunner(config).Run(ctx.Context, ctx.Args().Slice())
		},
	}
}

// RunConfig is configuration for NewRunner.
type RunConfig struct {
	Lang                string
	Version             string
	GenerateHistory     bool
	DisableHistoryCheck bool
	// If not set, defaults to creating one
	Server string
	// Defaults to unique value
	Namespace     string
	RetainTempDir bool
}

func (r *RunConfig) flags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:        "lang",
			Usage:       "SDK language to run ('go' or 'java' or 'ts')",
			Required:    true,
			Destination: &r.Lang,
		},
		&cli.StringFlag{
			Name:        "version",
			Usage:       "SDK language version to run",
			Destination: &r.Version,
		},
		&cli.BoolFlag{
			Name:        "generate-history",
			Usage:       "Generate the history of the features that are run (overwrites any existing history)",
			Destination: &r.GenerateHistory,
		},
		&cli.BoolFlag{
			Name:        "no-history-check",
			Usage:       "Do not verify history matches",
			Destination: &r.DisableHistoryCheck,
		},
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
		&cli.BoolFlag{
			Name:        "retain-temp-dir",
			Usage:       "Do not delete the temp directory after the run",
			Destination: &r.RetainTempDir,
		},
	}
}

// Runner can run features.
type Runner struct {
	log    log.Logger
	config RunConfig
	// Root of the sdk-features repo
	rootDir    string
	createTime time.Time
}

// NewRunner creates a new runner for the given config.
func NewRunner(config RunConfig) *Runner {
	return &Runner{
		// TODO(cretz): Configurable logger
		log:        log.NewCLILogger(),
		config:     config,
		rootDir:    rootDir(),
		createTime: time.Now(),
	}
}

// Run runs all matching features for the given patterns (or all if no patterns
// given).
func (r *Runner) Run(ctx context.Context, patterns []string) error {
	if r.config.Lang != "go" && r.config.Lang != "java" && r.config.Lang != "ts" {
		return fmt.Errorf("invalid language %q, must be one of: go or java or ts", r.config.Lang)
	}

	// Cannot generate history if a version isn't provided explicitly
	if r.config.GenerateHistory && r.config.Version == "" {
		return fmt.Errorf("must have explicit version to generate history")
	}

	// If the namespace is not set, set it ourselves
	if r.config.Namespace == "" {
		r.config.Namespace = "sdk-features-ns-" + uuid.NewString()
	}

	// Collect features to run
	features, err := r.GlobFeatures(patterns)
	if err != nil {
		return err
	} else if len(features) == 0 {
		return fmt.Errorf("no features matched")
	} else if len(features) > 1 && r.config.GenerateHistory {
		return fmt.Errorf("can only specify a single feature when generating history")
	}
	// Aa task queue to every feature
	run := &cmd.Run{Features: make([]cmd.RunFeature, len(features))}
	for i, feature := range features {
		run.Features[i].Dir = feature.Dir
		run.Features[i].TaskQueue = fmt.Sprintf("sdk-features-%v-%v", feature.Dir, uuid.NewString())
	}

	// If the server is not set, start it ourselves
	if r.config.Server == "" {
		server, err := temporalite.NewServer(
			// TODO(cretz): Allow multiple?
			temporalite.WithNamespaces(r.config.Namespace),
			temporalite.WithPersistenceDisabled(),
			temporalite.WithDynamicPorts(),
			// TODO(cretz): Server is noisy, allow separate config
			// temporalite.WithLogger(r.log),
			temporalite.WithLogger(log.NewNoopLogger()),
		)
		if err != nil {
			return fmt.Errorf("failed creating server: %w", err)
		}
		if err := server.Start(); err != nil {
			return fmt.Errorf("failed starting server: %w", err)
		}
		defer server.Stop()
		// Set server host:port
		r.config.Server = server.FrontendHostPort()
		r.log.Info("Started server", tag.NewStringTag("HostPort", r.config.Server))
	}

	err = nil
	switch r.config.Lang {
	case "go":
		// If there's a version we run external, otherwise we run local
		if r.config.Version != "" {
			err = r.RunGoExternal(ctx, run)
		} else {
			err = cmd.NewRunner(cmd.RunConfig{Server: r.config.Server, Namespace: r.config.Namespace}).Run(ctx, run)
		}
	case "java":
		err = r.RunJavaExternal(ctx, run)
	default:
		err = fmt.Errorf("unrecognized language")
	}
	if err != nil {
		return err
	}

	// Now that we have completed successfully, check or collect history
	return r.handleHistory(ctx, run)
}

func (r *Runner) handleHistory(ctx context.Context, run *cmd.Run) error {
	cl, err := client.NewClient(client.Options{
		HostPort:  r.config.Server,
		Namespace: r.config.Namespace,
		Logger:    log.NewSdkLogger(r.log),
	})
	if err != nil {
		return fmt.Errorf("failed creating client: %w", err)
	}
	defer cl.Close()

	// Handle each
	var failureCount int
	for _, feature := range run.Features {
		if err := r.handleSingleHistory(ctx, cl, feature); err != nil {
			failureCount++
			r.log.Error("Feature history handling failed", tag.NewStringTag("Feature", feature.Dir), tag.NewErrorTag(err))
		}
	}
	if failureCount > 0 {
		return fmt.Errorf("%v failure(s) reported", failureCount)
	}
	return nil
}

func (r *Runner) handleSingleHistory(ctx context.Context, client client.Client, feature cmd.RunFeature) error {
	// Obtain current history from the server even no history checking/generating
	fetcher := history.Fetcher{
		Client:         client,
		Namespace:      r.config.Namespace,
		TaskQueue:      feature.TaskQueue,
		FeatureStarted: r.createTime,
	}
	currHist, err := fetcher.Fetch(ctx)
	if err != nil {
		return fmt.Errorf("failed getting history: %w", err)
	}
	storage := history.Storage{Dir: filepath.Join(r.rootDir, "features", feature.Dir, "history"), Lang: r.config.Lang}

	// Do a check against all scrubbed existing histories to ensure nothing
	// changed
	if !r.config.DisableHistoryCheck {
		// Load all histories from storage to validate against
		existingSet, err := storage.Load()
		if err != nil {
			return err
		}

		// Check that all versions of history match the current one when scrubbed
		// TODO(cretz): Allow some versions to ignore histories from other versions
		currHistScrubbed := currHist.Clone()
		currHistScrubbed.ScrubRunSpecificFields()
		for version, existingHist := range existingSet.ByVersion {
			// Scrub, then check equality
			existingHist.ScrubRunSpecificFields()
			if !currHistScrubbed.Equals(existingHist) {
				// Convert both to JSON because it shows a better diff
				actualJSON, err := json.MarshalIndent(currHistScrubbed, "", "  ")
				if err != nil {
					return err
				}
				expectedJSON, err := json.MarshalIndent(existingHist, "", "  ")
				if err != nil {
					return err
				}
				// Technically, in Go, the version may be empty
				currVersion := r.config.Version
				if currVersion == "" {
					currVersion = "<current>"
				}
				// Use the same diff lib testify assertion uses
				diff, _ := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
					A:        difflib.SplitLines(string(expectedJSON)),
					B:        difflib.SplitLines(string(actualJSON)),
					FromFile: feature.Dir + "/history/history." + r.config.Lang + "." + version + ".json",
					FromDate: "",
					ToFile:   feature.Dir + "/history/history." + r.config.Lang + "." + currVersion + ".json",
					ToDate:   "",
					Context:  10,
				})
				// We are going to just dump this to log since it has a multiline output
				// that Zap is not cool with in a tag
				// TODO(cretz): Make equality output more configurable?
				r.log.Error("History check failed, diff:\n" + diff)
				return fmt.Errorf("on feature %v, history with current version %v didn't match version %v",
					feature.Dir, currVersion, version)
			}
		}
	}

	// Store history, overwriting if necessary
	if r.config.GenerateHistory {
		err = storage.Store(&history.StoredSet{ByVersion: map[string]history.Histories{r.config.Version: currHist}})
		if err != nil {
			return fmt.Errorf("failed storing history for %v: %w", feature.Dir, err)
		}
	}
	return nil
}

func rootDir() string {
	_, currFile, _, _ := runtime.Caller(0)
	return filepath.Dir(filepath.Dir(currFile))
}
