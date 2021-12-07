package io.temporal.sdkfeatures;

import com.google.common.base.Preconditions;

import java.util.Objects;

public class Version {
  public static final Version SDK = new Version(io.temporal.internal.Version.LIBRARY_VERSION);

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
    Preconditions.checkArgument(pieces.length == 3, "must have 3 parts of version");

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

    // Set fields, letting exceptions be thrown on invalid
    this.major = Integer.parseInt(pieces[0]);
    this.minor = Integer.parseInt(pieces[1]);
    this.patch = Integer.parseInt(pieces[2]);
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
    return major == version.major && minor == version.minor && patch == version.patch && hasExtra == version.hasExtra;
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
