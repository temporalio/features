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
	var myClient client.NamespaceClient
	defer func() {
		if myClient != nil {
			myClient.Close()
		}
	}()
	lastErr := RetryFor(50, 100*time.Millisecond, func() (bool, error) {
		if myClient == nil {
			var clientErr error
			myClient, clientErr = client.NewNamespaceClient(
				client.Options{HostPort: hostPortStr, Namespace: namespace})
			if clientErr != nil {
				return false, clientErr
			}
		}
		_, clientErr := myClient.Describe(ctx, namespace)
		return clientErr == nil, clientErr
	})
	if lastErr != nil {
		return fmt.Errorf("failed connecting after 5s, last error: %w", lastErr)
	}
	return nil
}

// RetryFor retries some function until it passes or we run out of attempts. Wait interval between
// attempts.
func RetryFor(maxAttempts int, interval time.Duration, cond func() (bool, error)) error {
	var lastErr error
	for i := 0; i < maxAttempts; i++ {
		if ok, curE := cond(); ok {
			return nil
		} else {
			lastErr = curE
		}
		time.Sleep(interval)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("failed after %d attempts", maxAttempts)
	}
	return lastErr
}
