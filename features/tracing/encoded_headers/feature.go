package encoded_headers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/temporalio/features/harness/go/harness"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	commonpb "go.temporal.io/api/common/v1"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/contrib/opentelemetry"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/interceptor"
	"go.temporal.io/sdk/workflow"
	"google.golang.org/protobuf/proto"
)

const encoding = "binary/encrypted"

type wrappingCodec struct{}

func (wrappingCodec) Encode(payloads []*commonpb.Payload) ([]*commonpb.Payload, error) {
	result := make([]*commonpb.Payload, len(payloads))
	for i, payload := range payloads {
		if string(payload.Metadata["encoding"]) == encoding {
			return nil, fmt.Errorf("payload encoded twice")
		}
		data, err := proto.Marshal(payload)
		if err != nil {
			return nil, err
		}
		for j := range data {
			data[j] ^= 0x5a
		}
		result[i] = &commonpb.Payload{Metadata: map[string][]byte{"encoding": []byte(encoding)}, Data: data}
	}
	return result, nil
}

func (wrappingCodec) Decode(payloads []*commonpb.Payload) ([]*commonpb.Payload, error) {
	result := make([]*commonpb.Payload, len(payloads))
	for i, payload := range payloads {
		if string(payload.Metadata["encoding"]) != encoding {
			return nil, fmt.Errorf("expected encoded payload, got %q", payload.Metadata["encoding"])
		}
		data := append([]byte(nil), payload.Data...)
		for j := range data {
			data[j] ^= 0x5a
		}
		result[i] = new(commonpb.Payload)
		if err := proto.Unmarshal(data, result[i]); err != nil {
			return nil, err
		}
	}
	return result, nil
}

var provider = sdktrace.NewTracerProvider()
var tracer = provider.Tracer("encoded-headers-feature")

var Feature = harness.Feature{
	Workflows: harness.WorkflowWithOptions{Workflow: Workflow, Options: workflow.RegisterOptions{Name: "workflow"}},
	ClientOptions: client.Options{
		DataConverter: converter.NewCodecDataConverter(converter.GetDefaultDataConverter(), wrappingCodec{}),
		Interceptors:  []interceptor.ClientInterceptor{newTracingInterceptor()},
	},
	Execute: execute,
}

func newTracingInterceptor() interceptor.Interceptor {
	tracing, err := opentelemetry.NewTracingInterceptor(opentelemetry.TracerOptions{
		Tracer: tracer, EncodeHeaders: true, TextMapPropagator: propagation.TraceContext{},
	})
	if err != nil {
		panic(err)
	}
	return tracing
}

func Workflow(ctx workflow.Context, message string) (map[string]string, error) {
	span, ok := opentelemetry.SpanFromWorkflowContext(ctx)
	if !ok || !span.SpanContext().IsValid() {
		return nil, fmt.Errorf("no active workflow tracing span")
	}
	return map[string]string{"message": message, "traceId": span.SpanContext().TraceID().String()}, nil
}

func execute(ctx context.Context, r *harness.Runner) (client.WorkflowRun, error) {
	ctx, parent := tracer.Start(ctx, "go-caller")
	defer parent.End()
	run, err := r.Client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		TaskQueue: r.TaskQueue, WorkflowExecutionTimeout: 15 * time.Second,
	}, "workflow", "hello")
	if err != nil {
		return nil, err
	}
	var result map[string]string
	resultErr := run.Get(ctx, &result)
	history := r.Client.GetWorkflowHistory(ctx, run.GetID(), run.GetRunID(), false, enumspb.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT)
	found := false
	for history.HasNext() {
		event, err := history.Next()
		if err != nil {
			return nil, err
		}
		if failed := event.GetWorkflowTaskFailedEventAttributes(); failed != nil {
			return nil, fmt.Errorf("workflow task failed: %v", failed.Failure)
		}
		if started := event.GetWorkflowExecutionStartedEventAttributes(); started != nil {
			header := started.Header.GetFields()["_tracer-data"]
			if header == nil {
				return nil, fmt.Errorf("missing tracing header")
			}
			decoded, err := (wrappingCodec{}).Decode([]*commonpb.Payload{header})
			if err != nil {
				return nil, err
			}
			var carrier map[string]string
			if err := json.Unmarshal(decoded[0].Data, &carrier); err != nil {
				return nil, err
			}
			parts := strings.Split(carrier["traceparent"], "-")
			if len(parts) != 4 || parts[1] != parent.SpanContext().TraceID().String() {
				return nil, fmt.Errorf("history traceparent mismatch: %v", carrier)
			}
			found = true
		}
	}
	if resultErr != nil {
		return nil, resultErr
	}
	if !found || result["message"] != "hello" || result["traceId"] != parent.SpanContext().TraceID().String() {
		return nil, fmt.Errorf("trace identity/input round-trip failed: history=%t result=%v", found, result)
	}
	return nil, nil
}
