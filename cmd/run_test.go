package cmd

import (
	"context"
	"strings"
	"testing"

	hcmd "github.com/temporalio/features/harness/go/cmd"
)

func TestDynamicConfigArgsAppliesOverrides(t *testing.T) {
	r := NewRunner(RunConfig{})
	args, err := r.dynamicConfigArgs(map[string]any{
		"frontend.enableCancelWorkerPollsOnShutdown": false,
	})
	if err != nil {
		t.Fatal(err)
	}

	joined := strings.Join(args, "\n")
	if !strings.Contains(joined, "frontend.enableCancelWorkerPollsOnShutdown=false") {
		t.Fatalf("dynamic config override missing from args:\n%s", joined)
	}
	if strings.Contains(joined, "frontend.enableCancelWorkerPollsOnShutdown=true") {
		t.Fatalf("dynamic config override should replace previous values:\n%s", joined)
	}
}

func TestMakeRunBatchesExpandsVariants(t *testing.T) {
	r := NewRunner(RunConfig{})
	features := []*RunFeature{
		{
			Dir: "worker_shutdown/poll_complete_on_shutdown",
			Config: hcmd.RunFeatureConfig{
				RunVariants: []hcmd.RunVariantConfig{
					{
						Name: "enabled",
						DynamicConfig: map[string]any{
							"frontend.enableCancelWorkerPollsOnShutdown": true,
						},
					},
					{
						Name: "disabled",
						DynamicConfig: map[string]any{
							"frontend.enableCancelWorkerPollsOnShutdown": false,
						},
					},
				},
			},
		},
	}

	batches, err := r.makeRunBatches(features)
	if err != nil {
		t.Fatal(err)
	}
	if len(batches) != 2 {
		t.Fatalf("expected 2 batches, got %d", len(batches))
	}
	for i, want := range []string{"enabled", "disabled"} {
		if batches[i].VariantName != want {
			t.Fatalf("batch %d variant name = %q, want %q", i, batches[i].VariantName, want)
		}
		if got := batches[i].Run.Features[0].SummaryName(); got != "worker_shutdown/poll_complete_on_shutdown#"+want {
			t.Fatalf("batch %d summary name = %q", i, got)
		}
	}
}

func TestRewriteVariantSummary(t *testing.T) {
	features := []hcmd.RunFeature{
		{Dir: "worker_shutdown/poll_complete_on_shutdown", VariantName: "enabled"},
	}
	summary := rewriteVariantSummary(Summary{
		{Name: "worker_shutdown/poll_complete_on_shutdown", Outcome: FeaturePassed},
	}, features)
	if got := summary[0].Name; got != "worker_shutdown/poll_complete_on_shutdown#enabled" {
		t.Fatalf("summary name = %q", got)
	}
}

func TestRunBatchRejectsVariantWithExternalServer(t *testing.T) {
	r := NewRunner(RunConfig{Server: "localhost:7233", Namespace: "default"})
	err := r.runBatch(context.Background(), runBatch{
		Run:         &hcmd.Run{},
		VariantName: "enabled",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "requires the embedded dev server") {
		t.Fatalf("unexpected error: %v", err)
	}
}
