package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/temporalio/features/harness/go/harness"
	"github.com/urfave/cli/v2"
	"go.temporal.io/sdk/log"
	"golang.org/x/mod/semver"
)

func buildImageCmd() *cli.Command {
	var config ImageBuildConfig
	return &cli.Command{
		Name:  "build-image",
		Usage: "Build a 'prepared' single language docker image",
		Flags: config.flags(),
		Action: func(ctx *cli.Context) error {
			return NewImageBuilder(config).BuildImage(ctx.Context)
		},
	}
}

const DEFAULT_IMAGE_NAME = "features"

// ImageBuildConfig is configuration for NewImageBuilder.
type ImageBuildConfig struct {
	Lang         string
	Version      string
	RepoURL      string
	RepoRef      string
	Platform     string
	ImageName    string
	SemverLatest string
	DryRun       bool
}

func (c *ImageBuildConfig) flags() []cli.Flag {
	return []cli.Flag{
		langFlag(&c.Lang),
		&cli.StringFlag{
			Name:        "version",
			Usage:       "SDK language version to build (mutually exclusive with repo-ref)",
			Required:    false,
			Destination: &c.Version,
		},
		&cli.StringFlag{
			Name:        "semver-latest",
			Usage:       "Affects which additional tags to apply - only respected when passing version, ('major' or 'minor')",
			Required:    false,
			Destination: &c.SemverLatest,
		},
		&cli.StringFlag{
			Name:        "repo-url",
			Usage:       "Git repository to pull the SDK from - used if repo-ref is provided (default temporalio/sdk-<lang>)",
			Required:    false,
			Destination: &c.RepoURL,
		},
		&cli.StringFlag{
			Name: "repo-ref",
			Usage: "Git reference to use (mutually exclusive with version), if set, repo will be cloned to a sub directory of features." +
				"Used as an image tag.",
			Required:    false,
			Destination: &c.RepoRef,
		},
		&cli.StringFlag{
			Name:        "platform",
			Usage:       "Platform to build the container for (docker build --platform)",
			Required:    false,
			Destination: &c.Platform,
		},
		&cli.StringFlag{
			Name:        "image-name",
			Usage:       "Name of the image to build",
			Required:    false,
			DefaultText: DEFAULT_IMAGE_NAME,
			Destination: &c.ImageName,
		},
		&cli.BoolFlag{
			Name:        "dry-run",
			Usage:       "If set, just print the docker build command that would be run.",
			Required:    false,
			Destination: &c.DryRun,
		},
	}
}

// ImageBuilder builds docker images.
type ImageBuilder struct {
	log     log.Logger
	config  ImageBuildConfig
	rootDir string
}

// NewImageBuilder creates a new builder for the given config.
func NewImageBuilder(config ImageBuildConfig) *ImageBuilder {
	return &ImageBuilder{
		// TODO(cretz): Configurable logger
		log:     harness.NewCLILogger(),
		config:  config,
		rootDir: rootDir(),
	}
}

// BuildImage builds a docker image based on builder config.
func (i *ImageBuilder) BuildImage(ctx context.Context) error {
	var err error
	if i.config.Lang, err = normalizeLangName(i.config.Lang); err != nil {
		return err
	}

	// Build from one of: repo or version
	if i.config.RepoRef != "" {
		if i.config.Version != "" {
			return fmt.Errorf("version and repo-ref are mutually exclusive")
		}
		return i.buildFromRepo(ctx)
	} else if i.config.Version != "" {
		return i.buildFromVersion(ctx)
	} else {
		return fmt.Errorf("either version or repo-ref is required")
	}
}

