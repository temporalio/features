package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/temporalio/features/harness/go/crosslanguage"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	ctx, timeout := context.WithTimeout(ctx, time.Minute)
	defer timeout()
	participants, err := crosslanguage.FromEnv()
	if err != nil {
		return err
	}
	queue := participants.TaskQueue + "-cs-ts"
	stop, err := participants.StartWorker(ctx, "ts", queue)
	if err != nil {
		return err
	}
	defer stop()
	fmt.Println("Testing encoded tracing headers: .NET client → TypeScript worker")
	return participants.RunClient(ctx, "cs", queue)
}
