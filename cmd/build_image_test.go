package cmd

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestImageCloneSelectsRequestedRef(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is unavailable")
	}
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	source := t.TempDir()
	hooks := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		command := exec.Command("git", append([]string{
			"-C", source, "-c", "user.name=Test", "-c", "user.email=test@example.com",
			"-c", "commit.gpgsign=false", "-c", "core.hooksPath=" + hooks,
		}, args...)...)
		command.Env = append(os.Environ(), "GIT_EDITOR=true")
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
		return strings.TrimSpace(string(output))
	}
	git("init")
	git("commit", "--allow-empty", "-m", "first")
	want := git("rev-parse", "HEAD")
	git("commit", "--allow-empty", "-m", "second")
	builder := NewImageBuilder(ImageBuildConfig{
		Lang: "go", RepoURL: source, RepoRef: want,
	})
	base := t.TempDir()
	repo, err := builder.gitClone(context.Background(), base)
	if err != nil {
		t.Fatal(err)
	}
	if repo != "sdk-go" {
		t.Fatalf("unexpected repository directory: %s", repo)
	}
	got, err := builder.gitRef(context.Background(), filepath.Join(base, repo, ".git"))
	if err != nil || got != want {
		t.Fatalf("requested %s, got %s (%v)", want, got, err)
	}
}
