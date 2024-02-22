package harness

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	commonpb "go.temporal.io/api/common/v1"
	enumspb "go.temporal.io/api/enums/v1"
	historypb "go.temporal.io/api/history/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/log"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

// RetryDisabled is a retry policy with 1 max attempt.
var RetryDisabled = &temporal.RetryPolicy{MaximumAttempts: 1}

// AppErrorf creates a formatted retryable application error.
func AppErrorf(msg string, args ...interface{}) error {
	return temporal.NewApplicationError(fmt.Sprintf(msg, args...), "SdkFeaturesError")
}

// Find the first event in the history that meets the condition.
func FindEvent(history client.HistoryEventIterator, cond func(*historypb.HistoryEvent) bool) (*historypb.HistoryEvent, error) {
	for history.HasNext() {
		ev, err := history.Next()
		if err != nil {
			return nil, err
		}
		if cond(ev) {
			return ev, nil
		}
	}
	return nil, nil
}

// ExecuteWithArgs runs a workflow with default arguments
func ExecuteWithArgs(workflow interface{}, args ...interface{}) func(ctx context.Context, r *Runner) (client.WorkflowRun, error) {
	return func(ctx context.Context, r *Runner) (client.WorkflowRun, error) {
		opts := client.StartWorkflowOptions{
			TaskQueue:                r.TaskQueue,
			WorkflowExecutionTimeout: 1 * time.Minute,
		}
		r.Feature.StartWorkflowOptionsMutator(&opts)
		return r.Client.ExecuteWorkflow(ctx, opts, workflow, args...)
	}
}

// GetWorkflowResultPayload returns the first payload of a workflow result
func GetWorkflowResultPayload(
	ctx context.Context,
	client client.Client,
	workflowID string,
) (*commonpb.Payload, error) {
	history := client.GetWorkflowHistory(ctx, workflowID, "", false, enumspb.HISTORY_EVENT_FILTER_TYPE_CLOSE_EVENT)

	event, err := FindEvent(history, func(ev *historypb.HistoryEvent) bool {
		attrs := ev.GetWorkflowExecutionCompletedEventAttributes()
		return attrs != nil
	})
	if err != nil {
		return nil, err
	}
	if event == nil {
		return nil, errors.New("missing WorkflowExecutionCompleted event")
	}

	attrs := event.GetWorkflowExecutionCompletedEventAttributes()
	if attrs == nil {
		return nil, errors.New("could not locate WorkflowExecutionCompleted event")
	}
	return attrs.GetResult().GetPayloads()[0], nil
}

// GetWorkflowArgumentPayload returns the first payload of a workflow argument
func GetWorkflowArgumentPayload(
	ctx context.Context,
	client client.Client,
	workflowID string,
) (*commonpb.Payload, error) {
	history := client.GetWorkflowHistory(ctx, workflowID, "", false, enumspb.HISTORY_EVENT_FILTER_TYPE_UNSPECIFIED)

	event, err := FindEvent(history, func(ev *historypb.HistoryEvent) bool {
		attrs := ev.GetWorkflowExecutionStartedEventAttributes()
		return attrs != nil
	})
	if err != nil {
		return nil, err
	}
	if event == nil {
		return nil, errors.New("missing WorkflowExecutionStarted event")
	}

	attrs := event.GetWorkflowExecutionStartedEventAttributes()
	if attrs == nil {
		return nil, errors.New("could not locate WorkflowExecutionStarted event")
	}
	return attrs.GetInput().GetPayloads()[0], nil
}

func CountEvents(history client.HistoryEventIterator, cond func(*historypb.HistoryEvent) bool) (int, error) {
	count := 0
	for history.HasNext() {
		ev, err := history.Next()
		if err != nil {
			return -1, err
		}
		if cond(ev) {
			count += 1
		}
	}
	return count, nil
}

func GetCountCompletedUpdates(ctx context.Context, client client.Client, workflowID string) (int, error) {
	history := client.GetWorkflowHistory(ctx, workflowID, "", false, enumspb.HISTORY_EVENT_FILTER_TYPE_UNSPECIFIED)
	return CountEvents(history, func(ev *historypb.HistoryEvent) bool {
		attrs := ev.GetWorkflowExecutionUpdateCompletedEventAttributes()
		return attrs != nil
	})
}

