package testutil

import (
	"os"
	"strings"
)

// ClearAmbientEnv unsets the environment variables that steer config loading:
// the NICKPIT_* settings, the provider API keys the built-in profiles reference
// as "$..._API_KEY", and the forge tokens. A developer shell that exports any of
// them — NICKPIT_PROFILE and NICKPIT_GITLAB_TOKEN are the usual ones — otherwise
// feeds a real profile and real credentials into tests that assert on a bare
// environment, so the same test passes in CI and fails locally.
//
// Call it from TestMain, before any test runs; individual tests still set what
// they need with t.Setenv, which restores the (now empty) value afterwards.
func ClearAmbientEnv() {
	for _, entry := range os.Environ() {
		name, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if isAmbientConfigEnv(name) {
			// The name comes from os.Environ, so it is always valid and the
			// only error Unsetenv can report cannot happen here.
			_ = os.Unsetenv(name)
		}
	}
}

func isAmbientConfigEnv(name string) bool {
	if name == "GITLAB_BASE_URL" {
		return true
	}
	return strings.HasPrefix(name, "NICKPIT_") ||
		strings.HasSuffix(name, "_API_KEY") ||
		strings.HasSuffix(name, "_TOKEN")
}
