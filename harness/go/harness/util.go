package harness

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"

	"go.temporal.io/sdk/temporal"
)

// RetryDisabled is a retry policy with 1 max attempt.
var RetryDisabled = &temporal.RetryPolicy{MaximumAttempts: 1}

// AppErrorf creates a formatted retryable application error.
func AppErrorf(msg string, args ...interface{}) error {
	return temporal.NewApplicationError(fmt.Sprintf(msg, args...), "SdkFeaturesError")
}

type JSONPayload struct {
	Data     string            `json:"data"`
	Metadata map[string]string `json:"metadata"`
}

func UnmarshalPayload(data []byte) (*JSONPayload, error) {
	var payload JSONPayload
	return &payload, json.Unmarshal(data, &payload)
}

// Thanks stackoverflow, I hope this is acceptable
func Filename() (string, error) {
	_, filename, _, ok := runtime.Caller(1)
	if !ok {
		return "", errors.New("unable to get the current filename")
	}
	return filename, nil
}

// Thanks stackoverflow, I hope this is acceptable
func Dirname() (string, error) {
	filename, err := Filename()
	if err != nil {
		return "", err
	}
	return filepath.Dir(filename), nil
}
