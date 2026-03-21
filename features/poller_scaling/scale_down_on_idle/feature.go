package scale_down_on_idle

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/temporalio/features/harness/go/harness"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
	"google.golang.org/grpc"
)

var concurrentPolls atomic.Int32

var Feature = harness.Feature{
	Workflows: Workflow,
	WorkerOptions: worker.Options{
		WorkflowTaskPollerBehavior: worker.NewPollerBehaviorAutoscaling(worker.PollerBehaviorAutoscalingOptions{
			InitialNumberOfPollers: 5,
			MinimumNumberOfPollers: 1,
			MaximumNumberOfPollers: 10,
		}),
	},
	ClientOptions: client.Options{
		ConnectionOptions: client.ConnectionOptions{
			DialOptions: []grpc.DialOption{
				grpc.WithChainUnaryInterceptor(pollInterceptor),
			},
		},
	},
	Execute:      execute,
	CheckHistory: harness.NoHistoryCheck,
}

func pollInterceptor(
	ctx context.Context,
	method string,
	req, reply any,
	cc *grpc.ClientConn,
	invoker grpc.UnaryInvoker,
	opts ...grpc.CallOption,
) error {
	if strings.HasSuffix(method, "PollWorkflowTaskQueue") {
		concurrentPolls.Add(1)
		defer concurrentPolls.Add(-1)
	}
	return invoker(ctx, method, req, reply, cc, opts...)
}

func execute(ctx context.Context, r *harness.Runner) (client.WorkflowRun, error) {
	descNs, err := r.Client.WorkflowService().DescribeNamespace(ctx, &workflowservice.DescribeNamespaceRequest{
		Namespace: r.Namespace,
	})
	if err != nil {
		return nil, fmt.Errorf("DescribeNamespace: %w", err)
	}
	caps := descNs.GetNamespaceInfo().GetCapabilities()
	if !caps.GetPollerAutoscaling() {
		return nil, r.Skip("server does not support poller autoscaling")
	}

	// Wait for concurrent polls to reach a peak (at least 3 of the 5 initial pollers).
	var peakPolls int32
	err = r.DoUntilEventually(ctx, 500*time.Millisecond, 30*time.Second, func() bool {
		current := concurrentPolls.Load()
		if current > peakPolls {
			peakPolls = current
		}
		r.Log.Info("Waiting for pollers to start", "current", current, "peak", peakPolls)
		return peakPolls >= 3
	})
	if err != nil {
		return nil, fmt.Errorf("pollers did not reach expected initial count (peak: %d): %w", peakPolls, err)
	}
	r.Log.Info("Peak concurrent polls reached", "peak", peakPolls)

	// Wait for concurrent polls to decrease. The queue is idle, so the SDK
	// should scale down once it sees the PollerAutoscaling capability on empty
	// poll responses and decides to reduce pollers.
	var lastSeen int32
	err = r.DoUntilEventually(ctx, 3*time.Second, 120*time.Second, func() bool {
		current := concurrentPolls.Load()
		r.Log.Info("Waiting for scale-down", "current", current, "peak", peakPolls)
		lastSeen = current
		return current < peakPolls
	})
	if err != nil {
		return nil, fmt.Errorf("pollers did not scale down from peak %d (last seen: %d): %w",
			peakPolls, lastSeen, err)
	}

	r.Log.Info("Pollers scaled down", "peak", peakPolls, "final", lastSeen)
	return nil, nil
}

func Workflow(ctx workflow.Context) error {
	return nil
}
