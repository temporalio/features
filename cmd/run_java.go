package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"go.temporal.io/sdk-features/harness/go/cmd"
	"go.temporal.io/sdk/log"
)

// PrepareJavaExternal prepares a Java run without running it. The preparer
// config directory is expected to be an absolute subdirectory just beneath the
// root directory.
func (p *Preparer) PrepareJavaExternal(ctx context.Context, build bool) error {
	// To do this, we're gonna create a temporary project, --include-build this
	// one, then gradle it
	p.log.Info("Building Java project", "Path", p.config.Dir)

	// Create build.gradle and settings.gradle
	temporalSDKDependency := ""
	if p.config.Version != "" {
		temporalSDKDependency = fmt.Sprintf("implementation 'io.temporal:temporal-sdk:%v'", p.config.Version)
	}
	buildGradle := `
plugins {
    id 'application'
}

repositories {
    mavenCentral()
}

dependencies {
    implementation 'io.temporal:sdk-features:0.1.0'
    ` + temporalSDKDependency + `
}

application {
    mainClass = 'io.temporal.sdkfeatures.Main'
}`
	if err := os.WriteFile(filepath.Join(p.config.Dir, "build.gradle"), []byte(buildGradle), 0644); err != nil {
		return fmt.Errorf("failed writing build.gradle: %w", err)
	}
	settingsGradle := fmt.Sprintf("rootProject.name = '%v'", filepath.Base(p.config.Dir))
	if err := os.WriteFile(filepath.Join(p.config.Dir, "settings.gradle"), []byte(settingsGradle), 0644); err != nil {
		return fmt.Errorf("failed writing settings.gradle: %w", err)
	}

	// Build if wanted
	if build {
		// This is really only to prime the system-level caches. The build won't be
		// used by run.
		return runGradle(ctx, p.log, p.config.Dir, []string{"--no-daemon", "--include-build", "../", "build"})
	}
	return nil
}

// RunJavaExternal runs the Java run in an external process. This expects the
// server to already be started.
func (r *Runner) RunJavaExternal(ctx context.Context, run *cmd.Run) error {
	// Create temp dir if needed and prepare the project
	if r.config.Dir == "" {
		var err error
		if r.config.Dir, err = os.MkdirTemp(r.rootDir, "sdk-features-java-test-"); err != nil {
			return fmt.Errorf("failed creating temp dir: %w", err)
		}
		r.createdTempDir = &r.config.Dir

		// Prepare the project but don't build since it'll happen when we run later
		if err := NewPreparer(r.config.PrepareConfig).PrepareJavaExternal(ctx, false); err != nil {
			return err
		}
	}

	// Prepare args for gradle run. Gradle args will be single quoted or double
	// quoted since they'll be in an argument themselves. Therefore for now to
	// keep it simple, we won't allow either in any of the arguments.
	runArgs := append([]string{"--server", r.config.Server, "--namespace", r.config.Namespace}, run.ToArgs()...)
	var runArgsStr string
	for _, runArg := range runArgs {
		if strings.ContainsAny(runArg, `"'`) {
			return fmt.Errorf("java argument cannot contain single or double quote")
		}
		if runArgsStr != "" {
			runArgsStr += " "
		}
		runArgsStr += "'" + runArg + "'"
	}

	// Run. We aren't using some previous prepared build or anything. Rather
	// preparing is just for priming the Gradle cache.
	return runGradle(ctx, r.log, r.config.Dir, []string{"--include-build", "../", "run", "--args", runArgsStr})
}

func runGradle(ctx context.Context, log log.Logger, dir string, args []string) error {
	// Prepare exe whether windows or not
	var exe string
	if runtime.GOOS == "windows" {
		exe = "cmd.exe"
		args = append([]string{"/C", "..\\gradlew"}, args...)
	} else {
		exe = "/bin/sh"
		args = append([]string{"../gradlew"}, args...)
	}

	// Run
	log.Debug("Running Gradle separately", "Exe", exe, "Args", args)
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Dir = dir
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed running: %w", err)
	}
	return nil
}
