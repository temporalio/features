package cmd

import (
	"context"

	"go.temporal.io/sdk-features/harness/go/cmd"
)

// RunPythonExternal runs the Python run in an external process. This expects the
// server to already be started.
func (r *Runner) RunPythonExternal(ctx context.Context, run *cmd.Run) error {
	panic("TODO")
}
