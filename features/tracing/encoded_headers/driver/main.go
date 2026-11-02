package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"slices"
	"strings"
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
	participants, err := crosslanguage.FromEnv()
	if err != nil {
		return err
	}
	pairs, err := selectPairs(participants, os.Getenv("ENCODED_HEADERS_PAIRS"))
	if err != nil {
		return err
	}
	var failures []error
	for _, pair := range pairs {
		if ctx.Err() != nil {
			return errors.Join(append(failures, ctx.Err())...)
		}
		fmt.Printf("Testing encoded tracing headers: %s client → %s worker\n", pair[0], pair[1])
		err := runPair(ctx, participants, pair)
		if err != nil {
			err = fmt.Errorf("%s → %s: %w", pair[0], pair[1], err)
			fmt.Fprintln(os.Stderr, err)
			failures = append(failures, err)
		} else {
			fmt.Printf("PASS %s → %s\n", pair[0], pair[1])
		}
	}
	return errors.Join(failures...)
}

var clients = []string{"cs", "go", "java", "py", "rb", "rs", "ts"}
var workers = []string{"cs", "go", "java", "py", "rb", "rs", "ts"}

func selectPairs(manifest *crosslanguage.Manifest, filter string) ([][2]string, error) {
	var pairs [][2]string
	if filter != "" {
		for _, selection := range strings.Split(filter, ",") {
			client, worker, ok := strings.Cut(selection, ":")
			if !ok || !slices.Contains(clients, client) || !slices.Contains(workers, worker) {
				return nil, fmt.Errorf("invalid ENCODED_HEADERS_PAIRS pair %q: clients=%v workers=%v; PHP has no tracing integration", selection, clients, workers)
			}
			if _, ok := manifest.Programs[client]; !ok {
				return nil, fmt.Errorf("client %s is not prepared; include --sdk-language %s", client, client)
			}
			if _, ok := manifest.Programs[worker]; !ok {
				return nil, fmt.Errorf("worker %s is not prepared; include --sdk-language %s", worker, worker)
			}
			pair := [2]string{client, worker}
			if !slices.Contains(pairs, pair) {
				pairs = append(pairs, pair)
			}
		}
	} else {
		for _, client := range clients {
			if _, ok := manifest.Programs[client]; !ok {
				continue
			}
			for _, worker := range workers {
				if _, ok := manifest.Programs[worker]; ok {
					pairs = append(pairs, [2]string{client, worker})
				}
			}
		}
	}
	if len(pairs) == 0 {
		return nil, fmt.Errorf("no supported client/worker pairs among prepared participants")
	}
	return pairs, nil
}

func runPair(ctx context.Context, participants *crosslanguage.Manifest, pair [2]string) error {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	queue := participants.TaskQueue + "-" + pair[0] + "-" + pair[1]
	stop, err := participants.StartWorker(ctx, pair[1], queue)
	if err != nil {
		return err
	}
	defer stop()
	return participants.RunClient(ctx, pair[0], queue)
}
