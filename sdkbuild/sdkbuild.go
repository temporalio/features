// Package sdkbuild provides helpers to build and run projects with SDKs across
// languages and versions.
package sdkbuild

import (
	"context"
	"io"
	"os"
	"os/exec"
)

// Program is a built SDK program that can be run.
type Program interface {
	// Dir is the directory the program is in. If created on the fly, usually this
	// temporary directory is deleted after use.
	Dir() string

	// NewCommand creates a new command for the program with given args and with
	// stdio set as the current stdio.
	NewCommand(ctx context.Context, args ...string) (*exec.Cmd, error)
}

// setupCommandIO sets up the command's I/O. If stdout or stderr are nil,
// defaults to os.Stdout and os.Stderr respectively. os.Stdin is always set to os.Stdin.
func setupCommandIO(cmd *exec.Cmd, stdout, stderr io.Writer) {
	cmd.Stdin = os.Stdin
	if stdout != nil {
		cmd.Stdout = stdout
	} else {
		cmd.Stdout = os.Stdout
	}
	if stderr != nil {
		cmd.Stderr = stderr
	} else {
		cmd.Stderr = os.Stderr
	}
}
