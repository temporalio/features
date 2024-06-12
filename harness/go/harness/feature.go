package harness

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"

	"github.com/urfave/cli/v2"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

// SDKVersion is the Go SDK version with the "v" prefix.
const SDKVersion = "v" + temporal.SDKVersion

// Feature represents a feature that can be executed.
type Feature struct {

	// Set of workflows to register. This can be a single workflow or a slice. You can provide
	// a function pointer, or WorkflowWithOptions.
	Workflows interface{}

	// Set of activities to register. This can be a single activity or a slice. You can provide
	// a function pointer, or ActivityWithOptions.
	Activities interface{}

	// If present, expects workflow to fail with this activity error string.
	ExpectActivityError string

	// If present, expects workflow to succeed with this value.
	ExpectRunResult interface{}

	// Client options for client creation. Some values like HostPort are always
	// overridden internally.
	ClientOptions client.Options

	// Worker options for worker creation. Some values like WorkflowPanicPolicy are always
	// overridden internally. By default, the harness sets the WorkflowPanicPolicy to FailWorkflow -
	// in order to set that one option here you must *also* set the
	// DisableWorkflowPanicPolicyOverride field to true.
	WorkerOptions worker.Options

	// BeforeDial provides a hook that will be called just before calling client.Dial.
	BeforeDial func(runner *Runner) error

	// BeforeWorkerStart provides a hook that will be called just before calling Worker.Start.
	BeforeWorkerStart func(runner *Runner) error

	// Can modify the workflow options that are used by the default executor. Some values such as
	// task queue and workflow execution timeout, are set by default (but may be overridden by this
	// mutator).
	StartWorkflowOptionsMutator func(*client.StartWorkflowOptions)

	// The harness will override the WorkflowPanicPolicy to be FailWorkflow
	// unless this field is set to true, in which case the WorkflowPanicPolicy
	// provided via the StartWorkflowOptions field will be honored.
	DisableWorkflowPanicPolicyOverride bool

	// Default is runner.ExecuteDefault which just runs the first workflow with no
	// params. If this returns a nil run, no replay or checks are performed. This
	// allows for advanced tests that do not want to test history.
	Execute func(ctx context.Context, runner *Runner) (client.WorkflowRun, error)

	// Default is runner.CheckResultDefault which uses Expect fields.
	CheckResult func(ctx context.Context, runner *Runner, run client.WorkflowRun) error

	// Default is runner.CheckHistoryDefault which checks current history and any
	// history files from older versions.
	CheckHistory func(ctx context.Context, runner *Runner, run client.WorkflowRun) error

	// If non-empty, this feature will be skipped without checking any other
	// values.
	SkipReason string

	// ClientUsesProxy indicates if the default client used by test harness
	// should be one that goes through the gRPC proxy
	// instead of talking directly to the server.
	ClientUsesProxy bool

	// WorkerUsesProxy indicates if the client used to run the worker
	// should be one that goes through the gRPC proxy
	// instead of talking directly to the server.
	WorkerUsesProxy bool
}

type WorkflowWithOptions struct {
	Workflow interface{}
	Options  workflow.RegisterOptions
}

type ActivityWithOptions struct {
	Activity interface{}
	Options  activity.RegisterOptions
}

// PreparedFeature represents a feature that has been validated and the
// directory has been derived.
type PreparedFeature struct {
	Feature
	// This is the relative directory beneath features/ and uses only slashes.
	Dir string
	// This is the absolute directory using platform-dependent separators.
	AbsDir     string
	Workflows  []interface{}
	Activities []interface{}
}

func (p *PreparedFeature) GetPrimaryWorkflow() (*WorkflowWithOptions, error) {
	if len(p.Workflows) == 0 {
		return nil, fmt.Errorf("feature missing workflow")
	}
	firstWF := p.Workflows[0]
	switch firstWF.(type) {
	case WorkflowWithOptions:
		asOpts := firstWF.(WorkflowWithOptions)
		return &asOpts, nil
	default:
		return &WorkflowWithOptions{
			Workflow: firstWF,
			Options:  workflow.RegisterOptions{},
		}, nil
	}
}

var registeredFeatures []*PreparedFeature
var registeredFeaturesLock sync.RWMutex

