package harness

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/worker"
)

// SDKVersion is the Go SDK version with the "v" prefix.
const SDKVersion = "v" + temporal.SDKVersion

// Feature represents a feature that can be executed.
type Feature struct {

	// Set of workflows to register. This can be a single workflow or a slice.
	Workflows interface{}

	// Set of activities to register. This can be a single activity or a slice.
	Activities interface{}

	// If present, expects workflow to fail with this activity error string.
	ExpectActivityError string

	// If present, expects workflow to succeed with this value.
	ExpectRunResult interface{}

	// Client options for client creation. Some values like HostPort are always
	// overridden internally.
	ClientOptions client.Options

	// Worker options for worker creation. Some values like WorkflowPanicPolicy
	// are always overridden internally.
	WorkerOptions worker.Options

	// Default is runner.ExecuteDefault which just runs the first workflow with no
	// params. If this returns a nil run, no replay or checks are performed. This
	// allows for advanced tests that do not want to test history.
	Execute func(ctx context.Context, runner *Runner) (client.WorkflowRun, error)

	// Default is runner.CheckResultDefault which uses Expect fields.
	CheckResult func(ctx context.Context, runner *Runner, run client.WorkflowRun) error

	// Default is runner.CheckHistoryDefault which checks current history and any
	// history files from older versions.
	CheckHistory func(ctx context.Context, runner *Runner, run client.WorkflowRun) error

	// If non-empty, this feature will be skiped without checking any other
	// values.
	SkipReason string
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

// PrepareFeature prepares the given feature.
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
	if len(p.Workflows) == 0 {
		return nil, fmt.Errorf("feature missing workflow")
	}
	// Use the first the dir of the first workflow
	var err error
	if p.Dir, p.AbsDir, err = featureDirFromFuncPointer(p.Workflows[0]); err != nil {
		return nil, err
	}
	return p, nil
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
	// Split and take after first "features" dir
	featuresIndex := -1
	dirPieces := strings.Split(slashDir, "/")
	for i, dirPiece := range dirPieces {
		if dirPiece == "features" {
			featuresIndex = i
			break
		}
	}
	if featuresIndex < 0 {
		return "", "", fmt.Errorf("workflow %T is not in a subdirectory of features", v)
	}
	return path.Join(dirPieces[featuresIndex+1:]...), absDir, nil
}
