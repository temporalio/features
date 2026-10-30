package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	hcmd "github.com/temporalio/features/harness/go/cmd"
	"github.com/temporalio/features/harness/go/crosslanguage"
	"github.com/temporalio/features/sdkbuild"
	"github.com/urfave/cli/v2"
)

type CrossLanguageRunConfig struct {
	RunConfig
	SDKVersions cli.StringSlice
}

func runCrossLanguageCmd() *cli.Command {
	var config CrossLanguageRunConfig
	return &cli.Command{
		Name:  "run-cross-language",
		Usage: "Run crossLanguage features using their custom drivers",
		Flags: append([]cli.Flag{
			&cli.StringSliceFlag{
				Name: "sdk-version", Destination: &config.SDKVersions,
				Usage: "SDK selection, e.g. cs=1.21.1 or ts=/path/to/sdk",
			},
			&cli.BoolFlag{
				Name: "retain-temp-dir", Destination: &config.RetainTempDir,
				Usage: "Do not delete temporary SDK and driver builds",
			},
		}, config.dockerRunFlags()...),
		Action: func(ctx *cli.Context) error {
			versions, err := config.sdkVersions()
			if err != nil {
				return err
			}
			runner := NewRunner(config.RunConfig)
			runner.crossLanguage, runner.crossVersions = true, versions
			features, err := runner.GlobCrossLanguageFeatures(ctx.Args().Slice())
			if err != nil {
				return err
			}
			return runner.runFeatures(ctx.Context, features, ctx.Args().Slice())
		},
	}
}

func (config CrossLanguageRunConfig) sdkVersions() (map[string]string, error) {
	versions := map[string]string{}
	for _, selection := range config.SDKVersions.Value() {
		lang, version, ok := strings.Cut(selection, "=")
		lang, err := normalizeLangName(lang)
		if !ok || err != nil || version == "" {
			return nil, fmt.Errorf("invalid --sdk-version %q: expected language=version-or-path", selection)
		}
		if _, exists := versions[lang]; exists {
			return nil, fmt.Errorf("duplicate --sdk-version for %s", lang)
		}
		versions[lang] = version
	}
	return versions, nil
}

func (r *Runner) runCrossLanguageBatch(ctx context.Context, config RunConfig, batch runBatch) error {
	restoreEnv := setNamespaceCapabilitiesEnv(config.NamespaceCapabilitiesJSON)
	defer restoreEnv()
	var summary Summary
	var failed bool
	for _, feature := range batch.Run.Features {
		err := r.runCrossLanguageFeature(ctx, config, feature, r.crossVersions)
		entry := SummaryEntry{Name: feature.SummaryName(), Outcome: FeaturePassed}
		if err != nil {
			entry.Outcome, entry.Message = "FAILED", err.Error()
			failed = true
		}
		summary = append(summary, entry)
	}
	label := batch.VariantName
	if label == "" {
		label = "default"
	}
	r.logFeatureSummary(label, summary)
	if failed {
		return fmt.Errorf("cross-language feature failed")
	}
	return nil
}

func (r *Runner) runCrossLanguageFeature(
	ctx context.Context, config RunConfig, feature hcmd.RunFeature, versions map[string]string,
) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	dir := filepath.Join(r.rootDir, "features", filepath.FromSlash(feature.Dir))
	files, err := filepath.Glob(filepath.Join(dir, "feature.*"))
	if err != nil {
		return err
	}
	manifest := crosslanguage.Manifest{
		Feature: feature.Dir, TaskQueue: feature.TaskQueue,
		Programs: map[string]crosslanguage.Program{},
	}
	for _, file := range files {
		lang := strings.TrimPrefix(filepath.Ext(file), ".")
		if _, err := normalizeLangName(lang); err != nil {
			continue
		}
		program, err := r.crossLanguageProgram(ctx, lang, versions[lang])
		if err != nil {
			return fmt.Errorf("prepare %s: %w", lang, err)
		}
		args := []string{"--server", config.Server, "--namespace", config.Namespace}
		for _, option := range [][2]string{
			{"--client-cert-path", config.ClientCertPath}, {"--client-key-path", config.ClientKeyPath},
			{"--ca-cert-path", config.CACertPath}, {"--tls-server-name", config.TLSServerName},
			{"--http-proxy-url", config.HTTPProxyURL},
		} {
			if option[1] != "" {
				args = append(args, option[:]...)
			}
		}
		if lang == "ts" {
			args = append([]string{"./tslib/harness/ts/main.js"}, args...)
		}
		command, err := program.NewCommand(ctx, args...)
		if err != nil {
			return err
		}
		manifest.Programs[lang] = crosslanguage.Program{Dir: command.Dir, Args: command.Args}
	}
	if len(manifest.Programs) == 0 {
		return fmt.Errorf("%s has no feature.<language> participants", feature.Dir)
	}
	buildDir, err := os.MkdirTemp(r.rootDir, "program-cross-driver-")
	if err != nil {
		return err
	}
	if !config.RetainTempDir {
		defer os.RemoveAll(buildDir)
	}
	binary := filepath.Join(buildDir, "driver")
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, ".")
	build.Dir = filepath.Join(dir, "driver")
	build.Stdout, build.Stderr = os.Stdout, os.Stderr
	if err := build.Run(); err != nil {
		return fmt.Errorf("build %s/driver: %w", feature.Dir, err)
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	command := exec.CommandContext(ctx, binary)
	command.Dir = dir
	command.Env = append(os.Environ(), crosslanguage.ManifestEnv+"="+string(data))
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	command.Cancel = func() error { return command.Process.Signal(os.Interrupt) }
	command.WaitDelay = 10 * time.Second
	if err := command.Run(); err != nil {
		return fmt.Errorf("%s driver: %w", feature.Dir, err)
	}
	return nil
}

func (r *Runner) crossLanguageProgram(ctx context.Context, lang, version string) (sdkbuild.Program, error) {
	if program := r.crossPrograms[lang]; program != nil {
		return program, nil
	}
	p := NewPreparer(PrepareConfig{Lang: lang, Version: version, crossLanguage: true})
	p.rootDir = r.rootDir
	var program sdkbuild.Program
	var err error
	switch lang {
	case "cs":
		program, err = p.BuildDotNetProgram(ctx)
	case "ts":
		program, err = p.BuildTypeScriptProgram(ctx)
	default:
		return nil, fmt.Errorf("cross-language harness roles are not implemented for %s yet", lang)
	}
	if err != nil {
		return nil, err
	}
	if r.crossPrograms == nil {
		r.crossPrograms = map[string]sdkbuild.Program{}
	}
	r.crossPrograms[lang] = program
	return program, nil
}
