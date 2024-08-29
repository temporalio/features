package cmd

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/urfave/cli/v2"
)

func latestSdkVersionCmd() *cli.Command {
	var config LatestSdkVersionConfig
	return &cli.Command{
		Name:  "latest-sdk-version",
		Usage: "get the latest SDK version",
		Flags: config.flags(),
		Action: func(ctx *cli.Context) error {
			return getLatestSdkVersion(config)
		},
	}
}

type LatestSdkVersionConfig struct {
	Lang string
}

func (p *LatestSdkVersionConfig) flags() []cli.Flag {
	return []cli.Flag{
		langFlag(&p.Lang),
	}
}

type Version struct {
	Name string `json:"name"`
}

func getLatestSdkVersion(config LatestSdkVersionConfig) error {
	var sdk string
	switch config.Lang {
	case "go":
		sdk = "go"
	case "java":
		sdk = "java"
	case "ts":
		sdk = "typescript"
	case "py":
		sdk = "python"
	case "cs":
		sdk = "dotnet"
	default:
		return fmt.Errorf("unrecognized language")
	}
	url := fmt.Sprintf("https://api.github.com/repos/temporalio/sdk-%s/releases/latest", sdk)
	curl := exec.Command("curl", url)
	out, err := curl.Output()
	if err != nil {
		return fmt.Errorf("failed to query the GH API for SDK version: %w", err)
	}
	var version Version
	err = json.Unmarshal(out, &version)

	if err != nil {
		return fmt.Errorf("failed to decode json response: %w", err)
	}

	fmt.Println(strings.Replace(version.Name, "v", "", 1))

	return nil
}
