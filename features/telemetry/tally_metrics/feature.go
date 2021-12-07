//go:build !pre1.11.0

package tally_metrics

import (
	"context"
	"strings"
	"time"

	"github.com/uber-go/tally/v4"
	"go.temporal.io/sdk-features/harness/go/harness"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
)

var Feature = harness.Feature{
	Workflows:  Workflow,
	Activities: CommaJoin,
	ClientOptions: client.Options{
		MetricsScope: tally.NewTestScope("", nil),
	},
	ExpectRunResult: "foo, bar, baz",
	CheckResult:     CheckResult,
}

func Workflow(ctx workflow.Context) (string, error) {
	// Update our own counter with a custom tag
	workflow.GetMetricsScope(ctx).Tagged(map[string]string{"mytag": "mytagvalue"}).Counter("my_workflow_counter").Inc(2)

	// Run two activities
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{ScheduleToCloseTimeout: 1 * time.Minute})
	var str string
	if err := workflow.ExecuteActivity(ctx, CommaJoin, []string{"foo", "bar"}).Get(ctx, &str); err != nil {
		return "", err
	} else if err := workflow.ExecuteActivity(ctx, CommaJoin, []string{str, "baz"}).Get(ctx, &str); err != nil {
		return "", err
	}
	return str, nil
}

func CommaJoin(ctx context.Context, strs []string) (string, error) {
	// Update our own counter with a custom tag
	activity.GetMetricsScope(ctx).Tagged(map[string]string{"mytag": "mytagvalue"}).Counter("my_activity_counter").Inc(2)
	return strings.Join(strs, ", "), nil
}

func CheckResult(ctx context.Context, r *harness.Runner, run client.WorkflowRun) error {
	// Check run result
	if err := r.CheckResultDefault(ctx, run); err != nil {
		return err
	}

	// Check counters with tags
	snapshot := r.Feature.ClientOptions.MetricsScope.(tally.TestScope).Snapshot()

	// 2 activity completions
	r.Require.Equal(int64(2), counterValue(snapshot, "temporal_request", map[string]string{
		"operation":     "RespondActivityTaskCompleted",
		"workflow_type": "Workflow",
	}))

	// Custom workflow counter we incremented by 2
	r.Require.Equal(int64(2), counterValue(snapshot, "my_workflow_counter", map[string]string{
		"mytag": "mytagvalue",
	}))

	// Custom activity counter we incremented by 2 twice
	r.Require.Equal(int64(4), counterValue(snapshot, "my_activity_counter", map[string]string{
		"mytag": "mytagvalue",
	}))
	return nil
}

func counterValue(snapshot tally.Snapshot, name string, expectedTags map[string]string) int64 {
	var total int64
CounterLoop:
	for _, counter := range snapshot.Counters() {
		// Check name
		if counter.Name() != name {
			continue
		}
		// Check tags
		for expectedK, expectedV := range expectedTags {
			if expectedV != counter.Tags()[expectedK] {
				continue CounterLoop
			}
		}
		// Add to total
		total += counter.Value()
	}
	return total
}
