package cmd

import (
	"context"
	"os"
	"os/exec"
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

func TestDynamicConfigArgsMissingBaseUsesOverrides(t *testing.T) {
	r := NewRunner(RunConfig{})
	r.rootDir = t.TempDir()
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
						ExpectNamespaceCapabilities: map[string]bool{"workerPollCompleteOnShutdown": true},
					},
					{
						Name: "disabled",
						DynamicConfig: map[string]any{
							"frontend.enableCancelWorkerPollsOnShutdown": false,
						},
						ExpectNamespaceCapabilities: map[string]bool{"workerPollCompleteOnShutdown": false},
					},
				},
			},
		},
	}

	batches := r.makeRunBatches(features)
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
		if got := batches[i].Capabilities["workerPollCompleteOnShutdown"]; got != (want == "enabled") {
			t.Fatalf("batch %d capability expectation = %t", i, got)
		}
		if got := batches[i].CapabilitiesJSON; !strings.Contains(got, `"workerPollCompleteOnShutdown"`) {
			t.Fatalf("batch %d capabilities env missing capability: %q", i, got)
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

func TestFilterFeaturesForExternalServerSkipsImplicitRunVariants(t *testing.T) {
	r := NewRunner(RunConfig{Server: "localhost:7233"})
	features := []*RunFeature{
		{Dir: "activity/basic"},
		{
			Dir: "worker_shutdown/poll_complete_on_shutdown",
			Config: hcmd.RunFeatureConfig{
				RunVariants: []hcmd.RunVariantConfig{{Name: "enabled"}},
			},
		},
	}

	filtered := r.filterFeaturesForExternalServer(features, nil)
	if len(filtered) != 1 || filtered[0].Dir != "activity/basic" {
		t.Fatalf("filtered features = %+v", filtered)
	}
}

func TestFilterFeaturesForExternalServerKeepsExplicitRunVariants(t *testing.T) {
	r := NewRunner(RunConfig{Server: "localhost:7233"})
	features := []*RunFeature{
		{
			Dir: "worker_shutdown/poll_complete_on_shutdown",
			Config: hcmd.RunFeatureConfig{
				RunVariants: []hcmd.RunVariantConfig{{Name: "enabled"}},
			},
		},
	}

	filtered := r.filterFeaturesForExternalServer(features, []string{"worker_shutdown/poll_complete_on_shutdown"})
	if len(filtered) != 1 || filtered[0].Dir != "worker_shutdown/poll_complete_on_shutdown" {
		t.Fatalf("filtered features = %+v", filtered)
	}
}

func TestApplyNamespaceCapabilitiesEnvForExternalRuns(t *testing.T) {
	cmd := exec.Command("feature-test")
	cmd.Env = []string{
		featureNamespaceCapabilitiesEnv + "=old",
		"KEEP=value",
	}
	applyNamespaceCapabilitiesEnv(cmd, `{"workerPollCompleteOnShutdown":true}`)

	joined := strings.Join(cmd.Env, "\n")
	if strings.Contains(joined, featureNamespaceCapabilitiesEnv+"=old") {
		t.Fatalf("old capabilities env was not replaced: %v", cmd.Env)
	}
	for _, want := range []string{
		"KEEP=value",
		featureNamespaceCapabilitiesEnv + `={"workerPollCompleteOnShutdown":true}`,
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("env missing %q: %v", want, cmd.Env)
		}
	}
}

func TestSetNamespaceCapabilitiesEnvForInProcessGoRunRestoresPreviousValues(t *testing.T) {
	t.Setenv(featureNamespaceCapabilitiesEnv, "old")
	restore := setNamespaceCapabilitiesEnv(`{"workerPollCompleteOnShutdown":false}`)
	if got := os.Getenv(featureNamespaceCapabilitiesEnv); got != `{"workerPollCompleteOnShutdown":false}` {
		t.Fatalf("capabilities env = %q", got)
	}

	restore()

	if got := os.Getenv(featureNamespaceCapabilitiesEnv); got != "old" {
		t.Fatalf("capabilities env after restore = %q, want old", got)
	}
}
