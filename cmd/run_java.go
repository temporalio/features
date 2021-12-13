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
	"go.temporal.io/server/common/log/tag"
)

// RunJavaExternal runs the Java run in an external process. This expects the
// server to already be started.
func (r *Runner) RunJavaExternal(ctx context.Context, run *cmd.Run) error {
	// To do this, we're gonna create a temporary project, --include-build this
	// one, then "gradle run" it

	// Create base dir
	_, currFile, _, _ := runtime.Caller(0)
	rootDir := filepath.Dir(filepath.Dir(currFile))
	tempDir, err := os.MkdirTemp(rootDir, "sdk-features-java-test-")
	if err != nil {
		return fmt.Errorf("failed creating temp dir: %w", err)
	}
	r.log.Info("Building temporary Java project", tag.NewStringTag("Path", tempDir))
	// Remove when done if configured to do so
	if !r.config.RetainTempDir {
		defer os.RemoveAll(tempDir)
	}

	// Create build.gradle and settings.gradle
	temporalSDKDependency := ""
	if r.config.Version != "" {
		temporalSDKDependency = fmt.Sprintf("implementation 'io.temporal:temporal-sdk:%v'", r.config.Version)
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
	if err := os.WriteFile(filepath.Join(tempDir, "build.gradle"), []byte(buildGradle), 0644); err != nil {
		return fmt.Errorf("failed writing build.gradle: %w", err)
	}
	settingsGradle := fmt.Sprintf("rootProject.name = '%v'", filepath.Base(tempDir))
	if err := os.WriteFile(filepath.Join(tempDir, "settings.gradle"), []byte(settingsGradle), 0644); err != nil {
		return fmt.Errorf("failed writing settings.gradle: %w", err)
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
	exeArgs := []string{"--include-build", "../", "run", "--args", runArgsStr}

	// Prepare exe whether windows or not
	var exe string
	if runtime.GOOS == "windows" {
		exe = "cmd.exe"
		exeArgs = append([]string{"/C", "..\\gradlew"}, exeArgs...)
	} else {
		exe = "/bin/sh"
		exeArgs = append([]string{"../gradlew"}, exeArgs...)
	}

	// Run
	r.log.Debug("Running Gradle separately", tag.NewStringTag("Exe", exe), tag.NewStringsTag("Args", exeArgs))
	cmd := exec.CommandContext(ctx, exe, exeArgs...)
	cmd.Dir = tempDir
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed running: %w", err)
	}
	return nil
}
