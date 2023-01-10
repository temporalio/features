//go:build !pre1.12.0

package activity_start_race

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"go.temporal.io/features/harness/go/harness"
	"go.temporal.io/features/harness/go/history"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
	"google.golang.org/grpc"
)

var Feature = harness.Feature{
	Workflows:  Workflow,
	Activities: SleepActivity,
	Execute:    Execute,
	ClientOptions: client.Options{
		ConnectionOptions: client.ConnectionOptions{
			DialOptions: []grpc.DialOption{grpc.WithUnaryInterceptor(delayWorkflow)},
		},
	},
	CheckHistory: CheckHistory,
}

var pollWorkflowWait, pollActivityWait sync.WaitGroup

const activitySleep = 200 * time.Millisecond

func Execute(ctx context.Context, r *harness.Runner) (client.WorkflowRun, error) {
	// Make the next activity poll wait
	pollActivityWait.Add(1)

	// Start the workflow
	run, err := r.ExecuteDefault(ctx)
	if err != nil {
		return nil, err
	}

	// Make workflow poll wait
	pollWorkflowWait.Add(1)

	// Resume workflow
	r.Require.NoError(r.Client.SignalWorkflow(ctx, run.GetID(), run.GetRunID(), "resume", nil))

	// Wait a bit, let activity poll succeed
	time.Sleep(activitySleep * 2)
	pollActivityWait.Done()

	// Wait a bit, then let workflow poll succeed
	time.Sleep(activitySleep * 4)
	pollWorkflowWait.Done()

	// Wait a bit, then cancel the workflow
	time.Sleep(activitySleep * 2)
	r.Require.NoError(r.Client.CancelWorkflow(ctx, run.GetID(), run.GetRunID()))
	return run, err
}

func CheckHistory(ctx context.Context, r *harness.Runner, run client.WorkflowRun) error {
	// We want to do the default history check which just does a replay of the
	// history. However, we have changed the filename because the history is
	// non-deterministic (it can appear in different order based on timing). But
	// we have captured a specific history to attempt to replay the error. See the
	// README for more details.

	// Load file
	_, currFile, _, _ := runtime.Caller(0)
	var hist history.Histories
	if b, err := os.ReadFile(filepath.Join(currFile, "../history/history.manual.json")); err != nil {
		return fmt.Errorf("failed reading history JSON: %w", err)
	} else if err = json.Unmarshal(b, &hist); err != nil {
		return fmt.Errorf("failed unmarshaling history JSON: %w", err)
	}
	r.Log.Debug("Checking history replay")
	if err := r.ReplayHistories(ctx, hist); err != nil {
		return fmt.Errorf("replay failed: %w", err)
	}
	return nil
}

func delayWorkflow(
	ctx context.Context,
	method string,
	req interface{},
	reply interface{},
	cc *grpc.ClientConn,
	invoker grpc.UnaryInvoker,
	opts ...grpc.CallOption,
) error {
	// Wait if polling workflow task queue
	if strings.HasSuffix(method, "PollWorkflowTaskQueue") {
		pollWorkflowWait.Wait()
	} else if strings.HasSuffix(method, "PollActivityTaskQueue") {
		pollActivityWait.Wait()
	}

	return invoker(ctx, method, req, reply, cc, opts...)
}

func Workflow(ctx workflow.Context) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: 1 * time.Minute})
	// Receive signals to execute activity
	signalCh := workflow.GetSignalChannel(ctx, "resume")
	for {
		// Execute activity but do not wait on result
		workflow.ExecuteActivity(ctx, SleepActivity)

		// Wait for signal or done
		selector := workflow.NewSelector(ctx)
		var done bool
		selector.AddReceive(ctx.Done(), func(c workflow.ReceiveChannel, more bool) {
			c.Receive(ctx, nil)
			done = true
		})
		selector.AddReceive(signalCh, func(c workflow.ReceiveChannel, more bool) {
			c.Receive(ctx, nil)
		})
		selector.Select(ctx)
		if done {
			return nil
		}
	}
}

func SleepActivity(context.Context) error {
	time.Sleep(activitySleep)
	return nil
}
