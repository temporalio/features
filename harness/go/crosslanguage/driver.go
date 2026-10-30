// Package crosslanguage lets feature drivers invoke prepared language harnesses.
package crosslanguage

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
)

const ManifestEnv = "FEATURE_CROSS_LANGUAGE"
const WorkerReady = "FEATURE_WORKER_READY"

type Program struct {
	Dir  string
	Args []string
}

type Manifest struct {
	Feature   string
	TaskQueue string
	Programs  map[string]Program
}

func FromEnv() (*Manifest, error) {
	var manifest Manifest
	if err := json.Unmarshal([]byte(os.Getenv(ManifestEnv)), &manifest); err != nil {
		return nil, fmt.Errorf("read runner manifest: %w", err)
	}
	return &manifest, nil
}

func (m *Manifest) command(ctx context.Context, lang, role, queue string) (*exec.Cmd, error) {
	program, ok := m.Programs[lang]
	if !ok || len(program.Args) == 0 {
		return nil, fmt.Errorf("feature has no prepared %s participant", lang)
	}
	args := append(append([]string{}, program.Args[1:]...), "--role", role, m.Feature+":"+queue)
	cmd := exec.CommandContext(ctx, program.Args[0], args...)
	cmd.Dir = program.Dir
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd, nil
}

func (m *Manifest) RunClient(ctx context.Context, lang, queue string) error {
	cmd, err := m.command(ctx, lang, "client", queue)
	if err != nil {
		return err
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s client: %w", lang, err)
	}
	return nil
}

// StartWorker waits for harness readiness. The returned function stops and reaps
// the process; drivers should defer it before executing any client scenarios.
func (m *Manifest) StartWorker(ctx context.Context, lang, queue string) (func(), error) {
	ctx, cancel := context.WithCancel(ctx)
	cmd, err := m.command(ctx, lang, "worker", queue)
	if err != nil {
		cancel()
		return nil, err
	}
	cmd.Stdout = nil
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, err
	}
	ready, done := make(chan struct{}), make(chan struct{})
	var runErr error
	go func() {
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 4096), 1024*1024)
		announced := false
		for scanner.Scan() {
			line := scanner.Text()
			if line == WorkerReady && !announced {
				close(ready)
				announced = true
			} else {
				fmt.Println(line)
			}
		}
		if scanner.Err() != nil {
			// A stopped reader can leave the worker blocked writing to its full pipe.
			cancel()
		}
		runErr = cmd.Wait()
		if scanner.Err() != nil {
			runErr = fmt.Errorf("read worker output: %w", scanner.Err())
		}
		close(done)
	}()
	stop := func() {
		cancel()
		<-done
	}
	select {
	case <-ready:
		return stop, nil
	case <-done:
		stop()
		return nil, fmt.Errorf("%s worker exited before readiness: %v", lang, runErr)
	case <-ctx.Done():
		stop()
		return nil, ctx.Err()
	}
}
