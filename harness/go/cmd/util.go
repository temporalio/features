package cmd

import "golang.org/x/mod/semver"

// GoBuildTags collects the set of tags used by different feature files based
// on the given SDK version.
func GoBuildTags(sdkVersion string) (tags []string) {
	// Add more tags as needed...

	if semver.Compare(sdkVersion, "v1.11.0") < 0 {
		tags = append(tags, "pre1.11.0")
	}

	return
}
