//go:build pre1.12.0

package activity_start_race

import (
	"github.com/temporalio/features/harness/go/harness"
)

var Feature = harness.Feature{
	SkipReason: "Requires at least v1.12.0 since it uses gRPC dial options",
}
