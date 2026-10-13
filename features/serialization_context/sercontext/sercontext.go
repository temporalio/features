// Package sercontext provides a payload codec and failure converter that are
// aware of the serialization context the SDK hands them, plus history helpers
// used by the serialization_context features to assert on the recorded context.
package sercontext

import (
	"context"
	"fmt"

	commonpb "go.temporal.io/api/common/v1"
	enumspb "go.temporal.io/api/enums/v1"
	failurepb "go.temporal.io/api/failure/v1"
	historypb "go.temporal.io/api/history/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/temporal"
	"google.golang.org/protobuf/proto"
)

// MetadataKey is the payload metadata entry Codec stamps with the signature of
// the serialization context it was created with.
const MetadataKey = "ctx-signature"

// NoContext is the signature used when a converter is used without any
// serialization context.
const NoContext = "none"

// DefaultFailureSource is what the default failure converter puts in
// Failure.Source. FailureConverter only overwrites its own fresh failures.
const DefaultFailureSource = "GoSDK"

func WorkflowSignature(namespace, workflowID string) string {
	return fmt.Sprintf("wf|%s|%s", namespace, workflowID)
}

func ActivitySignature(namespace, workflowID, workflowType, activityType, taskQueue string, isLocal bool) string {
	return fmt.Sprintf("act|%s|%s|%s|%s|%s|%t",
		namespace, workflowID, workflowType, activityType, taskQueue, isLocal)
}

// Signature renders a serialization context as a comparable string.
func Signature(ctx converter.SerializationContext) string {
	switch sc := ctx.(type) {
	case converter.WorkflowSerializationContext:
		return WorkflowSignature(sc.Namespace, sc.WorkflowID)
	case converter.ActivitySerializationContext:
		return ActivitySignature(sc.Namespace, sc.WorkflowID, sc.WorkflowType, sc.ActivityType, sc.TaskQueue, sc.IsLocal)
	}
	return NoContext
}

// ClientOptions returns client options wired with the context aware converters.
func ClientOptions() client.Options {
	return client.Options{
		DataConverter:    converter.NewCodecDataConverter(converter.GetDefaultDataConverter(), NewCodec()),
		FailureConverter: NewFailureConverter(),
	}
}

// Codec stamps every payload it encodes with the signature of its serialization
// context and rejects any payload that was encoded under a different one.
type Codec struct {
	signature string
}

func NewCodec() *Codec { return &Codec{signature: NoContext} }

func (c *Codec) WithSerializationContext(ctx converter.SerializationContext) converter.PayloadCodec {
	return &Codec{signature: Signature(ctx)}
}

func (c *Codec) Encode(payloads []*commonpb.Payload) ([]*commonpb.Payload, error) {
	result := make([]*commonpb.Payload, len(payloads))
	for i, p := range payloads {
		clone := proto.Clone(p).(*commonpb.Payload)
		if clone.Metadata == nil {
			clone.Metadata = map[string][]byte{}
		}
		clone.Metadata[MetadataKey] = []byte(c.signature)
		result[i] = clone
	}
	return result, nil
}

func (c *Codec) Decode(payloads []*commonpb.Payload) ([]*commonpb.Payload, error) {
	result := make([]*commonpb.Payload, len(payloads))
	for i, p := range payloads {
		if encoded := SignatureOf(p); encoded != c.signature {
			return nil, fmt.Errorf(
				"serialization context mismatch: payload encoded as %q, decoded as %q", encoded, c.signature)
		}
		clone := proto.Clone(p).(*commonpb.Payload)
		delete(clone.Metadata, MetadataKey)
		result[i] = clone
	}
	return result, nil
}

// FailureConverter records the signature of its serialization context in
// Failure.Source of the failures it creates.
type FailureConverter struct {
	parent    converter.FailureConverter
	signature string
}

func NewFailureConverter() *FailureConverter {
	return &FailureConverter{parent: temporal.GetDefaultFailureConverter(), signature: NoContext}
}

func (f *FailureConverter) WithSerializationContext(ctx converter.SerializationContext) converter.FailureConverter {
	return &FailureConverter{parent: f.parent, signature: Signature(ctx)}
}

func (f *FailureConverter) ErrorToFailure(err error) *failurepb.Failure {
	failure := f.parent.ErrorToFailure(err)
	// A failure that already travelled the wire is returned as-is by the default
	// converter, and its source must not be overwritten.
	if failure != nil && failure.Source == DefaultFailureSource {
		failure.Source = f.signature
	}
	return failure
}

func (f *FailureConverter) FailureToError(failure *failurepb.Failure) error {
	return f.parent.FailureToError(failure)
}

// SignatureOf returns the context signature a payload was encoded with.
func SignatureOf(payload *commonpb.Payload) string {
	return string(payload.GetMetadata()[MetadataKey])
}

// FirstSignature returns the context signature of the first payload.
func FirstSignature(payloads *commonpb.Payloads) string {
	if len(payloads.GetPayloads()) == 0 {
		return ""
	}
	return SignatureOf(payloads.GetPayloads()[0])
}

// Events collects the full history of a workflow execution.
func Events(ctx context.Context, c client.Client, workflowID, runID string) ([]*historypb.HistoryEvent, error) {
	iter := c.GetWorkflowHistory(ctx, workflowID, runID, false, enumspb.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT)
	var events []*historypb.HistoryEvent
	for iter.HasNext() {
		event, err := iter.Next()
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}

// FindEvent returns the first event matching cond, or an error if there is none.
func FindEvent(
	events []*historypb.HistoryEvent,
	name string,
	cond func(*historypb.HistoryEvent) bool,
) (*historypb.HistoryEvent, error) {
	for _, event := range events {
		if cond(event) {
			return event, nil
		}
	}
	return nil, fmt.Errorf("no %v event in history", name)
}