func (i *ImageBuilder) buildFromRepo(ctx context.Context) error {
	if i.config.RepoURL == "" {
		var err error
		i.config.RepoURL, err = defaultRepoURL(i.config.Lang)
		if err != nil {
			return err
		}
	}
	i.log.Info("Building from given repo ref", "RepoUrl", i.config.RepoURL, "RepoRef", i.config.RepoRef)

	// We have to clone into rootDir because it's part of the docker context
	tempDir, err := os.MkdirTemp(i.rootDir, "cloned-repo-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	repoBaseDir, err := i.gitClone(ctx, tempDir)
	if err != nil {
		return err
	}

	repoDir := path.Join(filepath.Base(tempDir), repoBaseDir)

	// Get the actual ref (in case passed in ref is a branch name or tag)
	repoRef, err := i.gitRef(ctx, filepath.Join(tempDir, repoBaseDir, ".git"))
	if err != nil {
		return err
	}

	tags := []string{fmt.Sprintf("%s-%s", i.config.Lang, i.config.RepoRef)}

	return i.dockerBuild(ctx, buildConfig{
		tags: tags,
		labels: map[string]string{
			"io.temporal.sdk.repo-url": i.config.RepoURL,
			"io.temporal.sdk.repo-ref": repoRef,
		},
		buildArgs: map[string]string{"SDK_VERSION": path.Join("/app", repoDir), "REPO_DIR_OR_PLACEHOLDER": repoDir},
	})
}

func (i *ImageBuilder) buildFromVersion(ctx context.Context) error {
	if !strings.HasPrefix(i.config.Version, "v") {
		return fmt.Errorf("version must start with 'v'")
	}
	isPreRelease := strings.HasPrefix(i.config.Version, "v0")
	version := semver.Canonical(i.config.Version)
	if version == "none" || version == "" {
		if isPreRelease {
			// Account for pre-release versions which don't have a major ver #
			version = i.config.Version
		} else {
			return fmt.Errorf("expected version to be valid semver")
		}
	}
	// Build the list of tags: lang-major.minor.patch, and optionally: lang-major.minor, lang-major, lang
	tags := []string{fmt.Sprintf("%s-%s", i.config.Lang, version[1:])}
	if i.config.SemverLatest == "minor" {
		if !isPreRelease {
			tags = append(tags, fmt.Sprintf("%s-%s", i.config.Lang, semver.MajorMinor(version)[1:]))
		}
	} else if i.config.SemverLatest == "major" {
		if isPreRelease {
			tags = append(tags, i.config.Lang)
		} else {
			tags = append(tags,
				fmt.Sprintf("%s-%s", i.config.Lang, semver.MajorMinor(version)[1:]),
				fmt.Sprintf("%s-%s", i.config.Lang, semver.Major(version)[1:]),
				i.config.Lang,
			)
		}
	} else if i.config.SemverLatest != "" {
		return fmt.Errorf("unsupported --semver-latest: %s, valid values ('minor' or 'major')", i.config.SemverLatest)
	}

	i.log.Info("Building from given version", "Version", version, "Tags", tags)

	return i.dockerBuild(ctx, buildConfig{
		tags:      tags,
		buildArgs: map[string]string{"SDK_VERSION": version, "REPO_DIR_OR_PLACEHOLDER": "main.go"},
		labels: map[string]string{
			"io.temporal.sdk.version": version,
		},
	})
}

type buildConfig struct {
	tags      []string
	labels    map[string]string
	buildArgs map[string]string
}

// dockerBuild generates a docker build command and runs it
func (i *ImageBuilder) dockerBuild(ctx context.Context, config buildConfig) error {
	imageName := i.config.ImageName
	if imageName == "" {
		imageName = DEFAULT_IMAGE_NAME
	}

	gitRef, err := i.gitRef(ctx, filepath.Join(i.rootDir, ".git"))
	if err != nil {
		return err
	}

	args := []string{
		"build",
		"--pull",
		"--file",
		fmt.Sprintf("dockerfiles/%s.Dockerfile", i.config.Lang),
	}
	platform := runtime.GOARCH
	if i.config.Platform != "" {
		platform = i.config.Platform
		args = append(args, "--platform", platform)
	}

	args = append(args, "--build-arg", fmt.Sprintf("PLATFORM=%s", platform))
	var imageTagsForPublish []string
	for _, tag := range config.tags {
		tagVal := fmt.Sprintf("%s:%s", imageName, tag)
		if tag != "" {
			args = append(args, "--tag", tagVal)
		}
		imageTagsForPublish = append(imageTagsForPublish, tagVal)
	}
	// Write the most specific tag, so that the tests can use it.
	err = writeGitHubEnv("FEATURES_BUILT_IMAGE_TAG", imageTagsForPublish[0])
	if err != nil {
		return fmt.Errorf("writing test image tag to github env failed: %s", err)
	}
	// Write all the produced image tags to an env var so that the GH workflow can later use it
	// to publish them, iff the tests passed.
	err = writeGitHubEnv("FEATURES_BUILT_IMAGE_TAGS", strings.Join(imageTagsForPublish, ";"))
	if err != nil {
		return fmt.Errorf("writing image tags to github env failed: %s", err)
	}

	// TODO(bergundy): Would be nicer to print plain text instead of markdown but this good enough for now
	usage, err := (&cli.App{
		Name:  "features",
		Usage: "run a test or set of features tests",
		Flags: (&RunConfig{}).dockerRunFlags(),
	}).ToMarkdown()
	if err != nil {
		return fmt.Errorf("failed to generate usage string: %s", err)
	}
	repoURL := os.Getenv("REPO_URL")
	if repoURL == "" {
		repoURL = "https://github.com/temporalio/features"
	}

	defaultLabels := map[string]string{
		"org.opencontainers.image.created":       time.Now().UTC().Format(time.RFC3339),
		"org.opencontainers.image.source":        repoURL,
		"org.opencontainers.image.vendor":        "Temporal Technologies Inc.",
		"org.opencontainers.image.authors":       "Temporal SDK team <sdk-team@temporal.io>",
		"org.opencontainers.image.licenses":      "MIT",
		"org.opencontainers.image.revision":      gitRef,
		"org.opencontainers.image.title":         fmt.Sprintf("SDK features compliance test suite for %s", i.config.Lang),
		"org.opencontainers.image.documentation": usage,
		"io.temporal.sdk.name":                   i.config.Lang,
	}
	for k, v := range defaultLabels {
		args = append(args, "--label", fmt.Sprintf("%s=%s", k, v))
	}

	for k, v := range config.labels {
		args = append(args, "--label", fmt.Sprintf("%s=%s", k, v))
	}
	for k, v := range config.buildArgs {
		args = append(args, "--build-arg", fmt.Sprintf("%s=%s", k, v))
	}
	args = append(args, i.rootDir)

	i.log.Info("Building docker image", "Args", args)
	if i.config.DryRun {
		return nil
	}
	dockerBuild := exec.Command("docker", args...)
	dockerBuild.Stdout = os.Stdout
	dockerBuild.Stderr = os.Stderr
	return dockerBuild.Run()
}

// gitRef gets the current git HEAD's ref
func (i *ImageBuilder) gitRef(ctx context.Context, gitDir string) (string, error) {
	cmd := exec.Command("git", "--git-dir", gitDir, "rev-parse", "HEAD")
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(out), "\r\n"), nil
}

func (i *ImageBuilder) gitClone(ctx context.Context, rootDir string) (string, error) {
	expandedLangName, err := expandLangName(i.config.Lang)
	if err != nil {
		return "", err
	}
	repoBaseDir := fmt.Sprintf("sdk-%s", expandedLangName)
	targetDir := filepath.Join(rootDir, repoBaseDir)
	args := []string{"clone", "--recurse-submodules", i.config.RepoURL, targetDir}
	i.log.Info("Fetching git repo", "Args", args)
	gitClone := exec.Command("git", args...)
	gitClone.Stdout = os.Stdout
	gitClone.Stderr = os.Stderr
	return repoBaseDir, gitClone.Run()
}

func defaultRepoURL(lang string) (string, error) {
	lang, err := expandLangName(lang)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("https://github.com/temporalio/sdk-%s", lang), nil
}
