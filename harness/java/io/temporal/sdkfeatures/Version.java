package io.temporal.sdkfeatures;

import java.util.Objects;

public class Version {
  public static final Version SDK = new Version(io.temporal.serviceclient.Version.LIBRARY_VERSION);

  public final int major, minor, patch;
  public final boolean hasExtra;

  public Version(int major, int minor, int patch, boolean hasExtra) {
    this.major = major;
    this.minor = minor;
    this.patch = patch;
    this.hasExtra = hasExtra;
  }

  public Version(String str) {
    // Split
    var pieces = str.split("\\.", 3);
    if (pieces.length != 3) {
      // Non-semver version string (e.g. git hash from a fork build without tags).
      // Default to 0.0.0 so all past histories are checked.
      this.major = 0;
      this.minor = 0;
      this.patch = 0;
      this.hasExtra = true;
      return;
    }

    // Check if it has an extra value and trim off
    var hasExtra = false;
    var extraIndex = pieces[2].lastIndexOf('-');
    if (extraIndex > 0) {
      hasExtra = true;
      pieces[2] = pieces[2].substring(0, extraIndex);
    }
    extraIndex = pieces[2].lastIndexOf('+');
    if (extraIndex > 0) {
      hasExtra = true;
      pieces[2] = pieces[2].substring(0, extraIndex);
    }

    // Set fields. Fall back to 0.0.0 if parts aren't valid integers
    // (e.g. version derived from git describe on a fork without tags).
    int maj, min, pat;
    try {
      maj = Integer.parseInt(pieces[0]);
      min = Integer.parseInt(pieces[1]);
      pat = Integer.parseInt(pieces[2]);
    } catch (NumberFormatException e) {
      maj = 0;
      min = 0;
      pat = 0;
      hasExtra = true;
    }
    this.major = maj;
    this.minor = min;
    this.patch = pat;
    this.hasExtra = hasExtra;
  }

  public int compareTo(Version other) {
    var c = Integer.compare(major, other.major);
    if (c == 0) {
      c = Integer.compare(minor, other.minor);
    }
    if (c == 0) {
      c = Integer.compare(patch, other.patch);
    }
    // One that has extra is always less than the one that doesn't
    if (c == 0) {
      c = Boolean.compare(other.hasExtra, hasExtra);
    }
    return c;
  }

  @Override
  public boolean equals(Object o) {
    if (this == o) return true;
    if (o == null || getClass() != o.getClass()) return false;
    Version version = (Version) o;
    return major == version.major
        && minor == version.minor
        && patch == version.patch
        && hasExtra == version.hasExtra;
  }

  @Override
  public int hashCode() {
    return Objects.hash(major, minor, patch, hasExtra);
  }

  @Override
  public String toString() {
    var s = major + "." + minor + "." + patch;
    // TODO(cretz): Capture exact extra?
    if (hasExtra) {
      s += "-extra";
    }
    return s;
  }
}
