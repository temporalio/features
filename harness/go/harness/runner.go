package harness

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"sync"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk-features/harness/go/history"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/log"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/worker"
	"golang.org/x/mod/semver"
)

// Runner represents a runner that can run a feature.
type Runner struct {
	RunnerConfig
	Client     client.Client
	Worker     worker.Worker
	Feature    *PreparedFeature
	CreateTime time.Time

	Assert        *assert.Assertions
	LastAssertErr error
	Require       *require.Assertions
}

// RunnerConfig is configuration for NewRunner.
type RunnerConfig struct {
	ServerHostPort string
	Namespace      string
	TaskQueue      string
	Log            log.Logger
}

// NewRunner creates a new runner for the given config and feature.
func NewRunner(config RunnerConfig, feature *PreparedFeature) (*Runner, error) {
	if config.ServerHostPort == "" {
		config.ServerHostPort = client.DefaultHostPort
	}
	if config.Namespace == "" {
		config.Namespace = client.DefaultNamespace
	}
	if config.Log == nil {
		config.Log = DefaultLogger
	}
	r := &Runner{RunnerConfig: config, Feature: feature}
	r.Assert = assert.New(assertTestingFunc(func(format string, args ...interface{}) {
		r.LastAssertErr = fmt.Errorf(format, args...)
	}))
	r.Require = require.New(&requireTestingPanic{})

	// Close on failure
	success := false
	defer func() {
		if !success {
			r.Close()
		}
	}()

	// Create client
	r.Feature.ClientOptions.HostPort = r.ServerHostPort
	r.Feature.ClientOptions.Namespace = r.Namespace
	if r.Feature.ClientOptions.Logger == nil {
		r.Feature.ClientOptions.Logger = r.Log
	}

	var err error
	if r.Client, err = client.NewClient(r.Feature.ClientOptions); err != nil {
		return nil, fmt.Errorf("failed creating client: %w", err)
	}

	// Create worker
	r.CreateTime = time.Now()
	r.Feature.WorkerOptions.WorkflowPanicPolicy = worker.FailWorkflow
	r.Worker = worker.New(r.Client, config.TaskQueue, r.Feature.WorkerOptions)

	// Register the workflows and activities
	for _, workflow := range r.Feature.Workflows {
		r.Worker.RegisterWorkflow(workflow)
	}
	for _, activity := range r.Feature.Activities {
		r.Worker.RegisterActivity(activity)
	}

	// Start the worker
	if err := r.Worker.Start(); err != nil {
		return nil, fmt.Errorf("failed starting worker: %w", err)
	}

	success = true
	return r, nil
}

// Run executes a single feature.
func (r *Runner) Run(ctx context.Context) error {
	// Do normal run
	r.Log.Debug("Executing feature", "Feature", r.Feature.Dir)
	var run client.WorkflowRun
	var err error
	if r.Feature.Execute != nil {
		run, err = r.Feature.Execute(ctx, r)
	} else {
		run, err = r.ExecuteDefault(ctx)
	}
	// Bail if there is an error or no run
	if run == nil || err != nil {
		return err
	}

	// Result check
	r.Log.Debug("Checking feature", "Feature", r.Feature.Dir)
	if r.Feature.CheckResult != nil {
		err = r.Feature.CheckResult(ctx, r, run)
	} else {
		err = r.CheckResultDefault(ctx, run)
	}
	if err != nil {
		return err
	}

	// History check
	r.Log.Debug("Checking history", "Feature", r.Feature.Dir)
	if r.Feature.CheckHistory != nil {
		err = r.Feature.CheckHistory(ctx, r, run)
	} else {
		err = r.CheckHistoryDefault(ctx, run)
	}
	return err
}

// ExecuteDefault is the default execution that just runs the first workflow and
// assumes it takes no parameters.
func (r *Runner) ExecuteDefault(ctx context.Context) (client.WorkflowRun, error) {
	return r.Client.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{TaskQueue: r.TaskQueue}, r.Feature.Workflows[0])
}

// CheckResultDefault performs the default result checks which just waits on
// completion and checks against feature expectations.
func (r *Runner) CheckResultDefault(ctx context.Context, run client.WorkflowRun) error {
	// If there's an expectation of result, build pointer to hold it
	var actualPtr interface{}
	if r.Feature.ExpectRunResult != nil {
		actualPtr = reflect.New(reflect.TypeOf(r.Feature.ExpectRunResult)).Interface()
	}

	// Wait for completion
	err := run.Get(ctx, actualPtr)

	// If an error is expected, check it
	if r.Feature.ExpectActivityError != "" {
		var actErr *temporal.ActivityError
		if !errors.As(err, &actErr) {
			return fmt.Errorf("expected activity error, got: %w", err)
		} else if !r.Assert.EqualError(actErr.Unwrap(), r.Feature.ExpectActivityError) {
			return fmt.Errorf("activity error string mismatch, error: %w", err)
		}
	}

	// If result is expected, check it
	if actualPtr != nil {
		err = r.CheckAssertion(r.Assert.Equal(r.Feature.ExpectRunResult, reflect.ValueOf(actualPtr).Elem().Interface()))
		if err != nil {
			return err
		}
	}

	return nil
}

