package history

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/gogo/protobuf/proto"
	"go.temporal.io/api/common/v1"
	"go.temporal.io/api/failure/v1"
	"go.temporal.io/api/history/v1"
	"go.temporal.io/api/taskqueue/v1"
)

// Histories is a collection of histories.
type Histories []*history.History

// Sort sorts histories by the workflow type but nothing else.
func (h Histories) Sort() {
	sort.Slice(h, func(i, j int) bool {
		a, aErr := historyFirstEventName(h[i])
		b, bErr := historyFirstEventName(h[j])
		if aErr != nil {
			panic(aErr)
		} else if bErr != nil {
			panic(bErr)
		}
		return a < b
	})
}

func historyFirstEventName(h *history.History) (string, error) {
	if len(h.Events) == 0 {
		return "", fmt.Errorf("no events in history")
	}
	attrs := h.Events[0].GetWorkflowExecutionStartedEventAttributes()
	if attrs == nil {
		return "", fmt.Errorf("first event not a workflow started event")
	}
	return attrs.WorkflowType.GetName(), nil
}

// UnmarshalJSON converts the given JSON to histories.
func (h *Histories) UnmarshalJSON(b []byte) error {
	// Unmarshal into raw JSON array, then into each with proto unmarshaler
	var halfUnmarshaled []json.RawMessage
	if err := json.Unmarshal(b, &halfUnmarshaled); err != nil {
		return err
	}
	hists := make([]*history.History, len(halfUnmarshaled))
	for i, histJSON := range halfUnmarshaled {
		var hist history.History
		if err := jsonpb.Unmarshal(bytes.NewReader(histJSON), &hist); err != nil {
			return err
		}
		hists[i] = &hist
	}
	*h = hists
	return nil
}

var historyMarshaler jsonpb.Marshaler

// MarshalJSON converts the histories to JSON.
func (h Histories) MarshalJSON() ([]byte, error) {
	// Copy and sort each history by its first event's name
	sorted := make(Histories, len(h))
	copy(sorted, h)
	var err error
	sort.Slice(sorted, func(i, j int) bool {
		a, aErr := historyFirstEventName(sorted[i])
		b, bErr := historyFirstEventName(sorted[j])
		if err == nil && aErr != nil {
			err = aErr
		} else if err == nil && bErr != nil {
			err = bErr
		}
		return a < b
	})

	// Marshal each history, then marshal the whole thing
	halfMarshaled := make([]json.RawMessage, len(sorted))
	for i, history := range sorted {
		s, err := historyMarshaler.MarshalToString(history)
		if err != nil {
			return nil, fmt.Errorf("failed marshaling history: %w", err)
		}
		halfMarshaled[i] = json.RawMessage(s)
	}
	return json.Marshal(halfMarshaled)
}

// Clone performs a deep clone of histories.
func (h Histories) Clone() Histories {
	ret := make(Histories, len(h))
	for i, hist := range h {
		ret[i] = proto.Clone(hist).(*history.History)
	}
	return ret
}

// Equals checks history equality.
func (h Histories) Equals(other Histories) bool {
	if len(h) != len(other) {
		return false
	}
	for i, hist := range h {
		if !proto.Equal(hist, other[i]) {
			return false
		}
	}
	return true
}

// ScrubRunSpecificFields removes all fields on the history that are specific to
// the run.
func (h Histories) ScrubRunSpecificFields() {
	scrubRunSpecificFields(reflect.ValueOf(h))
}

func scrubRunSpecificFields(v reflect.Value) {
	if !v.IsValid() || v.IsZero() {
		return
	}
	// First scrub the fields
	scrubRunSpecificScalars(v.Interface())
	// Now walk children and scrub
	switch v.Kind() {
	case reflect.Array, reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			scrubRunSpecificFields(v.Index(i))
		}
	case reflect.Interface, reflect.Ptr:
		scrubRunSpecificFields(v.Elem())
	case reflect.Map:
		iter := v.MapRange()
		for iter.Next() {
			scrubRunSpecificFields(iter.Value())
		}
	case reflect.Struct:
		for i := 0; i < v.Type().NumField(); i++ {
			if v.Type().Field(i).IsExported() {
				scrubRunSpecificFields(v.Field(i))
			}
		}
	}
}

