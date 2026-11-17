package sdkbuild

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func TestBuildPythonProgramStopsOnSDKCommandError(t *testing.T) {
	base := t.TempDir()
	if err := os.WriteFile(filepath.Join(base, "pyproject.toml"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	sdk := t.TempDir()
	if err := os.Mkdir(filepath.Join(sdk, "dist"), 0755); err != nil {
		t.Fatal(err)
	}
	wheel := filepath.Join(sdk, "dist", "branch-sdk.whl")
	if err := os.WriteFile(wheel, nil, 0644); err != nil {
		t.Fatal(err)
	}

	installErr := errors.New("wheel installation failed")
	var commands [][]string
	program, err := BuildPythonProgram(context.Background(), BuildPythonProgramOptions{
		BaseDir: base,
		Version: sdk,
		ApplyToCommand: func(_ context.Context, cmd *exec.Cmd) error {
			commands = append(commands, cmd.Args)
			return installErr
		},
	})
	if program != nil || !errors.Is(err, installErr) {
		t.Fatalf("expected installation failure, got program=%v, error=%v", program, err)
	}
	// Installing features after this failure could pull a published SDK instead.
	want := [][]string{{"uv", "add", wheel}}
	if !reflect.DeepEqual(commands, want) {
		t.Fatalf("commands = %v, want %v", commands, want)
	}
}
