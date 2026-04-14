package llm

import (
	"bytes"
	"strings"
	"text/template"
)

func RenderPrompt(tmplText string, data any) (string, error) {
	tmpl, err := template.New("prompt").Parse(tmplText)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()), nil
}