// MustRegisterFeatures registers the given features or panics.
func MustRegisterFeatures(features ...Feature) {
	registeredFeaturesLock.Lock()
	defer registeredFeaturesLock.Unlock()
	for _, feature := range features {
		prepared, err := PrepareFeature(feature)
		if err != nil {
			panic(err)
		}
		registeredFeatures = append(registeredFeatures, prepared)
	}
}

// RegisteredFeatures returns a shallow copy of all registered features.
func RegisteredFeatures() []*PreparedFeature {
	registeredFeaturesLock.RLock()
	defer registeredFeaturesLock.RUnlock()
	ret := make([]*PreparedFeature, len(registeredFeatures))
	copy(ret, registeredFeatures)
	return ret
}

// PrepareFeature prepares the given feature. See PreparedFeature for more info.
func PrepareFeature(feature Feature) (*PreparedFeature, error) {
	p := &PreparedFeature{
		Feature:    feature,
		Workflows:  rawToSlice(feature.Workflows),
		Activities: rawToSlice(feature.Activities),
	}
	// If it's skipped, just return it
	if p.SkipReason != "" {
		return p, nil
	}
	firstWorkflow, err := p.GetPrimaryWorkflow()
	if err != nil {
		return nil, err
	}
	// Use the first the dir of the first workflow
	if p.Dir, p.AbsDir, err = featureDirFromFuncPointer(firstWorkflow.Workflow); err != nil {
		return nil, err
	}
	return p, nil
}

func NoHistoryCheck(context.Context, *Runner, client.WorkflowRun) error {
	return nil
}

var registeredSubprocessCommands []*cli.Command
var registeredSubprocessCommandsMutex sync.RWMutex

// MustRegisterSubprocessCommand registers the given CLI command.
func MustRegisterSubprocessCommand(cmd *cli.Command) {
	registeredSubprocessCommandsMutex.Lock()
	defer registeredSubprocessCommandsMutex.Unlock()
	registeredSubprocessCommands = append(registeredSubprocessCommands, cmd)
}

// GetRegisteredSubprocessCommands gets a copy of all registered subcommands.
func GetRegisteredSubprocessCommands() []*cli.Command {
	registeredSubprocessCommandsMutex.RLock()
	defer registeredSubprocessCommandsMutex.RUnlock()
	return append(make([]*cli.Command, 0, len(registeredSubprocessCommands)), registeredSubprocessCommands...)
}

// CreateSubprocessCommand creates a subprocess command that will invoke a
// registered subprocess.
func CreateSubprocessCommand(ctx context.Context, name string, args ...string) (*exec.Cmd, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("cannot get current executable: %w", err)
	}
	cmd := exec.CommandContext(ctx, exe, append([]string{"go-subprocess", name}, args...)...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd, nil
}

func rawToSlice(v interface{}) []interface{} {
	val := reflect.ValueOf(v)
	if !val.IsValid() {
		return nil
	} else if val.Kind() != reflect.Slice {
		return []interface{}{v}
	}
	ret := make([]interface{}, val.Len())
	for i := 0; i < val.Len(); i++ {
		ret[i] = val.Index(i).Interface()
	}
	return ret
}

func featureDirFromFuncPointer(v interface{}) (relDir, absDir string, err error) {
	val := reflect.ValueOf(v)
	if val.Kind() != reflect.Func {
		return "", "", fmt.Errorf("first workflow %T is not a function", v)
	}
	absDir, _ = runtime.FuncForPC(val.Pointer()).FileLine(val.Pointer())
	absDir = filepath.Dir(absDir)
	slashDir := filepath.ToSlash(absDir)
	// Split and take after "features/.../features" dir.
	// In CI we end up with 3 directory levels names features, locally this would normally be only 2.
	featuresIndex := -1
	dirPieces := strings.Split(slashDir, "/")
	for i, dirPiece := range dirPieces {
		if dirPiece == "features" {
			for j := i; j < len(dirPieces); j++ {
				if dirPieces[j] == "features" {
					featuresIndex = j
				}
			}
			break
		}
	}
	if featuresIndex < 0 {
		return "", "", fmt.Errorf("workflow %T is not in a subdirectory of features", v)
	}
	return path.Join(dirPieces[featuresIndex+1:]...), absDir, nil
}
