package serve

import (
	"strings"
	"testing"
)

// The templates exist so a comment-box insertion posts a command the daemon
// actually parses; a template whose content drifted from ParseCommand would
// hand users a note that is silently ignored or answered with "unknown
// command".
func TestCommandTemplatesParseAsCommands(t *testing.T) {
	const keyword = "nickpit"
	templates := CommandTemplates(keyword)
	if len(templates) != 4 {
		t.Fatalf("templates = %d, want one per command", len(templates))
	}
	wantKinds := map[string]CommandKind{
		"nickpit: review": CommandReview,
		"nickpit: abort":  CommandAbort,
		"nickpit: status": CommandStatus,
		"nickpit: help":   CommandHelp,
	}
	for _, template := range templates {
		want, ok := wantKinds[template.Name]
		if !ok {
			t.Fatalf("unexpected template name %q", template.Name)
		}
		kind, arg := ParseCommand(template.Content, keyword)
		if kind != want {
			t.Errorf("template %q content %q parsed as %v, want %v", template.Name, template.Content, kind, want)
		}
		if arg != "" {
			t.Errorf("template %q parsed with arg %q", template.Name, arg)
		}
		if !strings.HasPrefix(template.Name, CommandTemplatePrefix(keyword)) {
			t.Errorf("template %q does not carry the prune prefix", template.Name)
		}
	}
}

// A custom command_keyword must reach both the template names and their bodies,
// or a tenant renaming the keyword would seed commands its daemon ignores.
func TestCommandTemplatesFollowKeyword(t *testing.T) {
	templates := CommandTemplates("bot")
	for _, template := range templates {
		if !strings.HasPrefix(template.Name, "bot: ") {
			t.Errorf("template name %q does not follow the keyword", template.Name)
		}
		if !strings.HasPrefix(template.Content, "/bot ") {
			t.Errorf("template content %q does not follow the keyword", template.Content)
		}
		if kind, _ := ParseCommand(template.Content, "bot"); kind == CommandNone || kind == CommandUnknown {
			t.Errorf("template content %q parsed as %v", template.Content, kind)
		}
	}
}