// WaitNamespaceAvailable waits for up to 5 seconds for the provided namespace to become available
func WaitNamespaceAvailable(ctx context.Context, logger log.Logger,
	hostPortStr, namespace, clientCertPath, clientKeyPath string) error {
	logger.Info("Waiting for namespace to become available", "namespace", namespace)

	var myClient client.Client
	defer func() {
		if myClient != nil {
			myClient.Close()
		}
	}()
	tlsCfg, err := LoadTLSConfig(clientCertPath, clientKeyPath)
	clientOpts := client.Options{HostPort: hostPortStr, Namespace: namespace, Logger: logger}
	clientOpts.ConnectionOptions.TLS = tlsCfg
	if err != nil {
		return err
	}
	lastErr := RetryFor(600, 100*time.Millisecond, func() (bool, error) {
		if myClient == nil {
			var clientErr error
			myClient, clientErr = client.Dial(clientOpts)
			if clientErr != nil {
				return false, clientErr
			}
		}
		_, clientErr := myClient.DescribeWorkflowExecution(ctx, "!sonotreal", "superneverexistwf!")
		if clientErr != nil {
			if strings.Contains(clientErr.Error(), "Invalid RunId") ||
				strings.Contains(clientErr.Error(), "operation GetCurrentExecution") {
				return true, nil
			}
		}
		return false, clientErr
	})
	if lastErr != nil {
		return fmt.Errorf("failed connecting / describing namespace after 5s, last error: %w", lastErr)
	}
	if _, ok := os.LookupEnv("WAIT_EXTRA_FOR_NAMESPACE"); !ok {
		return nil
	}
	logger.Info("Confirming namespace availability with workflow", "namespace", namespace)

	// Now we need to really _really_ make sure that this namespace actually works by running a
	// dummy workflow. Hopefully we can remove all this when
	// https://github.com/temporalio/temporal/issues/1336 is fixed
	var dummyReturn string
	dummyTq := fmt.Sprintf("dummy-wf-tq-%s", uuid.New())
	dummyWorker := worker.New(myClient, dummyTq, worker.Options{})
	dummyWorker.RegisterWorkflow(dummyWorkflow)
	err = dummyWorker.Start()
	if err != nil {
		return fmt.Errorf("failed to start namespace-ready-checking dummy worker: %w", err)
	}
	defer func() {
		dummyWorker.Stop()
	}()
	lastErr = RetryFor(600, 1*time.Second, func() (bool, error) {
		run, err := myClient.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
			ID:                       fmt.Sprintf("dummy-wf-%s", uuid.New()),
			TaskQueue:                dummyTq,
			WorkflowExecutionTimeout: 10 * time.Second,
		}, dummyWorkflow)
		if err != nil {
			return false, err
		}
		err = run.Get(ctx, &dummyReturn)
		if err != nil {
			return false, err
		}
		return true, nil
	})
	if lastErr != nil {
		return fmt.Errorf("failed to run namespace-ready-checking workflow: %w", lastErr)
	}

	return nil
}

func dummyWorkflow(_ workflow.Context) (string, error) {
	return "hello", nil
}

// RetryFor retries some function until it passes or we run out of attempts. Wait interval between
// attempts.
func RetryFor(maxAttempts int, interval time.Duration, cond func() (bool, error)) error {
	var lastErr error
	for i := 0; i < maxAttempts; i++ {
		if ok, curE := cond(); ok {
			return nil
		} else {
			lastErr = curE
		}
		time.Sleep(interval)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("failed after %d attempts", maxAttempts)
	}
	return lastErr
}

// LoadTLSConfig inits a TLS config from the provided cert and key files.
func LoadTLSConfig(clientCertPath, clientKeyPath string) (*tls.Config, error) {
	if clientCertPath != "" {
		if clientKeyPath == "" {
			return nil, errors.New("got TLS cert with no key")
		}
		cert, err := tls.LoadX509KeyPair(clientCertPath, clientKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load certs: %s", err)
		}
		return &tls.Config{Certificates: []tls.Certificate{cert}}, nil
	} else if clientKeyPath != "" {
		return nil, errors.New("got TLS key with no cert")
	}
	return nil, nil
}
