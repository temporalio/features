package history

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/timestamppb"

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
					EventTime: timestamppb.New(now),
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

func TestScrubSignalRequestID(t *testing.T) {
	attrs := &history.WorkflowExecutionSignaledEventAttributes{
		SignalName: "my_signal",
		Identity:   "client",
		RequestId:  "56fd6977-f9f5-4a75-b282-297b19214996",
		Input:      &common.Payloads{Payloads: []*common.Payload{{Data: []byte(`"arg"`)}}},
	}
	hist := Histories{{Events: []*history.HistoryEvent{{
		Attributes: &history.HistoryEvent_WorkflowExecutionSignaledEventAttributes{
			WorkflowExecutionSignaledEventAttributes: attrs,
		},
	}}}}
	hist.ScrubRunSpecificFields()
	assert.Empty(t, attrs.Identity)
	assert.Empty(t, attrs.RequestId)
	assert.Equal(t, "my_signal", attrs.SignalName)
	assert.Equal(t, []byte(`"arg"`), attrs.Input.Payloads[0].Data)
}
