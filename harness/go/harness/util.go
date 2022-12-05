package harness

import (
	"context"
	"fmt"
	"time"

	historypb "go.temporal.io/api/history/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
)

// RetryDisabled is a retry policy with 1 max attempt.
var RetryDisabled = &temporal.RetryPolicy{MaximumAttempts: 1}

// AppErrorf creates a formatted retryable application error.
func AppErrorf(msg string, args ...interface{}) error {
	return temporal.NewApplicationError(fmt.Sprintf(msg, args...), "SdkFeaturesError")
}

// Find the first event in the history that meets the condition.
func FindEvent(history client.HistoryEventIterator, cond func(*historypb.HistoryEvent) bool) (*historypb.HistoryEvent, error) {
	for history.HasNext() {
		ev, err := history.Next()
		if err != nil {
			return nil, err
		}
		if cond(ev) {
			return ev, nil
		}
	}
	return nil, nil
}

func WaitNamespaceAvailable(ctx context.Context, hostPortStr string, namespace string) error {
	// Try every 100ms for 5s to connect
	var clientErr error
	var myClient client.NamespaceClient
	defer func() {
		if myClient != nil {
			myClient.Close()
		}
	}()
	for i := 0; i < 50; i++ {
		if myClient == nil {
			myClient, clientErr = client.NewNamespaceClient(
				client.Options{HostPort: hostPortStr, Namespace: namespace})
			if clientErr != nil {
				continue
			}
		}
		_, clientErr = myClient.Describe(ctx, namespace)
		if clientErr == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if clientErr != nil {
		return fmt.Errorf("failed connecting after 5s, last error: %w", clientErr)
	}
	return nil
}
