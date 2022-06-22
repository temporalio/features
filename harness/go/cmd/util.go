package cmd

import "golang.org/x/mod/semver"

// GoBuildTags collects the set of tags used by different feature files based
// on the given SDK version.
func GoBuildTags(sdkVersion string) (tags []string) {
	// When realtive paths are used or an otherwise non-semver version is used, we
	// need to assume there are no tags
	if !semver.IsValid(sdkVersion) {
		return nil
	}

	// Add more tags as needed...
	if semver.Compare(sdkVersion, "v1.11.0") < 0 {
		tags = append(tags, "pre1.11.0")
	}
	if semver.Compare(sdkVersion, "v1.12.0") < 0 {
		tags = append(tags, "pre1.12.0")
	}

	return
}
