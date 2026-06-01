package cmd

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
)

func namespaceCapabilitiesEnv(capabilities map[string]bool) string {
	if len(capabilities) == 0 {
		return ""
	}
	capabilitiesJSON, _ := json.Marshal(capabilities)
	return string(capabilitiesJSON)
}

// applyNamespaceCapabilitiesEnv adds validated namespace capabilities metadata
// to a subprocess.
func applyNamespaceCapabilitiesEnv(cmd *exec.Cmd, capabilitiesJSON string) {
	if capabilitiesJSON == "" {
		return
	}
	if len(cmd.Env) == 0 {
		cmd.Env = os.Environ()
	} else {
		cmd.Env = append([]string(nil), cmd.Env...)
	}

	prefix := featureNamespaceCapabilitiesEnv + "="
	filtered := cmd.Env[:0]
	for _, entry := range cmd.Env {
		if !strings.HasPrefix(entry, prefix) {
			filtered = append(filtered, entry)
		}
	}
	cmd.Env = append(filtered, prefix+capabilitiesJSON)
}

// setNamespaceCapabilitiesEnv temporarily adds validated namespace capabilities
// metadata for in-process feature runs.
func setNamespaceCapabilitiesEnv(capabilitiesJSON string) func() {
	if capabilitiesJSON == "" {
		return func() {}
	}
	oldValue, ok := os.LookupEnv(featureNamespaceCapabilitiesEnv)
	_ = os.Setenv(featureNamespaceCapabilitiesEnv, capabilitiesJSON)
	return func() {
		if ok {
			_ = os.Setenv(featureNamespaceCapabilitiesEnv, oldValue)
		} else {
			_ = os.Unsetenv(featureNamespaceCapabilitiesEnv)
		}
	}
}
