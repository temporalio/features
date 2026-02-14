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
	CACertPath     string
	TLSServerName  string
}

var Feature = harness.Feature{
	Workflows:  SelfUpdateWorkflow,
	Activities: SelfUpdateActivity,
	Execute: func(ctx context.Context, runner *harness.Runner) (client.WorkflowRun, error) {
		runner.Log.Info("Starting self update test execution")

		if reason := updateutil.CheckServerSupportsUpdate(ctx, runner.Client); reason != "" {
			return nil, runner.Skip(reason)
		}

		runner.Log.Info("Starting workflow execution (workflow will update itself via activity)")
		opts := client.StartWorkflowOptions{
			TaskQueue:                runner.TaskQueue,
			WorkflowExecutionTimeout: 1 * time.Minute,
		}
		runner.Feature.StartWorkflowOptionsMutator(&opts)
		run, err := runner.Client.ExecuteWorkflow(ctx, opts, SelfUpdateWorkflow, ConnMaterial{
			HostPort:       runner.Feature.ClientOptions.HostPort,
			Namespace:      runner.Feature.ClientOptions.Namespace,
			Identity:       runner.Feature.ClientOptions.Identity,
			ClientCertPath: runner.ClientCertPath,
			ClientKeyPath:  runner.ClientKeyPath,
			CACertPath:     runner.CACertPath,
			TLSServerName:  runner.TLSServerName,
		})
		if err == nil {
			runner.Log.Info("Workflow started", "workflowID", run.GetID(), "runID", run.GetRunID())
		}
		return run, err
	},
}

func SelfUpdateWorkflow(ctx workflow.Context, cm ConnMaterial) (string, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("SelfUpdateWorkflow started")

	const expectedState = "called"
	state := "not " + expectedState

	logger.Info("Setting up update handler")
	workflow.SetUpdateHandler(ctx, updateName, func(ctx workflow.Context) error {
		logger.Info("Update handler invoked (called from activity)")
		state = expectedState
		logger.Info("Update handler completed, state updated")
		return nil
	})

	logger.Info("Starting activity that will update this workflow")
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
		logger.Error("Activity failed", "error", err)
		return "", err
	}
	logger.Info("Activity completed successfully")

	if state != expectedState {
		logger.Error("State validation failed", "expected", expectedState, "actual", state)
		return "", fmt.Errorf("expected state == %q but found %q", expectedState, state)
	}
	logger.Info("Workflow completed successfully", "state", state)
	return state, nil
}

func SelfUpdateActivity(ctx context.Context, cm ConnMaterial) error {
	logger := activity.GetLogger(ctx)
	logger.Info("SelfUpdateActivity started")

	tlsCfg, err := harness.LoadTLSConfig(cm.ClientCertPath, cm.ClientKeyPath, cm.CACertPath, cm.TLSServerName)
	if err != nil {
		logger.Error("Failed to load TLS config", "error", err)
		return err
	}

	logger.Info("Dialing Temporal client")
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
		logger.Error("Failed to dial client", "error", err)
		return err
	}

	wfe := activity.GetInfo(ctx).WorkflowExecution
	logger.Info("Sending update to own workflow", "workflowID", wfe.ID, "runID", wfe.RunID, "updateName", updateName)

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
		logger.Error("Failed to send update", "error", err)
		return err
	}
	logger.Info("Update request sent, waiting for completion")

	err = updateHandle.Get(ctx, nil)
	if err != nil {
		logger.Error("Update failed", "error", err)
		return err
	}
	logger.Info("Update completed successfully", "updateID", updateHandle.UpdateID())
	return nil
}
