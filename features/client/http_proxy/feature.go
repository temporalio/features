package http_proxy

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"

	"github.com/google/uuid"
	"github.com/temporalio/features/harness/go/harness"
	"github.com/urfave/cli/v2"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
)

var Feature = harness.Feature{
	Workflows:       Workflow,
	Execute:         Execute,
	ExpectRunResult: "done",
}

func Workflow(ctx workflow.Context) (string, error) { return "done", nil }

func Execute(ctx context.Context, r *harness.Runner) (client.WorkflowRun, error) {
	// Since HTTP proxy in gRPC only works with the environment variable, we have
	// to test in a subprocess to not infect other things in this process. The
	// subprocess will make the client call to run the workflow, this will just
	// return the run.

	// Due to strange nobody-knows-why Go rules, the proxy doesn't work for
	// localhost servers: https://github.com/golang/go/issues/28866
	host, _, err := net.SplitHostPort(r.ServerHostPort)
	r.Require.NoError(err)
	if ip := net.ParseIP(host); host == "localhost" || (ip != nil && ip.IsLoopback()) {
		return nil, r.Skip("Cannot run proxy test on localhost/loopback")
	}

	// Confirm no proxy in environment currently but proxy URL available
	addr, err := proxyAddr(r.ServerHostPort)
	r.Require.NoError(err)
	r.Require.Empty(addr)
	r.Require.NotEmpty(r.HTTPProxyURL)

	args := subprocessArgs{
		server:         r.ServerHostPort,
		namespace:      r.Namespace,
		clientCertPath: r.ClientCertPath,
		clientKeyPath:  r.ClientKeyPath,
		taskQueue:      r.TaskQueue,
		workflowID:     "wf-" + uuid.NewString(),
	}
	cmd, err := harness.CreateSubprocessCommand(ctx, subprocessCommandName, args.args()...)
	if err != nil {
		return nil, fmt.Errorf("failed creating subprocess command")
	}
	// Set env var so the client will proxy
	cmd.Env = append([]string{"HTTPS_PROXY=" + r.HTTPProxyURL}, os.Environ()...)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed running subprocess: %w", err)
	}
	return r.Client.GetWorkflow(ctx, args.workflowID, ""), nil
}

const subprocessCommandName = "http-proxy-subprocess"

func SubprocessExecuteWorkflow(ctx context.Context, args *subprocessArgs) error {
	// Ensure HTTP proxy is on environment
	if addr, err := proxyAddr(args.server); err != nil {
		return fmt.Errorf("failed getting proxy addr: %w", err)
	} else if addr == "" {
		return fmt.Errorf("proxy not enabled")
	}

	// Dial client
	clientOpts := client.Options{HostPort: args.server, Namespace: args.namespace}
	if args.clientCertPath != "" {
		var err error
		clientOpts.ConnectionOptions.TLS, err = harness.LoadTLSConfig(args.clientCertPath, args.clientKeyPath)
		if err != nil {
			return fmt.Errorf("failed loading TLS config: %w", err)
		}
	}
	cl, err := client.Dial(clientOpts)
	if err != nil {
		return fmt.Errorf("failed dialing client: %w", err)
	}
	defer cl.Close()

	// Execute workflow and confirm response
	run, err := cl.ExecuteWorkflow(
		ctx,
		client.StartWorkflowOptions{ID: args.workflowID, TaskQueue: args.taskQueue},
		Workflow,
	)
	if err != nil {
		return fmt.Errorf("failed starting workflow: %w", err)
	}
	var res string
	if err := run.Get(ctx, &res); err != nil {
		return fmt.Errorf("failed running workflow: %w", err)
	} else if res != "done" {
		return fmt.Errorf("workflow had unexpected response: %v", res)
	}
	return nil
}

// Empty with no error if no proxy set
func proxyAddr(destAddr string) (string, error) {
	if req, err := http.NewRequest("GET", "https://"+destAddr+"/", nil); err != nil {
		return "", err
	} else if url, err := http.ProxyFromEnvironment(req); url == nil || err != nil {
		return "", err
	} else {
		return url.String(), nil
	}
}

type subprocessArgs struct {
	server         string
	namespace      string
	clientCertPath string
	clientKeyPath  string
	taskQueue      string
	workflowID     string
}

func (s *subprocessArgs) flags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{Name: "server", Destination: &s.server, Required: true},
		&cli.StringFlag{Name: "namespace", Destination: &s.namespace, Required: true},
		&cli.StringFlag{Name: "client-cert-path", Destination: &s.clientCertPath},
		&cli.StringFlag{Name: "client-key-path", Destination: &s.clientKeyPath},
		&cli.StringFlag{Name: "task-queue", Destination: &s.taskQueue, Required: true},
		&cli.StringFlag{Name: "workflow-id", Destination: &s.workflowID, Required: true},
	}
}

func (s *subprocessArgs) args() []string {
	args := []string{
		"--server", s.server,
		"--namespace", s.namespace,
		"--task-queue", s.taskQueue,
		"--workflow-id", s.workflowID,
	}
	if s.clientCertPath != "" {
		args = append(args, "--client-cert-path", s.clientCertPath, "--client-key-path", s.clientKeyPath)
	}
	return args
}

func init() {
	var args subprocessArgs
	harness.MustRegisterSubprocessCommand(&cli.Command{
		Name:   subprocessCommandName,
		Usage:  "http proxy subprocess",
		Flags:  args.flags(),
		Action: func(ctx *cli.Context) error { return SubprocessExecuteWorkflow(ctx.Context, &args) },
	})
}
