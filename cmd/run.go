package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/pmezard/go-difflib/difflib"
	"github.com/temporalio/features/harness/go/cmd"
	"github.com/temporalio/features/harness/go/harness"
	"github.com/temporalio/features/harness/go/history"
	"github.com/temporalio/features/sdkbuild"
	"github.com/urfave/cli/v2"
	nexuspb "go.temporal.io/api/nexus/v1"
	"go.temporal.io/api/operatorservice/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/log"
	"go.temporal.io/sdk/testsuite"
	"gopkg.in/yaml.v3"
)

// nexusFeatureDirPrefix marks features that require a per-test Nexus endpoint.
const nexusFeatureDirPrefix = "nexus/"

const (
	summaryListenAddr = "127.0.0.1:0"
	FeaturePassed     = "PASSED"
)

func runCmd() *cli.Command {
	var config RunConfig
	return &cli.Command{
		Name:  "run",
		Usage: "Run a test or set of tests",
		Flags: config.flags(),
		Action: func(ctx *cli.Context) error {
			return NewRunner(config).Run(ctx.Context, ctx.Args().Slice())
		},
	}
}

type SummaryEntry struct {
	Name    string `json:"name"`
	Outcome string `json:"outcome"`
	Message string `json:"message"`
}

type Summary []SummaryEntry

// RunConfig is configuration for NewRunner.
type RunConfig struct {
	PrepareConfig
	Server              string
	Namespace           string
	ClientCertPath      string
	ClientKeyPath       string
	CACertPath          string
	TLSServerName       string
	GenerateHistory     bool
	DisableHistoryCheck bool
	RetainTempDir       bool
	SummaryURI          string
	HTTPProxyURL        string
}

// dockerRunFlags are a subset of flags that apply when running in a docker container
func (r *RunConfig) dockerRunFlags() []cli.Flag {
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
			Name:        "ca-cert-path",
			Usage:       "Path of CA cert to use for server verification (optional)",
			Destination: &r.CACertPath,
		},
		&cli.StringFlag{
			Name:        "tls-server-name",
			Usage:       "TLS server name to use for verification and SNI override (optional)",
			Destination: &r.TLSServerName,
		},
	}
}

