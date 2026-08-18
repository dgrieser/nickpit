package serve

import (
	"fmt"

	gitlab "github.com/dgrieser/nickpit/internal/scm/gitlab"
)

// GitLab has no extension point for quick actions: the "/" autocomplete in a
// comment box is fed by the server's own command definitions, and a registered
// quick action would additionally be stripped from the note body before the
// webhook fires, hiding the very command the daemon parses. Comment templates
// ("saved replies") are the supported path to the same discoverability: GitLab
// offers user-owned and group-owned templates in the comment box's template
// picker, and inserting one leaves the command text in the note.

// CommandTemplatePrefix returns the name prefix marking the comment templates
// nickpit owns in a scope, so a sync can prune its own stale templates without
// touching hand-written ones.
func CommandTemplatePrefix(keyword string) string {
	return keyword + ": "
}

// CommandTemplates returns the comment templates mirroring the note commands
// ParseCommand accepts, in help order. The content is the bare command line so
// inserting a template and submitting posts exactly what the daemon parses.
func CommandTemplates(keyword string) []gitlab.SavedReply {
	prefix := CommandTemplatePrefix(keyword)
	commands := []string{"review", "abort", "status", "help"}
	templates := make([]gitlab.SavedReply, 0, len(commands))
	for _, command := range commands {
		templates = append(templates, gitlab.SavedReply{
			Name:    prefix + command,
			Content: fmt.Sprintf("/%s %s", keyword, command),
		})
	}
	return templates
}
