package successful_start

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/temporalio/features/harness/go/harness"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
	"google.golang.org/grpc"
)

const expectedResult = "Hello World"

var numEagerlyStarted atomic.Uint64

var Feature = harness.Feature{
	Workflows:            Workflow,
	StartWorkflowOptions: client.StartWorkflowOptions{EnableEagerStart: true, WorkflowTaskTimeout: 1 * time.Hour},
	CheckResult:          CheckResult,
	ClientOptions: client.Options{
		ConnectionOptions: client.ConnectionOptions{
			DialOptions: []grpc.DialOption{grpc.WithUnaryInterceptor(EagerDetector(&numEagerlyStarted))},
		},
	},
}

// A "hello world" workflow
func Workflow(ctx workflow.Context) (string, error) {
	return expectedResult, nil
}

func CheckResult(ctx context.Context, runner *harness.Runner, run client.WorkflowRun) error {
	var result string
	if err := run.Get(ctx, &result); err != nil {
		return err
	}
	if result != expectedResult {
		return fmt.Errorf("expected %s, got: %s", expectedResult, result)
	}
	if numEager := numEagerlyStarted.Load(); numEager != 1 {
		// There is no way to check that this dynamic config is enabled in the namespace,
		// unless we run this test...
		// Instead of failing the test just skip it.
		msg := fmt.Sprintf("Enable dynamic config system.enableEagerWorkflowStart=true: numEagerlyStarted=%d", numEager)
		return runner.Skip(msg)
	}
	return nil
}

func EagerDetector(cntEager *atomic.Uint64) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, response interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		request_eager := false
		switch o := req.(type) {
		case *workflowservice.StartWorkflowExecutionRequest:
			request_eager = o.RequestEagerExecution
		}

		err := invoker(ctx, method, req, response, cc, opts...)
		if err != nil {
			return err
		}

		switch o := response.(type) {
		case *workflowservice.StartWorkflowExecutionResponse:
			if request_eager && o.GetEagerWorkflowTask() != nil {
				cntEager.Add(1)
			}
		}

		return nil
	}
}
