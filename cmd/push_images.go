package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/temporalio/features/harness/go/harness"
	"github.com/urfave/cli/v2"
)

func publishImageCmd() *cli.Command {
	var config PublishImageConfig
	return &cli.Command{
		Name:  "push-images",
		Usage: "Publish multi-platform docker image tags. Used by CI",
		Flags: config.flags(),
		Action: func(ctx *cli.Context) error {
			logger := harness.NewCLILogger()
			return publishImages(config, func(args ...string) error {
				logger.Info("Publishing docker image", "Args", args)
				if config.dryRun {
					_, err := fmt.Fprintln(ctx.App.Writer, "docker "+strings.Join(args, " "))
					return err
				}
				command := exec.CommandContext(ctx.Context, "docker", args...)
				command.Stdout = os.Stdout
				command.Stderr = os.Stderr
				return command.Run()
			})
		},
	}
}

// PublishImageConfig stores config for the publish-image command.
type PublishImageConfig struct {
	repoPrefix string
	digestsDir string
	tagsFile   string
	platforms  string
	dryRun     bool
}

func (c *PublishImageConfig) flags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:        "repo-prefix",
			Usage:       "Prefix for the docker image repository",
			Required:    false,
			Destination: &c.repoPrefix,
			Value:       "temporaliotest",
		},
		&cli.StringFlag{
			Name:        "digests-dir",
			Usage:       "Directory containing digest-* files for the images to publish",
			Required:    true,
			Destination: &c.digestsDir,
		},
		&cli.StringFlag{
			Name:        "tags-file",
			Usage:       "File containing semicolon-separated image:tag values",
			DefaultText: "tags.txt under --digests-dir",
			Destination: &c.tagsFile,
		},
		&cli.StringFlag{
			Name:        "platforms",
			Usage:       "Optional comma-separated platforms requiring exactly the matching digest-<platform> files",
			Destination: &c.platforms,
		},
		&cli.BoolFlag{
			Name:        "dry-run",
			Usage:       "If set, just print the docker commands that would be run",
			Destination: &c.dryRun,
		},
	}
}

func publishImages(config PublishImageConfig, runDocker func(...string) error) error {
	entries, err := os.ReadDir(config.digestsDir)
	if err != nil {
		return fmt.Errorf("reading image digests: %w", err)
	}
	var digestFiles, digests []string
	digestPattern := regexp.MustCompile(`^sha256:[a-fA-F0-9]{64}$`)
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "digest-") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(config.digestsDir, entry.Name()))
		if err != nil {
			return fmt.Errorf("reading image digest: %w", err)
		}
		digest := strings.TrimSpace(string(data))
		if !digestPattern.MatchString(digest) {
			return fmt.Errorf("invalid digest in %s: expected sha256 followed by 64 hex digits", entry.Name())
		}
		digestFiles = append(digestFiles, entry.Name())
		digests = append(digests, digest)
	}
	if len(digests) == 0 {
		return fmt.Errorf("no digest files found in %s", config.digestsDir)
	}
	if config.platforms != "" {
		var expected []string
		for _, platform := range strings.Split(config.platforms, ",") {
			expected = append(expected, "digest-"+strings.TrimSpace(platform))
		}
		slices.Sort(expected)
		if !slices.Equal(digestFiles, expected) {
			return fmt.Errorf("digest files do not match --platforms: expected %v, found %v", expected, digestFiles)
		}
	}
	if config.tagsFile == "" {
		config.tagsFile = filepath.Join(config.digestsDir, "tags.txt")
	}
	data, err := os.ReadFile(config.tagsFile)
	if err != nil {
		return fmt.Errorf("reading image tags: %w", err)
	}
	if strings.Trim(string(data), "; \t\r\n") == "" {
		return fmt.Errorf("no image tags found in %s", config.tagsFile)
	}
	tags := strings.Split(string(data), ";")
	const repositoryComponent = `[a-z0-9]+(?:(?:[._]|__|-+)[a-z0-9]+)*`
	tagPattern := regexp.MustCompile(`^` + repositoryComponent + `(?:/` + repositoryComponent + `)*:[a-zA-Z0-9_][a-zA-Z0-9_.-]{0,127}$`)
	for index, tag := range tags {
		tags[index] = strings.TrimSpace(tag)
		if !tagPattern.MatchString(tags[index]) {
			return fmt.Errorf("invalid image tag %q in %s: expected image:tag", tags[index], config.tagsFile)
		}
	}
	for _, tag := range tags {
		image, _, _ := strings.Cut(tag, ":")
		args := []string{"buildx", "imagetools", "create", "--tag", config.repoPrefix + "/" + tag}
		for _, digest := range digests {
			args = append(args, config.repoPrefix+"/"+image+"@"+digest)
		}
		if err := runDocker(args...); err != nil {
			return fmt.Errorf("publishing docker image %s: %w", tag, err)
		}
	}

	return nil
}
