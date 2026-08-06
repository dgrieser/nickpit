// Package configtest holds helpers shared by tests that load nickpit
// configuration. It deliberately imports nothing from internal/config so the
// config package's own in-package tests can use it without an import cycle.
package configtest

import (
	"os"
	"strings"
)

// credentialEnvNames are the non-NICKPIT_ variables config resolution reads.
// They are cleared alongside every NICKPIT_* variable so a developer's shell or
// a CI runner cannot make a test pass (or fail) through ambient credentials.
var credentialEnvNames = []string{
	"GITHUB_TOKEN", "GITLAB_TOKEN", "GITLAB_BASE_URL",
	"OPENROUTER_API_KEY", "MITTWALD_LLM_API_KEY", "MISTRAL_API_KEY",
	"DEEPSEEK_API_KEY", "DASHSCOPE_API_KEY", "NVIDIA_API_KEY",
}

// ClearAmbientEnv removes every environment variable that influences config
// loading. Call it from TestMain before running any test.
func ClearAmbientEnv() {
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(name, "NICKPIT_") {
			_ = os.Unsetenv(name)
		}
	}
	for _, name := range credentialEnvNames {
		_ = os.Unsetenv(name)
	}
}
