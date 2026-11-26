package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPublishImages(t *testing.T) {
	amd64 := "sha256:" + strings.Repeat("a", 64)
	arm64 := "sha256:" + strings.Repeat("b", 64)
	for _, tt := range []struct {
		name      string
		tagsFile  string
		platforms string
	}{
		{name: "default tags file"},
		{name: "explicit tags file", tagsFile: "tags-outside-digests.txt"},
		{name: "required platforms", platforms: "arm64,amd64"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, "digest-arm64"), []byte(arm64+"\n"), 0o644))
			require.NoError(t, os.WriteFile(filepath.Join(dir, "digest-amd64"), []byte(" \n"+amd64+"\n"), 0o644))
			require.NoError(t, os.WriteFile(filepath.Join(dir, "ignored"), []byte("invalid"), 0o644))
			tagsFile := filepath.Join(dir, "tags.txt")
			config := PublishImageConfig{repoPrefix: "registry.example/test", digestsDir: dir, platforms: tt.platforms}
			if tt.tagsFile != "" {
				tagsFile = filepath.Join(t.TempDir(), tt.tagsFile)
				config.tagsFile = tagsFile
			}
			require.NoError(t, os.WriteFile(tagsFile, []byte(" features:go-1.2.3;features:go-1;other:latest\n"), 0o644))
			var commands [][]string
			err := publishImages(config, func(args ...string) error {
				commands = append(commands, args)
				return nil
			})
			require.NoError(t, err)
			require.Equal(t, [][]string{
				{"buildx", "imagetools", "create", "--tag", "registry.example/test/features:go-1.2.3", "registry.example/test/features@" + amd64, "registry.example/test/features@" + arm64},
				{"buildx", "imagetools", "create", "--tag", "registry.example/test/features:go-1", "registry.example/test/features@" + amd64, "registry.example/test/features@" + arm64},
				{"buildx", "imagetools", "create", "--tag", "registry.example/test/other:latest", "registry.example/test/other@" + amd64, "registry.example/test/other@" + arm64},
			}, commands)
		})
	}
}

func TestPublishImagesRejectsInvalidInputsBeforePublishing(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	for _, tt := range []struct {
		name      string
		digests   map[string]string
		tags      string
		omitTags  bool
		platforms string
		wantError string
	}{
		{name: "missing digests", tags: "features:go", wantError: "no digest files"},
		{name: "empty digest", digests: map[string]string{"digest-amd64": " \n"}, tags: "features:go", wantError: "invalid digest"},
		{name: "malformed digest", digests: map[string]string{"digest-amd64": "sha256:xyz"}, tags: "features:go", wantError: "invalid digest"},
		{name: "non hex digest", digests: map[string]string{"digest-amd64": "sha256:" + strings.Repeat("z", 64)}, tags: "features:go", wantError: "invalid digest"},
		{name: "missing tags file", digests: map[string]string{"digest-amd64": digest}, omitTags: true, wantError: "reading image tags"},
		{name: "empty tags", digests: map[string]string{"digest-amd64": digest}, tags: " \n", wantError: "no image tags"},
		{name: "only tag separators", digests: map[string]string{"digest-amd64": digest}, tags: " ; \n; ", wantError: "no image tags"},
		{name: "invalid later tag", digests: map[string]string{"digest-amd64": digest}, tags: "features:go;invalid", wantError: "invalid image tag"},
		{name: "invalid later tag syntax", digests: map[string]string{"digest-amd64": digest}, tags: "features:go;features:bad/tag", wantError: "invalid image tag"},
		{name: "invalid later image name", digests: map[string]string{"digest-amd64": digest}, tags: "features:go;Features:latest", wantError: "invalid image tag"},
		{name: "empty tag", digests: map[string]string{"digest-amd64": digest}, tags: "features:go;", wantError: "invalid image tag"},
		{name: "invalid docker tag", digests: map[string]string{"digest-amd64": digest}, tags: "features:bad tag", wantError: "invalid image tag"},
		{name: "missing platform", digests: map[string]string{"digest-amd64": digest}, tags: "features:go", platforms: "amd64,arm64", wantError: "digest files do not match --platforms"},
		{name: "wrong platform names", digests: map[string]string{"digest-amd64": digest, "digest-other": digest}, tags: "features:go", platforms: "amd64,arm64", wantError: "digest files do not match --platforms"},
		{name: "extra platform", digests: map[string]string{"digest-amd64": digest, "digest-arm64": digest, "digest-other": digest}, tags: "features:go", platforms: "amd64,arm64", wantError: "digest files do not match --platforms"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range tt.digests {
				require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
			}
			if !tt.omitTags {
				require.NoError(t, os.WriteFile(filepath.Join(dir, "tags.txt"), []byte(tt.tags), 0o644))
			}
			var commands [][]string
			err := publishImages(PublishImageConfig{repoPrefix: "temporaliotest", digestsDir: dir, platforms: tt.platforms}, func(args ...string) error {
				commands = append(commands, args)
				return nil
			})
			require.ErrorContains(t, err, tt.wantError)
			require.Empty(t, commands)
		})
	}
}

func TestPublishImagesStopsOnCommandFailure(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "digest-amd64"), []byte("sha256:"+strings.Repeat("a", 64)), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tags.txt"), []byte("features:go;features:latest"), 0o644))
	want := errors.New("docker failed")
	calls := 0
	err := publishImages(PublishImageConfig{repoPrefix: "temporaliotest", digestsDir: dir}, func(args ...string) error {
		calls++
		return want
	})
	require.ErrorIs(t, err, want)
	require.Equal(t, 1, calls)
}

func TestPushImagesDryRun(t *testing.T) {
	dir := t.TempDir()
	digest := "sha256:" + strings.Repeat("a", 64)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "digest-amd64"), []byte(digest), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tags.txt"), []byte("features:go"), 0o644))
	t.Setenv("PATH", t.TempDir())
	var output bytes.Buffer
	app := NewApp()
	app.Writer = &output
	require.NoError(t, app.Run([]string{"features", "push-images", "--digests-dir", dir, "--dry-run"}))
	require.Contains(t, output.String(), "docker buildx imagetools create --tag temporaliotest/features:go temporaliotest/features@"+digest)
}
