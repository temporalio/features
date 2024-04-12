package http_proxy_auth

import (
	"github.com/temporalio/features/features/client/http_proxy"
	"github.com/temporalio/features/harness/go/harness"
	"go.temporal.io/sdk/workflow"
)

var Feature = harness.Feature{
	Workflows:       Workflow,
	Execute:         http_proxy.HTTPProxyTest{UseAuth: true}.Execute,
	ExpectRunResult: "done",
}

func Workflow(ctx workflow.Context) (string, error) { return "done", nil }
