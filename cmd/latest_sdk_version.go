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
		Usage: "get the latest SDK version from the package registry",
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

// registryQuery describes how to fetch the latest version from a package registry.
type registryQuery struct {
	url     string
	extract func(body []byte) (string, error)
}

// registryQueries maps normalized language names to their package registry queries.
var registryQueries = map[string]registryQuery{
	"go": {
		url: "https://proxy.golang.org/go.temporal.io/sdk/@latest",
		extract: func(body []byte) (string, error) {
			var resp struct {
				Version string `json:"Version"`
			}
			if err := json.Unmarshal(body, &resp); err != nil {
				return "", err
			}
			return resp.Version, nil
		},
	},
	// All @temporalio/* packages are published from the same monorepo at the same
	// version, so checking any one package (client) gives the version for all.
	"typescript": {
		url: "https://registry.npmjs.org/@temporalio/client/latest",
		extract: func(body []byte) (string, error) {
			var resp struct {
				Version string `json:"version"`
			}
			if err := json.Unmarshal(body, &resp); err != nil {
				return "", err
			}
			return resp.Version, nil
		},
	},
	"java": {
		url: "https://search.maven.org/solrsearch/select?q=g:io.temporal+AND+a:temporal-sdk&rows=1&wt=json",
		extract: func(body []byte) (string, error) {
			var resp struct {
				Response struct {
					Docs []struct {
						LatestVersion string `json:"latestVersion"`
					} `json:"docs"`
				} `json:"response"`
			}
			if err := json.Unmarshal(body, &resp); err != nil {
				return "", err
			}
			if len(resp.Response.Docs) == 0 {
				return "", fmt.Errorf("no results found on Maven Central")
			}
			return resp.Response.Docs[0].LatestVersion, nil
		},
	},
	"php": {
		url: "https://repo.packagist.org/p2/temporal/sdk.json",
		extract: func(body []byte) (string, error) {
			var resp struct {
				Packages map[string][]struct {
					Version string `json:"version"`
				} `json:"packages"`
			}
			if err := json.Unmarshal(body, &resp); err != nil {
				return "", err
			}
			versions := resp.Packages["temporal/sdk"]
			if len(versions) == 0 {
				return "", fmt.Errorf("no versions found on Packagist")
			}
			return versions[0].Version, nil
		},
	},
	"python": {
		url: "https://pypi.org/pypi/temporalio/json",
		extract: func(body []byte) (string, error) {
			var resp struct {
				Info struct {
					Version string `json:"version"`
				} `json:"info"`
			}
			if err := json.Unmarshal(body, &resp); err != nil {
				return "", err
			}
			return resp.Info.Version, nil
		},
	},
	"dotnet": {
		url: "https://api.nuget.org/v3-flatcontainer/temporalio/index.json",
		extract: func(body []byte) (string, error) {
			var resp struct {
				Versions []string `json:"versions"`
			}
			if err := json.Unmarshal(body, &resp); err != nil {
				return "", err
			}
			// Find the latest stable version (no pre-release suffix)
			for i := len(resp.Versions) - 1; i >= 0; i-- {
				if !strings.Contains(resp.Versions[i], "-") {
					return resp.Versions[i], nil
				}
			}
			return "", fmt.Errorf("no stable versions found on NuGet")
		},
	},
	"ruby": {
		url: "https://rubygems.org/api/v1/gems/temporalio.json",
		extract: func(body []byte) (string, error) {
			var resp struct {
				Version string `json:"version"`
			}
			if err := json.Unmarshal(body, &resp); err != nil {
				return "", err
			}
			return resp.Version, nil
		},
	},
}

func getLatestSdkVersion(config LatestSdkVersionConfig) error {
	lang, err := expandLangName(config.Lang)
	if err != nil {
		return err
	}

	query, ok := registryQueries[lang]
	if !ok {
		return fmt.Errorf("no package registry configured for language %q", lang)
	}

	resp, err := http.Get(query.url)
	if err != nil {
		return fmt.Errorf("failed to query package registry: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("package registry returned status %d for %s", resp.StatusCode, query.url)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	version, err := query.extract(body)
	if err != nil {
		return fmt.Errorf("failed to extract version from response: %w", err)
	}

	fmt.Println(strings.TrimPrefix(version, "v"))

	return nil
}
