package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v2"
	"go.temporal.io/server/common/log"
	"go.temporal.io/server/common/log/tag"
	"golang.org/x/mod/semver"
)

func buildImageCmd() *cli.Command {
	var config ImageBuildConfig
	return &cli.Command{
		Name:  "build-image",
		Usage: "build a 'prepared' single language docker image",
		Flags: config.flags(),
		Action: func(ctx *cli.Context) error {
			return NewImageBuilder(config).BuildImage(ctx.Context)
		},
	}
}

const DEFAULT_IMAGE_NAME = "sdk-features"

// ImageBuildConfig is configuration for NewImageBuilder.
type ImageBuildConfig struct {
	Lang         string
	Version      string
	RepoURL      string
	RepoRef      string
	Platform     string
	ImageName    string
	SemverLatest string
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
			Usage: "Git reference to use (mutually exclusive with version), if set, repo will be cloned to a sub directory of sdk-features." +
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
		log:     log.NewCLILogger(),
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
	i.log.Info("Building from given repo ref", tag.NewStringTag("repoURL", i.config.RepoURL), tag.NewStringTag("repoRef", i.config.RepoRef))

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

	repoDir := filepath.Join(filepath.Base(tempDir), repoBaseDir)

	// Get the actual ref (in case passed in ref is a branch name or tag)
	repoRef, err := i.gitRef(ctx, filepath.Join(tempDir, repoBaseDir, ".git"))
	if err != nil {
		return err
	}

	tags := []string{fmt.Sprintf("%s-%s", i.config.Lang, i.config.RepoRef)}

	return i.dockerBuild(ctx, buildConfig{
		tags:      tags,
		labels:    map[string]string{"SDK_REPO_URL": i.config.RepoURL, "SDK_REPO_REF": repoRef},
		buildArgs: map[string]string{"SDK_VERSION": repoDir, "REPO_DIR_OR_PLACEHOLDER": repoDir},
	})
}

func (i *ImageBuilder) buildFromVersion(ctx context.Context) error {
	version := semver.Canonical(i.config.Version)
	if version == "none" {
		// TODO: python is an exception
		return fmt.Errorf("expected version to be valid semver")
	}
	i.log.Info("Building from given version", tag.NewStringTag("version", version))

	// Build the list of tags: lang-major.minor.patch, and optionally: lang-major.minor, lang-major, lang
	tags := []string{fmt.Sprintf("%s-%s", i.config.Lang, version[1:])}
	if i.config.SemverLatest == "minor" {
		tags = append(tags, fmt.Sprintf("%s-%s", i.config.Lang, semver.MajorMinor(version)[1:]))
	} else if i.config.SemverLatest == "major" {
		tags = append(tags,
			fmt.Sprintf("%s-%s", i.config.Lang, semver.MajorMinor(version)[1:]),
			fmt.Sprintf("%s-%s", i.config.Lang, semver.Major(version)[1:]),
			i.config.Lang,
		)
	} else if i.config.SemverLatest != "" {
		return fmt.Errorf("unsupported --semver-latest: %s, valid values ('minor' or 'major')", i.config.SemverLatest)
	}
	return i.dockerBuild(ctx, buildConfig{
		tags:      tags,
		buildArgs: map[string]string{"SDK_VERSION": version, "REPO_DIR_OR_PLACEHOLDER": "main.go"},
		labels:    map[string]string{"SDK_VERSION": version},
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
		"--label",
		fmt.Sprintf("SDK_FEATURES_REF=%s", gitRef),
	}
	if i.config.Platform != "" {
		args = append(args, "--platform", i.config.Platform)
	}
	for _, tag := range config.tags {
		args = append(args, "--tag", fmt.Sprintf("%s:%s", imageName, tag))
	}
	for k, v := range config.labels {
		args = append(args, "--label", fmt.Sprintf("%s=%s", k, v))
	}
	for k, v := range config.buildArgs {
		args = append(args, "--build-arg", fmt.Sprintf("%s=%s", k, v))
	}
	args = append(args, i.rootDir)

	i.log.Info("Building docker image", tag.NewAnyTag("args", args))
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
	i.log.Info("Fetching git repo", tag.NewAnyTag("args", args))
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
