package main

import (
	"strings"
	"testing"

	"github.com/temporalio/features/harness/go/crosslanguage"
)

func TestPairSelection(t *testing.T) {
	manifest := &crosslanguage.Manifest{Programs: map[string]crosslanguage.Program{
		"cs": {}, "ts": {}, "go": {},
	}}
	pairs, err := selectPairs(manifest, "")
	if err != nil || len(pairs) != 9 {
		t.Fatalf("expected cs/go/ts clients × cs/go/ts workers: %v %v", pairs, err)
	}
	pairs, err = selectPairs(manifest, "cs:ts,cs:ts")
	if err != nil || len(pairs) != 1 || pairs[0] != [2]string{"cs", "ts"} {
		t.Fatalf("unexpected focused selection: %v %v", pairs, err)
	}
	for filter, message := range map[string]string{
		"go:java": "not prepared", "rs:ts": "not prepared",
		"cs:php": "no tracing integration",
	} {
		if _, err := selectPairs(manifest, filter); err == nil || !strings.Contains(err.Error(), message) {
			t.Fatalf("%s: expected %q, got %v", filter, message, err)
		}
	}
}
