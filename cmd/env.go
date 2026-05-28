package cmd

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
)

func variantEnv(variant string, capabilities map[string]bool) map[string]string {
	if variant == "" && len(capabilities) == 0 {
		return nil
	}
	env := make(map[string]string, 2)
	if variant != "" {
		env[featureRunVariantEnv] = variant
	}
	if len(capabilities) > 0 {
		capabilitiesJSON, _ := json.Marshal(capabilities)
		env[featureNamespaceCapabilitiesEnv] = string(capabilitiesJSON)
	}
	return env
}

// applyCommandEnv adds harness-owned feature metadata to a subprocess.
func applyCommandEnv(cmd *exec.Cmd, env map[string]string) {
	if len(env) == 0 {
		return
	}
	if len(cmd.Env) == 0 {
		cmd.Env = os.Environ()
	} else {
		cmd.Env = append([]string(nil), cmd.Env...)
	}
	for key := range env {
		prefix := key + "="
		filtered := cmd.Env[:0]
		for _, entry := range cmd.Env {
			if !strings.HasPrefix(entry, prefix) {
				filtered = append(filtered, entry)
			}
		}
		cmd.Env = filtered
	}
	for key, value := range env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
}

// setProcessEnv temporarily adds harness-owned feature metadata for in-process feature runs.
func setProcessEnv(env map[string]string) func() {
	if len(env) == 0 {
		return func() {}
	}
	type prevValue struct {
		value string
		ok    bool
	}
	prev := make(map[string]prevValue, len(env))
	for key, value := range env {
		oldValue, ok := os.LookupEnv(key)
		prev[key] = prevValue{value: oldValue, ok: ok}
		_ = os.Setenv(key, value)
	}
	return func() {
		for key, value := range prev {
			if value.ok {
				_ = os.Setenv(key, value.value)
			} else {
				_ = os.Unsetenv(key)
			}
		}
	}
}
