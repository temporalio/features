package crosslanguage

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"testing"
	"time"
)

func helperManifest(t *testing.T, behavior string) Manifest {
	t.Helper()
	t.Setenv("CROSS_LANGUAGE_TEST_PROCESS", behavior)
	return Manifest{
		Feature: "tracing/headers",
		Programs: map[string]Program{
			"test": {Args: []string{os.Args[0], "-test.run=^TestHarnessProcess$", "--"}},
		},
	}
}

func TestWorkerReadinessAndCleanup(t *testing.T) {
	manifest := helperManifest(t, "ready")
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	stop, err := manifest.StartWorker(ctx, "test", "queue")
	if err != nil {
		t.Fatal(err)
	}
	stop()
}

func TestWorkerExitsBeforeReadiness(t *testing.T) {
	manifest := helperManifest(t, "exit")
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	if _, err := manifest.StartWorker(ctx, "test", "queue"); err == nil ||
		!strings.Contains(err.Error(), "before readiness") {
		t.Fatalf("expected readiness failure, got %v", err)
	}
}

func TestWorkerCanceledBeforeReadiness(t *testing.T) {
	manifest := helperManifest(t, "wait")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := manifest.StartWorker(ctx, "test", "queue"); err == nil {
		t.Fatal("canceled worker unexpectedly started")
	}
}

func TestClientRoleAndFailure(t *testing.T) {
	for _, behavior := range []string{"client", "exit"} {
		manifest := helperManifest(t, behavior)
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		err := manifest.RunClient(ctx, "test", "queue")
		cancel()
		if (err != nil) != (behavior == "exit") {
			t.Fatalf("behavior=%s: unexpected result %v", behavior, err)
		}
	}
}

func TestHarnessProcess(t *testing.T) {
	behavior := os.Getenv("CROSS_LANGUAGE_TEST_PROCESS")
	if behavior == "" {
		return
	}
	switch behavior {
	case "client":
		args := strings.Join(os.Args, " ")
		if !strings.Contains(args, "--role client tracing/headers:queue") {
			os.Exit(2)
		}
	case "ready", "wait":
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()
		if behavior == "ready" {
			fmt.Println(WorkerReady)
			fmt.Println(WorkerReady)
		}
		<-ctx.Done()
	case "exit":
		os.Exit(3)
	}
}