// CheckHistoryDefault is the default history checker which fetches the history
// and replays it to confirm it succeeds. It also replays all other histories
// for versions <= the current SDK version.
func (r *Runner) CheckHistoryDefault(ctx context.Context, _ client.WorkflowRun) error {
	// First check our own history
	r.Log.Debug("Checking current execution replay", "Feature", r.Feature.Dir)
	fetcher := &history.Fetcher{
		Client:         r.Client,
		Namespace:      r.Namespace,
		TaskQueue:      r.TaskQueue,
		FeatureStarted: r.CreateTime,
	}
	histories, err := fetcher.Fetch(ctx)
	if err != nil {
		return fmt.Errorf("failed fetching histories: %w", err)
	}
	if err := r.ReplayHistories(ctx, histories); err != nil {
		return fmt.Errorf("failed replaying current execution: %w", err)
	}

	// Now load up all other histories
	storage := &history.Storage{Dir: filepath.Join(r.Feature.AbsDir, "history"), Lang: "go"}
	set, err := storage.Load()
	if err != nil {
		return fmt.Errorf("failed loading histories: %w", err)
	}

	// Go over each history, and every one that's on or before this version should
	// replay successfully
	for version, histories := range set.ByVersion {
		// Don't include newer histories
		if semver.Compare(version, SDKVersion) > 0 {
			r.Log.Debug("Skipping history because it's for later version", "Feature", r.Feature.Dir, "Version", version)
			continue
		}

		r.Log.Debug("Checking previous history replay", "Feature", r.Feature.Dir, "Version", version)
		if err := r.ReplayHistories(ctx, histories); err != nil {
			return fmt.Errorf("failed replaying history version %v: %w", version, err)
		}
	}
	return nil
}

// ReplayHistories replays the given histories checking for errors.
func (r *Runner) ReplayHistories(ctx context.Context, histories history.Histories) error {
	// Create replayer with all the workflow funcs
	replayer := worker.NewWorkflowReplayer()
	for _, workflow := range r.Feature.Workflows {
		replayer.RegisterWorkflow(workflow)
	}
	// Replay each
	for _, history := range histories {
		if err := replayer.ReplayWorkflowHistory(nil, history); err != nil {
			return err
		}
	}
	return nil
}

// QueryUntilEventually runs the given query every so often until the value
// matches the expected value.
func (r *Runner) QueryUntilEventually(
	ctx context.Context,
	run client.WorkflowRun,
	query string,
	expected interface{},
	interval time.Duration,
	timeout time.Duration,
) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	timeoutCh := time.After(timeout)
	var lastErr error
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeoutCh:
			return fmt.Errorf("timeout waiting for query %v to get proper value, last error: %w", query, lastErr)
		case <-ticker.C:
			val, err := r.Client.QueryWorkflow(ctx, run.GetID(), run.GetRunID(), query)
			// We allow a "query failed" if the query is not registered yet
			var queryFailed *serviceerror.QueryFailed
			if errors.As(err, &queryFailed) {
				continue
			} else if err != nil {
				return fmt.Errorf("failed querying %v: %w", query, err)
			}
			// Convert to actual
			actualPtr := reflect.New(reflect.TypeOf(expected)).Interface()
			if err := val.Get(actualPtr); err != nil {
				return fmt.Errorf("failed converting result of query %v: %w", query, err)
			}
			actual := reflect.ValueOf(actualPtr).Elem().Interface()
			if lastErr = r.CheckAssertion(r.Assert.Equal(expected, actual)); lastErr == nil {
				return nil
			}
		}
	}
}

// Close closes this runner.
func (r *Runner) Close() {
	if r.Worker == nil {
		r.Worker.Stop()
		r.Worker = nil
	}
	if r.Client == nil {
		r.Client.Close()
		r.Client = nil
	}
}

// CheckAssertion can be used with a result to Runner.Assert calls to return the
// last error if false.
func (r *Runner) CheckAssertion(result bool) error {
	if !result {
		return r.LastAssertErr
	}
	return nil
}

type assertTestingFunc func(format string, args ...interface{})

func (a assertTestingFunc) Errorf(format string, args ...interface{}) { a(format, args...) }

type requireTestingPanic struct {
	lastErr     error
	lastErrLock sync.RWMutex
}

func (r *requireTestingPanic) Errorf(format string, args ...interface{}) {
	r.lastErrLock.Lock()
	defer r.lastErrLock.Unlock()
	r.lastErr = fmt.Errorf(format, args...)
}

func (r *requireTestingPanic) FailNow() {
	r.lastErrLock.RLock()
	defer r.lastErrLock.RUnlock()
	if r.lastErr != nil {
		panic(r.lastErr)
	}
}
