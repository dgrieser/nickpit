package prompts

import (
	"embed"
	"fmt"
)

// FS stores the built-in prompt templates shipped inside the binary.
//
//go:embed *.tmpl
var FS embed.FS

func Load(name string) (string, error) {
	data, err := FS.ReadFile(name)
	if err != nil {
		return "", fmt.Errorf("prompts: reading %s: %w", name, err)
	}
	return string(data), nil
}
