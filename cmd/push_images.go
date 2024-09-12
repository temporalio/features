package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/urfave/cli/v2"
)

func publishImageCmd() *cli.Command {
	var config PublishImageConfig
	return &cli.Command{
		Name:  "push-images",
		Usage: "Push docker image(s) to our test repository. Used by CI",
		Flags: config.flags(),
		Action: func(ctx *cli.Context) error {
			return publishImages(config)
		},
	}
}

// PublishImageConfig stores config for the publish-image command.
type PublishImageConfig struct {
	repoPrefix string
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
	}
}

func publishImages(config PublishImageConfig) error {
	tagsFromEnv := strings.Split(os.Getenv("FEATURES_BUILT_IMAGE_TAGS"), ";")
	if len(tagsFromEnv) == 0 {
		return fmt.Errorf("no image tags found in FEATURES_BUILT_IMAGE_TAGS")
	}

	var pushedTags []string
	for _, tag := range tagsFromEnv {
		pushAs := fmt.Sprintf("%s/%s", config.repoPrefix, tag)
		dockerTag := exec.Command("docker", "tag", tag, pushAs)
		dockerTag.Stdout = os.Stdout
		dockerTag.Stderr = os.Stderr
		err := dockerTag.Run()
		if err != nil {
			return fmt.Errorf("failed to tag docker image: %w", err)
		}
		pushedTags = append(pushedTags, pushAs)
	}

	for _, tag := range pushedTags {
		dockerPush := exec.Command("docker", "push", tag)
		dockerPush.Stdout = os.Stdout
		dockerPush.Stderr = os.Stderr
		err := dockerPush.Run()
		if err != nil {
			return fmt.Errorf("failed to push docker image: %w", err)
		}
	}

	return nil
}
