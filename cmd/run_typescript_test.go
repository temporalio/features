package cmd

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func mockTypeScriptRegistry(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	original := registryQueries["typescript"]
	query := original
	query.url = server.URL
	registryQueries["typescript"] = query
	t.Cleanup(func() { registryQueries["typescript"] = original })
}

func TestTypeScriptVersionDefaultsToLatest(t *testing.T) {
	requests := 0
	mockTypeScriptRegistry(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method: %s", r.Method)
		}
		_, _ = w.Write([]byte(`{"version":"2.0.0"}`))
	})
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"),
		[]byte(`{"dependencies":{"@temporalio/client":"1.25.0"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	p := Preparer{rootDir: root}
	version, err := p.typeScriptVersion()
	if err != nil {
		t.Fatal(err)
	}
	if version != "2.0.0" {
		t.Fatalf("expected latest release independent of repository pins, got %q", version)
	}
	if requests != 1 {
		t.Fatalf("expected one registry query, got %d", requests)
	}
}
