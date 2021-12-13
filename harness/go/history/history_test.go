package history

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.temporal.io/api/common/v1"
	"go.temporal.io/api/history/v1"
	"go.temporal.io/api/taskqueue/v1"
)

func TestScrub(t *testing.T) {
	// Just check a couple of values
	now := time.Now()
	actual := Histories{
		{
			Events: []*history.HistoryEvent{
				{
					EventId:   1,
					EventTime: &now,
					TaskId:    2,
					Attributes: &history.HistoryEvent_WorkflowExecutionStartedEventAttributes{
						WorkflowExecutionStartedEventAttributes: &history.WorkflowExecutionStartedEventAttributes{
							WorkflowType: &common.WorkflowType{Name: "myworkflow"},
							TaskQueue:    &taskqueue.TaskQueue{Name: "mytaskqueue"},
							Identity:     "myidentity",
							Attempt:      1,
						},
					},
				},
			},
		},
	}
	actual.ScrubRunSpecificFields()
	expected := Histories{
		{
			Events: []*history.HistoryEvent{
				{
					EventId: 1,
					Attributes: &history.HistoryEvent_WorkflowExecutionStartedEventAttributes{
						WorkflowExecutionStartedEventAttributes: &history.WorkflowExecutionStartedEventAttributes{
							WorkflowType: &common.WorkflowType{Name: "myworkflow"},
							TaskQueue:    &taskqueue.TaskQueue{},
							Attempt:      1,
						},
					},
				},
			},
		},
	}
	assert.Equal(t, expected, actual)
}
