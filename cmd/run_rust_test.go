package cmd

import (
	"strings"
	"testing"
)

func TestRustLanguageAndRegistry(t *testing.T) {
	for _, name := range []string{"rs", "rust"} {
		if got, err := normalizeLangName(name); err != nil || got != "rs" {
			t.Fatalf("normalize %q: %q, %v", name, got, err)
		}
		if got, err := expandLangName(name); err != nil || got != "rust" {
			t.Fatalf("expand %q: %q, %v", name, got, err)
		}
		if got, err := defaultRepoURL(name); err != nil || !strings.HasSuffix(got, "/sdk-rust") {
			t.Fatalf("repo %q: %q, %v", name, got, err)
		}
	}
	query := registryQueries["rust"]
	got, err := query.extract([]byte(`{"crate":{"max_version":"2.0.0-alpha.1","max_stable_version":"1.2.3","newest_version":"0.9.0"}}`))
	if err != nil || got != "1.2.3" {
		t.Fatalf("latest stable version: %q, %v", got, err)
	}
	if _, err := query.extract([]byte(`{"crate":{"max_stable_version":null}}`)); err == nil {
		t.Fatal("missing stable release must fail")
	}
}
