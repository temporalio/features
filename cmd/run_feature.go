package cmd

import (
	"fmt"
	"io/fs"
	"path/filepath"

	"go.temporal.io/features/harness/go/cmd"
	"golang.org/x/mod/semver"
)

// RunFeature represents a feature on disk.
type RunFeature struct {
	Dir    string
	Config cmd.RunFeatureConfig
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
		if err := feature.Config.LoadFromDir(filepath.Dir(path)); err != nil {
			return fmt.Errorf("failed reading config from %v: %w", path, err)
		}

		// If there's a min version, check we're within it
		if r.config.Lang == "go" && r.config.Version != "" && feature.Config.Go.MinVersion != "" {
			if semver.Compare(r.config.Version, feature.Config.Go.MinVersion) < 0 {
				r.log.Debug("Skipping feature because version too low", "Feature", feature.Dir,
					"MinVersion", feature.Config.Go.MinVersion)
				return nil
			}
		}
		features = append(features, feature)

		return nil
	})
	return features, err
}
