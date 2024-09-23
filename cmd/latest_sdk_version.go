package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

func getLatestSdkVersion(config LatestSdkVersionConfig) error {
	var sdk string
	sdk, err := expandLangName(config.Lang)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://api.github.com/repos/temporalio/sdk-%s/releases/latest", sdk)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to query the GH API for SDK version: %w", err)
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read body of GitHub Get request: %w", err)
	}

	var version struct {
		TagName string `json:"tag_name"`
	}
	err = json.Unmarshal(body, &version)
	if err != nil {
		return fmt.Errorf("failed to decode json response: %w", err)
	}

	fmt.Println(strings.TrimPrefix(version.TagName, "v"))

	return nil
}