func (r *RunConfig) flags() []cli.Flag {
	return append([]cli.Flag{
		langFlag(&r.Lang),
		&cli.StringFlag{
			Name: "version",
			Usage: "SDK language version to run. Most languages support versions as paths. " +
				"Version cannot be present if prepared directory is.",
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
		&cli.BoolFlag{
			Name:        "retain-temp-dir",
			Usage:       "Do not delete the temp directory after the run",
			Destination: &r.RetainTempDir,
		},
		&cli.StringFlag{
			Name:        "prepared-dir",
			Usage:       "Relative directory already prepared. Cannot include version with this.",
			Destination: &r.DirName,
		},
	}, r.dockerRunFlags()...)
}

// Runner can run features.
type Runner struct {
	log    log.Logger
	config RunConfig
	// Root of the features repo
	rootDir    string
	createTime time.Time
	program    sdkbuild.Program
}

type runBatch struct {
	Run           *cmd.Run
	VariantName   string
	DynamicConfig map[string]any
	ExpectsProxy  bool
}

// NewRunner creates a new runner for the given config.
func NewRunner(config RunConfig) *Runner {
	return &Runner{
		// TODO(cretz): Configurable logger
		log:        harness.NewCLILogger(),
		config:     config,
		rootDir:    rootDir(),
		createTime: time.Now(),
	}
}

func (r *Runner) makeRunBatches(features []*RunFeature) ([]runBatch, error) {
	defaultBatch := runBatch{Run: &cmd.Run{}}
	var batches []runBatch
	for _, feature := range features {
		if len(feature.Config.RunVariants) == 0 {
			defaultBatch.Run.Features = append(defaultBatch.Run.Features, cmd.RunFeature{
				Dir:       feature.Dir,
				TaskQueue: r.taskQueueForFeature(feature.Dir, ""),
				Config:    feature.Config,
			})
			if feature.Config.ExpectUnauthedProxyCount > 0 || feature.Config.ExpectAuthedProxyCount > 0 {
				defaultBatch.ExpectsProxy = true
			}
			continue
		}
		for _, variant := range feature.Config.RunVariants {
			runFeature := cmd.RunFeature{
				Dir:         feature.Dir,
				TaskQueue:   r.taskQueueForFeature(feature.Dir, variant.Name),
				Config:      feature.Config,
				VariantName: variant.Name,
			}
			batches = append(batches, runBatch{
				Run: &cmd.Run{Features: []cmd.RunFeature{
					runFeature,
				}},
				VariantName:   variant.Name,
				DynamicConfig: variant.DynamicConfig,
				ExpectsProxy:  feature.Config.ExpectUnauthedProxyCount > 0 || feature.Config.ExpectAuthedProxyCount > 0,
			})
		}
	}
	if len(defaultBatch.Run.Features) > 0 {
		batches = append([]runBatch{defaultBatch}, batches...)
	}
	return batches, nil
}

func (r *Runner) taskQueueForFeature(dir string, variant string) string {
	if variant == "" {
		return fmt.Sprintf("features-%v-%v", dir, uuid.NewString())
	}
	return fmt.Sprintf("features-%v-%v-%v", dir, variant, uuid.NewString())
}

type dynamicConfigValue struct {
	Constraints map[string]any
	Value       any
}

func (r *Runner) dynamicConfigArgs(overrides map[string]any) ([]string, error) {
	cfgPath := filepath.Join(r.rootDir, "dockerfiles", "dynamicconfig", "docker.yaml")
	yamlBytes, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read dynamic config: %w", err)
	}
	var yamlValues map[string][]dynamicConfigValue
	if err = yaml.Unmarshal(yamlBytes, &yamlValues); err != nil {
		return nil, fmt.Errorf("unable to decode dynamic config: %w", err)
	}
	for key, value := range overrides {
		yamlValues[key] = []dynamicConfigValue{{Constraints: map[string]any{}, Value: value}}
	}

	keys := make([]string, 0, len(yamlValues))
	for key := range yamlValues {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	dynamicConfigArgs := make([]string, 0, len(yamlValues)*2)
	for _, key := range keys {
		for _, value := range yamlValues[key] {
			asJsonStr, err := json.Marshal(value.Value)
			if err != nil {
				return nil, fmt.Errorf("unable to marshal dynamic config value %s: %w", key, err)
			}
			dynamicConfigArgs = append(dynamicConfigArgs, "--dynamic-config-value", fmt.Sprintf("%s=%s", key, asJsonStr))
		}
	}
	return dynamicConfigArgs, nil
}

// Run runs all matching features for the given patterns (or all if no patterns
// given).
func (r *Runner) Run(ctx context.Context, patterns []string) error {
	var err error
	if r.config.Lang, err = normalizeLangName(r.config.Lang); err != nil {
		return err
	}

	// Cannot generate history if a version isn't provided explicitly
	if r.config.GenerateHistory && r.config.Version == "" {
		return fmt.Errorf("must have explicit version to generate history")
	}

	// If prepared dir given, validate
	if r.config.DirName != "" {
		if strings.ContainsAny(r.config.DirName, `\/`) {
			return fmt.Errorf("prepared directory must not have path separators, it is always relative to the SDK features root")
		} else if r.config.Version != "" {
			return fmt.Errorf("cannot provide version with prepared directory")
		}
		if _, err := os.Stat(filepath.Join(r.rootDir, r.config.DirName)); err != nil {
			return fmt.Errorf("failed checking prepared directory: %w", err)
		}
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
	if r.config.GenerateHistory {
		for _, feature := range features {
			if len(feature.Config.RunVariants) > 0 {
				return fmt.Errorf("cannot generate history for feature %q because it defines runVariants", feature.Dir)
			}
		}
	}

	// Create a Nexus endpoint per feature under features/nexus/ targeting that feature's task
	// queue. Endpoint names are passed to the lang harness through RunFeature.NexusEndpoint and
	// the endpoints are deleted once the lang harness completes.
	deleteEndpoints, err := r.createNexusEndpoints(ctx, run)
	if err != nil {
		return err
	}
	defer deleteEndpoints()
	if len(run.Features) == 0 {
		r.log.Info("No features left to run after Nexus skip; treating run as successful")
		return nil
	}

	// Ensure any created temp dir is cleaned on ctrl-c or normal exit
	if r.config.DirName == "" && !r.config.RetainTempDir {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-c
			r.destroyTempDir()
			os.Exit(1)
		}()
		defer r.destroyTempDir()
	}

	batches, err := r.makeRunBatches(features)
	if err != nil {
		return err
	}
	for _, batch := range batches {
		if err := r.runBatch(ctx, batch); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runner) runBatch(ctx context.Context, batch runBatch) error {
	config := r.config
	if config.Namespace == "" {
		config.Namespace = "features-ns-" + uuid.NewString()
	}
	label := "default"
	if batch.VariantName != "" {
		label = batch.VariantName
	}

	if config.Server == "" {
		dynamicConfigArgs, err := r.dynamicConfigArgs(batch.DynamicConfig)
		if err != nil {
			return err
		}
		server, err := testsuite.StartDevServer(ctx, testsuite.DevServerOptions{
			LogLevel:      "error",
			ClientOptions: &client.Options{Namespace: config.Namespace},
			ExtraArgs:     dynamicConfigArgs,
		})
		if err != nil {
			return fmt.Errorf("failed starting devserver: %w", err)
		}
		defer server.Stop()
		config.Server = server.FrontendHostPort()
		r.log.Info("Started server", "HostPort", config.Server, "Variant", label, "DynamicConfigOverrides", batch.DynamicConfig)
	} else {
		if batch.VariantName != "" {
			return fmt.Errorf("feature run variant %q requires the embedded dev server, but --server was provided", label)
		}
		err := harness.WaitNamespaceAvailable(ctx, r.log,
			config.Server, config.Namespace, config.ClientCertPath, config.ClientKeyPath, config.CACertPath, config.TLSServerName)
		if err != nil {
			return err
		}
	}

	var proxyServer *harness.HTTPConnectProxyServer
	if batch.ExpectsProxy {
		var err error
		proxyServer, err = harness.StartHTTPConnectProxyServer(harness.HTTPConnectProxyServerOptions{Log: r.log})
		if err != nil {
			return fmt.Errorf("could not start proxy server: %w", err)
		}
		config.HTTPProxyURL = "http://" + proxyServer.Address
		r.log.Info("Started HTTP CONNECT proxy server", "address", proxyServer.Address)
		defer proxyServer.Close()
	}

	l, err := net.Listen("tcp", summaryListenAddr)
	if err != nil {
		return err
	}
	defer l.Close()
	summaryChan := make(chan Summary)
	go r.summaryServer(l, summaryChan)
	config.SummaryURI = "tcp://" + l.Addr().String()

	r.log.Info("Running feature batch", "Variant", label, "Features", len(batch.Run.Features))

	origConfig := r.config
	r.config = config
	defer func() {
		r.config = origConfig
	}()

	err = nil
	switch config.Lang {
	case "go":
		// If there's a version or prepared dir we run external, otherwise we run local
		if config.Version != "" || config.DirName != "" {
			if config.DirName != "" {
				r.program, err = sdkbuild.GoProgramFromDir(filepath.Join(r.rootDir, config.DirName))
			}
			if err == nil {
				err = r.RunGoExternal(ctx, batch.Run)
			}
		} else {
			err = cmd.NewRunner(cmd.RunConfig{
				Server:         config.Server,
				Namespace:      config.Namespace,
				ClientCertPath: config.ClientCertPath,
				ClientKeyPath:  config.ClientKeyPath,
				CACertPath:     config.CACertPath,
				TLSServerName:  config.TLSServerName,
				SummaryURI:     config.SummaryURI,
				HTTPProxyURL:   config.HTTPProxyURL,
			}).Run(ctx, batch.Run)
		}
	case "java":
		if config.DirName != "" {
			r.program, err = sdkbuild.JavaProgramFromDir(filepath.Join(r.rootDir, config.DirName))
		}
		if err == nil {
			err = r.RunJavaExternal(ctx, batch.Run)
		}
	case "ts":
		if config.DirName != "" {
			r.program, err = sdkbuild.TypeScriptProgramFromDir(filepath.Join(r.rootDir, config.DirName))
		}
		if err == nil {
			err = r.RunTypeScriptExternal(ctx, batch.Run)
		}
	case "php":
		if config.DirName != "" {
			r.program, err = sdkbuild.PhpProgramFromDir(
				filepath.Join(r.rootDir, config.DirName),
				r.rootDir,
			)
		}
		if err == nil {
			err = r.RunPhpExternal(ctx, batch.Run)
		}
	case "py":
		if config.DirName != "" {
			r.program, err = sdkbuild.PythonProgramFromDir(filepath.Join(r.rootDir, config.DirName))
		}
		if err == nil {
			err = r.RunPythonExternal(ctx, batch.Run)
		}
	case "cs":
		if config.DirName != "" {
			r.program, err = sdkbuild.DotNetProgramFromDir(filepath.Join(r.rootDir, config.DirName))
		}
		if err == nil {
			err = r.RunDotNetExternal(ctx, batch.Run)
		}
	case "rb":
		if config.DirName != "" {
			r.program, err = sdkbuild.RubyProgramFromDir(
				filepath.Join(r.rootDir, config.DirName),
				filepath.Join(r.rootDir, "harness", "ruby"),
			)
		}
		if err == nil {
			err = r.RunRubyExternal(ctx, batch.Run)
		}
	default:
		err = fmt.Errorf("unrecognized language")
	}
	if err != nil {
		return err
	}
	l.Close()
	summary, ok := <-summaryChan
	if !ok {
		r.log.Debug("did not receive a test run summary - adopting legacy behavior of assuming no tests were skipped")
		for _, feature := range batch.Run.Features {
			summary = append(summary, SummaryEntry{Name: feature.SummaryName(), Outcome: FeaturePassed})
		}
	} else if batch.VariantName != "" {
		summary = rewriteVariantSummary(summary, batch.Run.Features)
	}

	// For features that expected proxy connections, count how many expected
	// ignoring skips and compare count with actual. If any failed we don't need
	// even do the comparison.
	if proxyServer != nil {
		var anyFailed bool
		var expectUnauthedProxyCount, expectAuthedProxyCount int
		for _, summ := range summary {
			if summ.Outcome == "FAILED" {
				anyFailed = true
				break
			} else if summ.Outcome == "PASSED" {
				for _, feature := range batch.Run.Features {
					if feature.SummaryName() == summ.Name {
						expectUnauthedProxyCount += feature.Config.ExpectUnauthedProxyCount
						expectAuthedProxyCount += feature.Config.ExpectAuthedProxyCount
						break
					}
				}
			}
		}
		if !anyFailed {
			if proxyServer.UnauthedConnectionsTunneled.Load() != uint32(expectUnauthedProxyCount) {
				return fmt.Errorf("expected %v unauthed HTTP proxy connections, got %v",
					expectUnauthedProxyCount, proxyServer.UnauthedConnectionsTunneled.Load())
			} else if proxyServer.AuthedConnectionsTunneled.Load() != uint32(expectAuthedProxyCount) {
				return fmt.Errorf("expected %v authed HTTP proxy connections, got %v",
					expectAuthedProxyCount, proxyServer.AuthedConnectionsTunneled.Load())
			} else {
				r.log.Debug("Matched expected HTTP proxy connections",
					"expectUnauthed", expectUnauthedProxyCount, "actualUnauthed", proxyServer.UnauthedConnectionsTunneled.Load(),
					"expectAuthed", expectAuthedProxyCount, "actualAuthed", proxyServer.AuthedConnectionsTunneled.Load())
			}
		}
	}

	return r.handleHistory(ctx, batch.Run, summary)
}

func rewriteVariantSummary(summary Summary, features []cmd.RunFeature) Summary {
	for i, entry := range summary {
		for _, feature := range features {
			if entry.Name == feature.Dir {
				summary[i].Name = feature.SummaryName()
				break
			}
		}
	}
	return summary
}

func (r *Runner) handleHistory(ctx context.Context, run *cmd.Run, summary Summary) error {
	// Handle each
	var cl client.Client
	var failureCount int
	for _, feature := range run.Features {
		// We ignore history if there are no workflows
		if feature.Config.NoWorkflow {
			continue
		}
		entry, ok := summary.Find(feature.SummaryName())
		if !ok {
			r.log.Info("skipping history check because feature not listed in execution summary", "feature", feature.SummaryName())
			continue
		}
		if entry.Outcome == "SKIPPED" {
			r.log.Info("skipping history check because feature was skipped", "feature", feature.Dir, "reason", entry.Message)
			continue
		}

		// Dial client if not already done
		if cl == nil {
			opts := client.Options{
				HostPort:  r.config.Server,
				Namespace: r.config.Namespace,
				Logger:    r.log,
			}
			tlsCfg, err := harness.LoadTLSConfig(
				r.config.ClientCertPath,
				r.config.ClientKeyPath,
				r.config.CACertPath,
				r.config.TLSServerName,
			)
			if err != nil {
				return fmt.Errorf("failed to load TLS config: %w", err)
			}
			opts.ConnectionOptions.TLS = tlsCfg
			if cl, err = client.Dial(opts); err != nil {
				return fmt.Errorf("failed creating client: %w", err)
			}
			defer cl.Close()
		}

		// Check history
		if err := r.handleSingleHistory(ctx, cl, feature); err != nil {
			failureCount++
			r.log.Error("Feature history handling failed", "Feature", feature.Dir, "error", err)
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
	storage := history.Storage{Dir: filepath.Join(r.rootDir, "features", feature.Dir, "history"), Lang: r.config.Lang}
	// Load all histories from storage to validate against
	existingSet, err := storage.Load()
	if err != nil {
		return err
	}
	if !r.config.GenerateHistory && (r.config.DisableHistoryCheck || len(existingSet.ByVersion) == 0) {
		r.log.Info("Skipping history check since nothing to check against and not generating",
			"Feature", feature.Dir)
		return nil
	}
	currHist, err := fetcher.Fetch(ctx)
	if err != nil {
		return fmt.Errorf("failed getting history: %w", err)
	}

	// Do a check against all scrubbed existing histories to ensure nothing
	// changed
	if !r.config.DisableHistoryCheck {

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

// summaryServer uses the supplied listener to handle a single incoming
// connection that sends JSONL data describing the execution status of feature
// tests as determined by a lower level test execution harness. JSONL data items
// are expected to have the following fields
//   - Name (string) the name of the test
//   - Outcome (string) one of PASSED|FAILED|SKIPPED
//   - Message (string) a free text field
func (r *Runner) summaryServer(l net.Listener, out chan<- Summary) {
	conn, err := l.Accept()
	if err != nil {
		// Accept returns an error if the listener is closed
		close(out)
		return
	}
	summary := Summary{}
	rdr := bufio.NewReaderSize(conn, 4096)
	for {
		line, err := rdr.ReadBytes('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			r.log.Error("error reading from summary socket", "error", err.Error())
			close(out)
			return
		}
		var entry SummaryEntry
		err = json.Unmarshal(line, &entry)
		if err != nil {
			r.log.Error("error unmarshalling summary entry", "error", err.Error(), "line", string(line))
			continue
		}
		summary = append(summary, entry)
	}
	out <- summary
}

func rootDir() string {
	_, currFile, _, _ := runtime.Caller(0)
	return filepath.Dir(filepath.Dir(currFile))
}

// createNexusEndpoints creates a Nexus endpoint per RunFeature whose Dir is under
// features/nexus, populating RunFeature.NexusEndpoint. It returns a cleanup function that
// deletes the created endpoints. The cleanup function is always safe to call.
func (r *Runner) createNexusEndpoints(ctx context.Context, run *cmd.Run) (func(), error) {
	noop := func() {}
	var nexusFeatures []*cmd.RunFeature
	for i := range run.Features {
		if strings.HasPrefix(run.Features[i].Dir, nexusFeatureDirPrefix) {
			nexusFeatures = append(nexusFeatures, &run.Features[i])
		}
	}
	if len(nexusFeatures) == 0 {
		return noop, nil
	}

	opts := client.Options{HostPort: r.config.Server, Namespace: r.config.Namespace, Logger: r.log}
	tlsCfg, err := harness.LoadTLSConfig(r.config.ClientCertPath, r.config.ClientKeyPath, r.config.CACertPath, r.config.TLSServerName)
	if err != nil {
		return noop, fmt.Errorf("failed to load TLS config: %w", err)
	}
	opts.ConnectionOptions.TLS = tlsCfg
	cl, err := client.Dial(opts)
	if err != nil {
		return noop, fmt.Errorf("failed creating client for nexus endpoint setup: %w", err)
	}

	type createdEndpoint struct {
		ID      string
		Version int64
		Name    string
	}
	var created []createdEndpoint
	cleanup := func() {
		for _, ep := range created {
			delCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			_, err := cl.OperatorService().DeleteNexusEndpoint(delCtx, &operatorservice.DeleteNexusEndpointRequest{
				Id:      ep.ID,
				Version: ep.Version,
			})
			cancel()
			if err != nil {
				r.log.Warn("Failed deleting nexus endpoint", "Endpoint", ep.Name, "Error", err)
			}
		}
		cl.Close()
	}

	// Nexus endpoint names have stricter, DNS-style validation than task queues, so / and _
	// in the feature dir must be normalized to -.
	sanitize := strings.NewReplacer("/", "-", "_", "-")
	for _, feature := range nexusFeatures {
		name := "features-nexus-" + sanitize.Replace(feature.Dir) + "-" + uuid.NewString()
		createCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		res, err := cl.OperatorService().CreateNexusEndpoint(createCtx, &operatorservice.CreateNexusEndpointRequest{
			Spec: &nexuspb.EndpointSpec{
				Name: name,
				Target: &nexuspb.EndpointTarget{
					Variant: &nexuspb.EndpointTarget_Worker_{
						Worker: &nexuspb.EndpointTarget_Worker{
							Namespace: r.config.Namespace,
							TaskQueue: feature.TaskQueue,
						},
					},
				},
			},
		})
		cancel()
		if err != nil {
			// Only skip when the server signals that Nexus endpoint management is unavailable.
			var permDenied *serviceerror.PermissionDenied
			if !errors.As(err, &permDenied) {
				cleanup()
				return noop, fmt.Errorf("failed creating nexus endpoint for %v: %w", feature.Dir, err)
			}
			r.log.Warn("Skipping Nexus features: server does not support Nexus endpoint creation",
				"Feature", feature.Dir, "Error", err)
			cleanup()
			kept := run.Features[:0]
			for _, f := range run.Features {
				if !strings.HasPrefix(f.Dir, nexusFeatureDirPrefix) {
					kept = append(kept, f)
				}
			}
			run.Features = kept
			return noop, nil
		}
		feature.NexusEndpoint = name
		created = append(created, createdEndpoint{ID: res.Endpoint.Id, Version: res.Endpoint.Version, Name: name})
	}
	return cleanup, nil
}

func (r *Runner) destroyTempDir() {
	if r.program != nil {
		_ = os.RemoveAll(r.program.Dir())
	}
}

func normalizeLangName(lang string) (string, error) {
	// Normalize to file extension
	switch lang {
	case "go", "java", "ts", "php", "py", "cs", "rb":
	case "typescript":
		lang = "ts"
	case "python":
		lang = "py"
	case "dotnet", "csharp":
		lang = "cs"
	case "ruby":
		lang = "rb"
	default:
		return "", fmt.Errorf("invalid language %q, must be one of: go or java or ts or py or cs or rb", lang)
	}
	return lang, nil
}

func expandLangName(lang string) (string, error) {
	// Expand to lang name
	switch lang {
	case "go", "java", "typescript", "php", "python", "ruby":
	case "ts":
		lang = "typescript"
	case "py":
		lang = "python"
	case "cs":
		lang = "dotnet"
	case "rb":
		lang = "ruby"
	default:
		return "", fmt.Errorf("invalid language %q, must be one of: go or java or ts or py or cs or rb", lang)
	}
	return lang, nil
}

func langFlag(destination *string) *cli.StringFlag {
	return &cli.StringFlag{
		Name:        "lang",
		Usage:       "SDK language to run ('go' or 'java' or 'ts' or 'py' or 'cs' or 'rb')",
		Required:    true,
		Destination: destination,
	}
}

func (s Summary) Find(featureName string) (*SummaryEntry, bool) {
	for _, entry := range s {
		if entry.Name == featureName {
			return &entry, true
		}
	}
	return nil, false
}
