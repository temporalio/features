//go:build pre1.11.0

package tally_metrics

import (
	"go.temporal.io/sdk-features/harness/go/harness"
)

var Feature = harness.Feature{
	SkipReason: "Requires at least v1.11.0",
}
