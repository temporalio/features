package cmd

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	hcmd "github.com/temporalio/features/harness/go/cmd"
	"github.com/temporalio/features/sdkbuild"
	"github.com/urfave/cli/v2"
)

func TestCrossLanguageFeatureDiscovery(t *testing.T) {
	root := t.TempDir()
	for path, contents := range map[string]string{
		"features/tracing/encoded/config.json": `{"crossLanguage":true}`,
		"features/tracing/encoded/feature.cs":  "",
		"features/tracing/encoded/feature.ts":  "",
		"features/tracing/plain/feature.ts":    "",
		"features/tracing/plain/config.json":   `{}`,
	} {
		path = filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, test := range []struct {
		lang, pattern, want string
	}{
		{"", "", "tracing/encoded"},
		{"", "features/tracing/encoded", "tracing/encoded"},
		{"", "tracing/encoded/config.json", "tracing/encoded"},
		{"ts", "", "tracing/plain"},
		{"cs", "", ""},
		{"ts", "tracing/encoded", ""},
	} {
		runner := NewRunner(RunConfig{PrepareConfig: PrepareConfig{Lang: test.lang}})
		runner.rootDir = root
		var patterns []string
		if test.pattern != "" {
			patterns = []string{test.pattern}
		}
		var features []*RunFeature
		var err error
		if test.lang == "" {
			features, err = runner.GlobCrossLanguageFeatures(patterns)
		} else {
			features, err = runner.GlobFeatures(patterns)
		}
		if err != nil {
			t.Fatal(err)
		}
		if test.want == "" {
			if len(features) != 0 {
				t.Fatalf("lang=%q pattern=%q: unwanted features %v", test.lang, test.pattern, features)
			}
		} else if len(features) != 1 || features[0].Dir != test.want {
			t.Fatalf("lang=%q pattern=%q: got %v, want %s", test.lang, test.pattern, features, test.want)
		}
	}
}

func TestCrossLanguageSDKSelections(t *testing.T) {
	for _, selections := range [][]string{{"ts"}, {"ts="}, {"unknown=1"}, {"ts=1", "typescript=2"}} {
		config := CrossLanguageRunConfig{SDKVersions: *cli.NewStringSlice(selections...)}
		if _, err := config.sdkVersions(); err == nil {
			t.Fatalf("accepted invalid SDK selections %v", selections)
		}
	}
	config := CrossLanguageRunConfig{SDKVersions: *cli.NewStringSlice("dotnet=1.21.1", "ts=/local/sdk")}
	versions, err := config.sdkVersions()
	if err != nil || versions["cs"] != "1.21.1" || versions["ts"] != "/local/sdk" {
		t.Fatalf("unexpected selections %v, %v", versions, err)
	}
}

type preparedCrossLanguageProgram struct{ dir string }

func (p preparedCrossLanguageProgram) Dir() string { return p.dir }
func (p preparedCrossLanguageProgram) NewCommand(ctx context.Context, args ...string) (*exec.Cmd, error) {
	command := exec.CommandContext(ctx, "unused-harness", args...)
	command.Dir = p.dir
	return command, nil
}

func TestCrossLanguageDriverExitControlsFeatureOutcome(t *testing.T) {
	for _, exit := range []string{"0", "7"} {
		t.Run(exit, func(t *testing.T) {
			root := t.TempDir()
			for path, contents := range map[string]string{
				"go.mod":                     "module driver-test\n\ngo 1.26.0\n",
				"features/sample/feature.ts": "",
				"features/sample/driver/main.go": `package main
import ("os"; "strings")
func main() {
    if !strings.Contains(os.Getenv("FEATURE_CROSS_LANGUAGE"), "\"Feature\":\"sample\"") { os.Exit(8) }
    os.Exit(` + exit + `)
}`,
			} {
				path = filepath.Join(root, path)
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
					t.Fatal(err)
				}
			}
			runner := NewRunner(RunConfig{})
			runner.rootDir = root
			runner.crossPrograms = map[string]sdkbuild.Program{
				"ts": preparedCrossLanguageProgram{root},
			}
			err := runner.runCrossLanguageBatch(t.Context(), RunConfig{}, runBatch{
				Run: &hcmd.Run{Features: []hcmd.RunFeature{{Dir: "sample", TaskQueue: "queue"}}},
			})
			if (err != nil) != (exit != "0") {
				t.Fatalf("driver exit=%s: unexpected result %v", exit, err)
			}
		})
	}
}