func scrubRunSpecificScalars(v interface{}) {
	// TODO: Add to this as more things are discovered
	switch v := v.(type) {
	case *common.WorkflowExecution:
		v.RunId = ""
	case *failure.Failure:
		v.Source = ""
		v.StackTrace = ""
	case *failure.ActivityFailureInfo:
		v.ActivityId = ""
		v.Identity = ""
	case *failure.ChildWorkflowExecutionFailureInfo:
		// TODO(cretz): Should we instead just replace namespaces with fixed values
		// keeping same namespaces the same fixed value?
		v.Namespace = ""
	case *history.HistoryEvent:
		v.EventTime = nil
		v.TaskId = 0
	case *history.WorkflowExecutionStartedEventAttributes:
		v.OriginalExecutionRunId = ""
		v.Identity = ""
		v.FirstExecutionRunId = ""
	case *history.WorkflowExecutionCompletedEventAttributes:
		// TODO(cretz): Do we want something to show it is set or not though?
		v.NewExecutionRunId = ""
	case *history.WorkflowExecutionFailedEventAttributes:
		v.NewExecutionRunId = ""
	case *history.WorkflowExecutionTimedOutEventAttributes:
	case *history.WorkflowTaskScheduledEventAttributes:
	case *history.WorkflowTaskStartedEventAttributes:
		v.Identity = ""
		v.RequestId = ""
	case *history.WorkflowTaskCompletedEventAttributes:
		v.Identity = ""
		v.BinaryChecksum = ""
	case *history.WorkflowTaskTimedOutEventAttributes:
	case *history.WorkflowTaskFailedEventAttributes:
		v.Identity = ""
		v.BaseRunId = ""
		v.NewRunId = ""
		v.BinaryChecksum = ""
	case *history.ActivityTaskScheduledEventAttributes:
		// These are UUIDs in Java, even though they are deterministic numbers in Go
		v.ActivityId = ""
		v.Namespace = ""
	case *history.ActivityTaskStartedEventAttributes:
		v.Identity = ""
		v.RequestId = ""
	case *history.ActivityTaskCompletedEventAttributes:
		v.Identity = ""
	case *history.ActivityTaskFailedEventAttributes:
		v.Identity = ""
	case *history.ActivityTaskTimedOutEventAttributes:
	case *history.TimerStartedEventAttributes:
	case *history.TimerFiredEventAttributes:
	case *history.ActivityTaskCancelRequestedEventAttributes:
	case *history.ActivityTaskCanceledEventAttributes:
		v.Identity = ""
	case *history.TimerCanceledEventAttributes:
		v.Identity = ""
	case *history.MarkerRecordedEventAttributes:
	case *history.WorkflowExecutionSignaledEventAttributes:
		v.Identity = ""
	case *history.WorkflowExecutionTerminatedEventAttributes:
		v.Identity = ""
	case *history.WorkflowExecutionCancelRequestedEventAttributes:
		v.Identity = ""
	case *history.WorkflowExecutionCanceledEventAttributes:
	case *history.RequestCancelExternalWorkflowExecutionInitiatedEventAttributes:
		v.Namespace = ""
	case *history.RequestCancelExternalWorkflowExecutionFailedEventAttributes:
	case *history.ExternalWorkflowExecutionCancelRequestedEventAttributes:
		v.Namespace = ""
	case *history.WorkflowExecutionContinuedAsNewEventAttributes:
		v.NewExecutionRunId = ""
	case *history.StartChildWorkflowExecutionInitiatedEventAttributes:
		v.Namespace = ""
	case *history.StartChildWorkflowExecutionFailedEventAttributes:
		v.Namespace = ""
	case *history.ChildWorkflowExecutionStartedEventAttributes:
	case *history.ChildWorkflowExecutionCompletedEventAttributes:
		v.Namespace = ""
	case *history.ChildWorkflowExecutionFailedEventAttributes:
		v.Namespace = ""
	case *history.ChildWorkflowExecutionCanceledEventAttributes:
		v.Namespace = ""
	case *history.ChildWorkflowExecutionTimedOutEventAttributes:
		v.Namespace = ""
	case *history.ChildWorkflowExecutionTerminatedEventAttributes:
		v.Namespace = ""
	case *history.SignalExternalWorkflowExecutionInitiatedEventAttributes:
		v.Namespace = ""
	case *history.SignalExternalWorkflowExecutionFailedEventAttributes:
		v.Namespace = ""
	case *history.ExternalWorkflowExecutionSignaledEventAttributes:
		v.Namespace = ""
	case *history.UpsertWorkflowSearchAttributesEventAttributes:
	case *taskqueue.TaskQueue:
		v.Name = ""
	}
}
