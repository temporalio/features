package self

import (
	"context"
	"fmt"
	"time"

	"github.com/temporalio/features/features/update/updateutil"
	"github.com/temporalio/features/harness/go/harness"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	updateName                = "update!"
	updateNotEnabledErrorType = "PermissionDenied"
)

type ConnMaterial struct {
	HostPort       string
	Namespace      string
	Identity       string
	ClientCertPath string
	ClientKeyPath  string
	TLSServerName  string
}

var Feature = harness.Feature{
	Workflows:  SelfUpdateWorkflow,
	Activities: SelfUpdateActivity,
	Execute: func(ctx context.Context, runner *harness.Runner) (client.WorkflowRun, error) {
		if reason := updateutil.CheckServerSupportsUpdate(ctx, runner.Client); reason != "" {
			return nil, runner.Skip(reason)
		}
		opts := client.StartWorkflowOptions{
			TaskQueue:                runner.TaskQueue,
			WorkflowExecutionTimeout: 1 * time.Minute,
		}
		runner.Feature.StartWorkflowOptionsMutator(&opts)
		return runner.Client.ExecuteWorkflow(ctx, opts, SelfUpdateWorkflow, ConnMaterial{
			HostPort:       runner.Feature.ClientOptions.HostPort,
			Namespace:      runner.Feature.ClientOptions.Namespace,
			Identity:       runner.Feature.ClientOptions.Identity,
			ClientCertPath: runner.ClientCertPath,
			ClientKeyPath:  runner.ClientKeyPath,
			TLSServerName:  runner.TLSServerName,
		})
	},
}

func SelfUpdateWorkflow(ctx workflow.Context, cm ConnMaterial) (string, error) {
	const expectedState = "called"
	state := "not " + expectedState
	workflow.SetUpdateHandler(ctx, updateName, func(ctx workflow.Context) error {
		state = expectedState
		return nil
	})
	err := workflow.ExecuteActivity(
		workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
			StartToCloseTimeout: 5 * time.Second,
			RetryPolicy: &temporal.RetryPolicy{
				NonRetryableErrorTypes: []string{updateNotEnabledErrorType},
			},
		}),
		SelfUpdateActivity,
		cm,
	).Get(ctx, nil)
	if err != nil {
		return "", err
	}
	if state != expectedState {
		return "", fmt.Errorf("expected state == %q but found %q", expectedState, state)
	}
	return state, nil
}

func SelfUpdateActivity(ctx context.Context, cm ConnMaterial) error {
	tlsCfg, err := harness.LoadTLSConfig(cm.ClientCertPath, cm.ClientKeyPath, cm.TLSServerName)
	if err != nil {
		return err
	}
	c, err := client.Dial(
		client.Options{
			HostPort:          cm.HostPort,
			Namespace:         cm.Namespace,
			Logger:            activity.GetLogger(ctx),
			MetricsHandler:    activity.GetMetricsHandler(ctx),
			Identity:          cm.Identity,
			ConnectionOptions: client.ConnectionOptions{TLS: tlsCfg},
		},
	)
	if err != nil {
		return err
	}
	wfe := activity.GetInfo(ctx).WorkflowExecution
	updateHandle, err := c.UpdateWorkflow(
		ctx,
		client.UpdateWorkflowOptions{
			WorkflowID:   wfe.ID,
			RunID:        wfe.RunID,
			WaitForStage: client.WorkflowUpdateStageCompleted,
			UpdateName:   updateName,
		},
	)
	if err != nil {
		return err
	}
	return updateHandle.Get(ctx, nil)
}
