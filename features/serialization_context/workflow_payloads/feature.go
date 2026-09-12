package workflow_payloads

import (
	"context"

	"github.com/temporalio/features/features/serialization_context/sercontext"
	"github.com/temporalio/features/features/update/updateutil"
	"github.com/temporalio/features/harness/go/harness"
	enumspb "go.temporal.io/api/enums/v1"
	historypb "go.temporal.io/api/history/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
)

const (
	signalName = "append"
	queryName  = "prefixed"
	updateName = "suffixed"
	memoKey    = "ser-ctx-memo"

	workflowInput   = "input"
	memoValue       = "memo"
	queryArg        = "query-"
	updateArg       = "-update"
	signalData      = "signal"
	sideEffectValue = "side"
)

var Feature = harness.Feature{
	Workflows:     Workflow,
	ClientOptions: sercontext.ClientOptions(),
	StartWorkflowOptionsMutator: func(opts *client.StartWorkflowOptions) {
		opts.Memo = map[string]interface{}{memoKey: memoValue}
	},
	Execute:      Execute,
	CheckResult:  CheckResult,
	CheckHistory: harness.NoHistoryCheck,
}

// Workflow exercises every workflow scoped payload: its own input and result, a
// side effect, a signal, a query and an update.
func Workflow(ctx workflow.Context, input string) (string, error) {
	err := workflow.SetQueryHandler(ctx, queryName, func(prefix string) (string, error) {
		return prefix + input, nil
	})
	if err != nil {
		return "", err
	}

	err = workflow.SetUpdateHandler(ctx, updateName, func(ctx workflow.Context, suffix string) (string, error) {
		return input + suffix, nil
	})
	if err != nil {
		return "", err
	}

	var sideEffect string
	if err := workflow.SideEffect(ctx, func(workflow.Context) interface{} {
		return sideEffectValue
	}).Get(&sideEffect); err != nil {
		return "", err
	}

	var signaled string
	workflow.GetSignalChannel(ctx, signalName).Receive(ctx, &signaled)

	return input + "|" + sideEffect + "|" + signaled, nil
}

func Execute(ctx context.Context, runner *harness.Runner) (client.WorkflowRun, error) {
	if reason := updateutil.CheckServerSupportsUpdate(ctx, runner.Client); reason != "" {
		return nil, runner.Skip(reason)
	}

	run, err := harness.ExecuteWithArgs(Workflow, workflowInput)(ctx, runner)
	if err != nil {
		return nil, err
	}

	queryValue, err := runner.Client.QueryWorkflow(ctx, run.GetID(), run.GetRunID(), queryName, queryArg)
	runner.Require.NoError(err)
	var queryResult string
	runner.Require.NoError(queryValue.Get(&queryResult))
	runner.Require.Equal(queryArg+workflowInput, queryResult)

	handle, err := runner.Client.UpdateWorkflow(ctx, client.UpdateWorkflowOptions{
		WorkflowID:   run.GetID(),
		RunID:        run.GetRunID(),
		UpdateName:   updateName,
		Args:         []interface{}{updateArg},
		WaitForStage: client.WorkflowUpdateStageCompleted,
	})
	runner.Require.NoError(err)
	var updateResult string
	runner.Require.NoError(handle.Get(ctx, &updateResult))
	runner.Require.Equal(workflowInput+updateArg, updateResult)

	runner.Require.NoError(runner.Client.SignalWorkflow(ctx, run.GetID(), run.GetRunID(), signalName, signalData))
	return run, nil
}

func CheckResult(ctx context.Context, runner *harness.Runner, run client.WorkflowRun) error {
	var result string
	if err := run.Get(ctx, &result); err != nil {
		return err
	}
	runner.Require.Equal(workflowInput+"|"+sideEffectValue+"|"+signalData, result)

	events, err := sercontext.Events(ctx, runner.Client, run.GetID(), run.GetRunID())
	if err != nil {
		return err
	}
	expected := sercontext.WorkflowSignature(runner.Namespace, run.GetID())

	started, err := sercontext.FindEvent(events, "WorkflowExecutionStarted", func(e *historypb.HistoryEvent) bool {
		return e.GetWorkflowExecutionStartedEventAttributes() != nil
	})
	if err != nil {
		return err
	}
	startedAttrs := started.GetWorkflowExecutionStartedEventAttributes()
	runner.Require.Equal(expected, sercontext.FirstSignature(startedAttrs.GetInput()))
	runner.Require.Equal(expected, sercontext.SignatureOf(startedAttrs.GetMemo().GetFields()[memoKey]))

	completed, err := sercontext.FindEvent(events, "WorkflowExecutionCompleted", func(e *historypb.HistoryEvent) bool {
		return e.GetWorkflowExecutionCompletedEventAttributes() != nil
	})
	if err != nil {
		return err
	}
	runner.Require.Equal(expected,
		sercontext.FirstSignature(completed.GetWorkflowExecutionCompletedEventAttributes().GetResult()))

	signaled, err := sercontext.FindEvent(events, "WorkflowExecutionSignaled", func(e *historypb.HistoryEvent) bool {
		return e.GetWorkflowExecutionSignaledEventAttributes() != nil
	})
	if err != nil {
		return err
	}
	runner.Require.Equal(expected,
		sercontext.FirstSignature(signaled.GetWorkflowExecutionSignaledEventAttributes().GetInput()))

	sideEffect, err := sercontext.FindEvent(events, "SideEffect marker", func(e *historypb.HistoryEvent) bool {
		return e.GetMarkerRecordedEventAttributes().GetMarkerName() == "SideEffect"
	})
	if err != nil {
		return err
	}
	runner.Require.Equal(expected,
		sercontext.FirstSignature(sideEffect.GetMarkerRecordedEventAttributes().GetDetails()["data"]))

	accepted, err := sercontext.FindEvent(events, "WorkflowExecutionUpdateAccepted", func(e *historypb.HistoryEvent) bool {
		return e.GetEventType() == enumspb.EVENT_TYPE_WORKFLOW_EXECUTION_UPDATE_ACCEPTED
	})
	if err != nil {
		return err
	}
	acceptedAttrs := accepted.GetWorkflowExecutionUpdateAcceptedEventAttributes()
	runner.Require.Equal(expected,
		sercontext.FirstSignature(acceptedAttrs.GetAcceptedRequest().GetInput().GetArgs()))

	updateCompleted, err := sercontext.FindEvent(events, "WorkflowExecutionUpdateCompleted", func(e *historypb.HistoryEvent) bool {
		return e.GetEventType() == enumspb.EVENT_TYPE_WORKFLOW_EXECUTION_UPDATE_COMPLETED
	})
	if err != nil {
		return err
	}
	runner.Require.Equal(expected, sercontext.FirstSignature(
		updateCompleted.GetWorkflowExecutionUpdateCompletedEventAttributes().GetOutcome().GetSuccess()))

	return nil
}
