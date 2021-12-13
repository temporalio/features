package history

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/filter/v1"
	"go.temporal.io/api/history/v1"
	"go.temporal.io/api/workflow/v1"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
)

// Fetcher fetches histories.
type Fetcher struct {
	Client    client.Client
	Namespace string
	TaskQueue string
	// Approximate value, is given leeway
	FeatureStarted time.Time
}

// Fetch returns histories sorted via Histories.Sort. This will continually try
// to find at least one completed workflow or fail.
func (f *Fetcher) Fetch(ctx context.Context) (Histories, error) {
	// Collect executions. Try until there are no open executions and at least one
	// closed execution. The reason we do this is the server can still show a
	// workflow as not complete or not present even though the SDK has been told
	// it is complete.
	// TODO(cretz): Provide way to ignore specific still-open workflows. Maybe
	// they can have a header or a name-prefix or something else.
	var execs []*workflow.WorkflowExecutionInfo
	var stillRunning []string
	const maxOpenWait = 5 * time.Second
	for start := time.Now(); time.Since(start) < maxOpenWait; {
		var err error
		execs, err = f.GetExecutions(ctx)
		if err != nil {
			return nil, err
		}
		stillRunning = stillRunning[:0]
		for _, exec := range execs {
			if exec.Status == enums.WORKFLOW_EXECUTION_STATUS_RUNNING {
				stillRunning = append(stillRunning,
					fmt.Sprintf("%v (run: %v)", exec.Execution.WorkflowId, exec.Execution.RunId))
			}
		}
		if len(stillRunning) == 0 && len(execs) > 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if len(stillRunning) > 0 {
		return nil, fmt.Errorf("after %v, %v workflow(s) are still running: %v", maxOpenWait, len(stillRunning),
			strings.Join(stillRunning, ", "))
	} else if len(execs) == 0 {
		return nil, fmt.Errorf("after %v, no workflow(s) found", maxOpenWait)
	}

	// Collect histories
	var ret Histories
	for _, exec := range execs {
		// All workflows must be done
		if exec.Status == enums.WORKFLOW_EXECUTION_STATUS_RUNNING {
			return nil, fmt.Errorf("workflow %v (id %v, run id %v) is still running", exec.Type.Name,
				exec.Execution.WorkflowId, exec.Execution.RunId)
		}
		var hist history.History
		iter := f.Client.GetWorkflowHistory(ctx, exec.Execution.WorkflowId, exec.Execution.RunId, false,
			enums.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT)
		for iter.HasNext() {
			event, err := iter.Next()
			if err != nil {
				return nil, fmt.Errorf("failed getting next history event: %w", err)
			}
			hist.Events = append(hist.Events, event)
		}
		ret = append(ret, &hist)
	}

	// Sort and return
	ret.Sort()
	return ret, nil
}

// GetExecutions returns all open/closed executions.
func (f *Fetcher) GetExecutions(ctx context.Context) ([]*workflow.WorkflowExecutionInfo, error) {
	// Get open and closed workflows within a minute of when the runner started
	earliest := f.FeatureStarted.Add(-5 * time.Minute)
	var execs []*workflow.WorkflowExecutionInfo
	seenExecs := map[string]bool{}
	var nextPageToken []byte
	for {
		resp, err := f.Client.ListOpenWorkflow(ctx, &workflowservice.ListOpenWorkflowExecutionsRequest{
			Namespace:       f.Namespace,
			MaximumPageSize: 1000,
			NextPageToken:   nextPageToken,
			StartTimeFilter: &filter.StartTimeFilter{EarliestTime: &earliest},
		})
		if err != nil {
			return nil, fmt.Errorf("failed listing workflows: %w", err)
		}
		for _, exec := range resp.Executions {
			seenKey := exec.Execution.WorkflowId + "_||_" + exec.Execution.RunId
			if exec.TaskQueue == f.TaskQueue && !seenExecs[seenKey] {
				execs = append(execs, exec)
				seenExecs[seenKey] = true
			}
		}
		if nextPageToken = resp.NextPageToken; len(nextPageToken) == 0 {
			break
		}
	}
	for {
		resp, err := f.Client.ListClosedWorkflow(ctx, &workflowservice.ListClosedWorkflowExecutionsRequest{
			Namespace:       f.Namespace,
			MaximumPageSize: 1000,
			NextPageToken:   nextPageToken,
			StartTimeFilter: &filter.StartTimeFilter{EarliestTime: &earliest},
		})
		if err != nil {
			return nil, fmt.Errorf("failed listing workflows: %w", err)
		}
		for _, exec := range resp.Executions {
			seenKey := exec.Execution.WorkflowId + "_||_" + exec.Execution.RunId
			if exec.TaskQueue == f.TaskQueue && !seenExecs[seenKey] {
				execs = append(execs, exec)
				seenExecs[seenKey] = true
			}
		}
		if nextPageToken = resp.NextPageToken; len(nextPageToken) == 0 {
			break
		}
	}
	return execs, nil
}
