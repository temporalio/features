package cmd

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"go.temporal.io/server/common/log/tag"
	"golang.org/x/mod/semver"
)

// RunFeature represents a feature on disk.
type RunFeature struct {
	Dir    string
	Config RunFeatureConfig
}

// RunFeatureConfig is JSON from a .config.json file.
type RunFeatureConfig struct {
	Go RunFeatureConfigGo `json:"go"`
}

// RunFeatureConfigGo is go-specific configuration in the JSON file.
type RunFeatureConfigGo struct {
	MinVersion string `json:"minVersion"`
}

// GlobFeatures collects all features for this runner using the given patterns
// or all features if no patterns given.
func (r *Runner) GlobFeatures(patterns []string) ([]*RunFeature, error) {
	// Collect all feature dirs that have a lang entry
	var features []*RunFeature
	featuresDir := filepath.Join(r.rootDir, "features")
	err := filepath.WalkDir(featuresDir, func(path string, _ fs.DirEntry, _ error) error {
		// Only files that are feature + ext matter
		if filepath.Base(path) != "feature."+r.config.Lang {
			return nil
		}

		// Get relative, /-slashed, dir
		dir, err := filepath.Rel(featuresDir, filepath.Dir(path))
		if err != nil {
			// Should never happen
			return err
		}
		dir = filepath.ToSlash(dir)

		// If no patterns present, all match. Otherwise, check dir or file on each.
		if len(patterns) > 0 {
			foundMatch := false
			for _, pattern := range patterns {
				match, err := filepath.Match(pattern, dir)
				if !match && err == nil {
					match, err = filepath.Match(pattern, dir+"/feature."+r.config.Lang)
				}
				if err != nil {
					return fmt.Errorf("invalid pattern %q: %w", pattern, err)
				} else if match {
					foundMatch = true
					break
				}
			}
			if !foundMatch {
				return nil
			}
		}

		// Load config
		feature := &RunFeature{Dir: dir}
		configFile := filepath.Join(filepath.Dir(path), ".config.json")
		configBytes, err := os.ReadFile(configFile)
		if err != nil {
			if !os.IsNotExist(err) {
				return fmt.Errorf("failed reading %v: %w", configFile, err)
			}
		} else if err := json.Unmarshal(configBytes, &feature.Config); err != nil {
			return fmt.Errorf("failed unmarshalling %v: %w", configFile, err)
		}

		// If there's a min version, check we're within it
		if r.config.Lang == "go" && r.config.Version != "" && feature.Config.Go.MinVersion != "" {
			if semver.Compare(r.config.Version, feature.Config.Go.MinVersion) < 0 {
				r.log.Debug("Skipping feature because version too low", tag.NewStringTag("Feature", feature.Dir),
					tag.NewStringTag("MinVersion", feature.Config.Go.MinVersion))
				return nil
			}
		}
		features = append(features, feature)

		return nil
	})
	return features, err
}
