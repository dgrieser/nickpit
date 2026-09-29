package serve

import (
	"fmt"
	"strings"
	"time"
)

var (
	chatMuteAliases   = map[string]bool{"shutup": true, "mute": true, "skip": true, "ignore": true}
	chatResumeAliases = map[string]bool{"respond": true, "comment": true, "unmute": true, "resume": true}
)

// ChatDirectives are full-line controls embedded in a discussion comment.
// Remaining is the comment with recognized controls removed, ready for prompt
// history. Mute wins if a malformed comment contains both directions; Skip
// suppresses the comment's initial response but does not change persistent
// thread state. A later request reaction may explicitly override Skip when the
// comment also contains a non-control prompt.
type ChatDirectives struct {
	Mute      bool
	Resume    bool
	Request   bool
	Skip      bool
	Remaining string
}

// ParseChatDirectives scans every line for response commands and configured
// skip phrases. Matching is case-insensitive after whitespace normalization;
// commands must occupy the complete line and take no arguments.
func ParseChatDirectives(body, keyword string, skipPhrases []string) ChatDirectives {
	skip := make(map[string]bool, len(skipPhrases))
	for _, phrase := range skipPhrases {
		if normalized := normalizeControlLine(phrase); normalized != "" {
			skip[normalized] = true
		}
	}
	var out ChatDirectives
	var kept []string
	for line := range strings.SplitSeq(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		normalized := normalizeControlLine(line)
		fields := strings.Fields(normalized)
		if len(fields) == 2 && strings.EqualFold(fields[0], "/"+keyword) {
			switch {
			case chatMuteAliases[fields[1]]:
				out.Mute = true
				continue
			case chatResumeAliases[fields[1]]:
				out.Resume = true
				out.Request = true
				continue
			}
		}
		if skip[normalized] {
			out.Skip = true
			continue
		}
		kept = append(kept, line)
	}
	if out.Mute {
		out.Resume = false
		out.Request = false
	}
	out.Remaining = strings.TrimSpace(strings.Join(kept, "\n"))
	return out
}

// AllowsReply reports whether the comment these directives were parsed from
// still permits a reply. An explicit request overrides a skip phrase only —
// never a mute command, and never a comment left with no question to answer.
// The live body is re-parsed immediately before posting, so an edit made while
// the model ran is honored even for a previously requested note.
func (d ChatDirectives) AllowsReply(requested bool) bool {
	if d.Mute || d.Remaining == "" {
		return false
	}
	return requested || !d.Skip
}

func normalizeControlLine(line string) string {
	return strings.ToLower(strings.Join(strings.Fields(line), " "))
}

// CommandKind classifies a "/<keyword> <command>" note command (and the
// trigger-emoji revoke, which maps to CommandAbort).
type CommandKind int

const (
	// CommandNone marks notes that carry no command; they are ignored.
	CommandNone CommandKind = iota
	// CommandReview requests a manual review, same semantics as the trigger
	// emoji.
	CommandReview
	// CommandAbort cancels the MR's queued or running review.
	CommandAbort
	// CommandStatus asks for the MR's current review state.
	CommandStatus
	// CommandHelp asks for the command list.
	CommandHelp
	// CommandUnknown is an unrecognized subcommand after the keyword; the user
	// addressed the bot, so it gets an error reply instead of silence.
	CommandUnknown
	// CommandChat is a plain (non-keyword) reply inside a discussion thread. It
	// is a candidate for the discussion agent; the handler confirms via I/O that
	// the thread was started by nickpit (its root note carries a review marker)
	// before answering, so unrelated MR comments are dropped.
	CommandChat
)

func (k CommandKind) String() string {
	switch k {
	case CommandReview:
		return "review"
	case CommandAbort:
		return "abort"
	case CommandStatus:
		return "status"
	case CommandHelp:
		return "help"
	case CommandUnknown:
		return "unknown"
	case CommandChat:
		return "chat"
	default:
		return "none"
	}
}

// ParseCommand parses the first non-blank line of a note body for
// "/<keyword> <command>". Keyword and command match case-insensitively; extra
// fields after the command are ignored so future arguments stay
// backward-compatible ("/nickpit review me" is a review request). A bare
// "/<keyword>" asks for help. Notes that do not address the keyword return
// CommandNone. arg carries the unrecognized subcommand for CommandUnknown error
// replies.
//
// Only that one line is examined: a command quoted further down a comment (a
// reply citing an earlier request, the help text) must not execute.
func ParseCommand(body, keyword string) (kind CommandKind, arg string) {
	fields := commandFields(body)
	if len(fields) == 0 || !strings.EqualFold(fields[0], "/"+keyword) {
		return CommandNone, ""
	}
	if len(fields) == 1 {
		return CommandHelp, ""
	}
	switch strings.ToLower(fields[1]) {
	case "review":
		return CommandReview, ""
	case "abort":
		return CommandAbort, ""
	case "status":
		return CommandStatus, ""
	case "help":
		return CommandHelp, ""
	default:
		return CommandUnknown, fields[1]
	}
}

// commandFields returns the whitespace-separated fields of the body's first
// non-blank line. Leading blank lines are skipped: an editor (or a paste) that
// starts the comment with a newline must not hide the command.
func commandFields(body string) []string {
	for line := range strings.SplitSeq(body, "\n") {
		if fields := strings.Fields(line); len(fields) > 0 {
			return fields
		}
	}
	return nil
}

// The reply builders start with plain text, never with "/<keyword>", so the
// daemon's own replies can never read as commands (belt and braces on top of
// the bot-user guard in Decide).

func helpText(keyword string) string {
	return fmt.Sprintf(`NickPit commands (as a merge request comment):

- `+"`/%s review`"+` — request a review (works on drafts and non-opted-in projects)
- `+"`/%s abort`"+` — cancel the queued or running review for this merge request
- `+"`/%s status`"+` — show the review state for this merge request
- `+"`/%s help`"+` — show this help

NickPit review-thread response controls (command must be on its own line):

- `+"`/%s mute`"+` — stop responses in this review thread (`+"`shutup`"+`, `+"`skip`"+`, `+"`ignore`"+` are aliases)
- `+"`/%s respond`"+` — re-enable and request a response to text in the same comment (`+"`comment`"+`, `+"`unmute`"+`, `+"`resume`"+` are aliases)`, keyword, keyword, keyword, keyword, keyword, keyword)
}

// commandHasExtraText reports whether a command note carries anything beyond
// the "/<keyword> <command>" line itself: more words on that line, or further
// non-blank lines. Commands take no arguments, so that text is ignored and the
// author is told so.
func commandHasExtraText(body string) bool {
	seen := false
	for line := range strings.SplitSeq(body, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if seen || len(fields) > 2 {
			return true
		}
		seen = true
	}
	return false
}

// ignoredTextNotice tells the author that the text around their command was
// not used.
func ignoredTextNotice(keyword string, command CommandKind) string {
	notice := fmt.Sprintf("Text after `/%s %s` is ignored: commands take no instructions.", keyword, command)
	if command == CommandReview {
		notice += " The review runs the standard workflow; to discuss its result, reply in one of its threads."
	}
	return notice
}

// withIgnoredTextNotice prefixes a command reply with ignoredTextNotice when
// the command note carried extra text.
func withIgnoredTextNotice(decision Decision, keyword, reply string) string {
	if !decision.IgnoredText {
		return reply
	}
	return ignoredTextNotice(keyword, decision.Command) + "\n\n" + reply
}

// mentionHelpText answers an @-mention of the bot outside its own threads.
func mentionHelpText(keyword string) string {
	return fmt.Sprintf("NickPit answers only in its own review threads: reply in one of them to discuss a finding or the review. "+
		"Commands: `/%s review`, `/%s status`, `/%s abort`, `/%s help`.", keyword, keyword, keyword, keyword)
}

// mentionsUser reports whether body @-mentions username (case-insensitive,
// as GitLab matches handles). The handle must stand alone: "@nickpit-bot"
// does not mention "nickpit".
func mentionsUser(body, username string) bool {
	if username == "" {
		return false
	}
	lower, handle := strings.ToLower(body), "@"+strings.ToLower(username)
	for offset := 0; ; {
		i := strings.Index(lower[offset:], handle)
		if i < 0 {
			return false
		}
		start, end := offset+i, offset+i+len(handle)
		offset = end
		if start > 0 && isHandleRune(rune(lower[start-1])) {
			continue
		}
		if next := lower[end:]; next != "" && isHandleRune(rune(next[0])) {
			// Periods that end a sentence do not extend the handle; periods
			// followed by more handle characters do ("@nickpit.bot").
			if rest := strings.TrimLeft(next, "."); next[0] != '.' || (rest != "" && isHandleRune(rune(rest[0]))) {
				continue
			}
		}
		return true
	}
}

func isHandleRune(r rune) bool {
	return r == '_' || r == '-' || r == '.' || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
}

func unknownText(keyword, arg string) string {
	return fmt.Sprintf("Unknown command %q.\n\n%s", arg, helpText(keyword))
}

func statusText(info JobInfo) string {
	switch {
	case info.Running && info.Pending:
		return fmt.Sprintf("A review has been running for %s; another review is queued behind it.", info.Since.Round(time.Second))
	case info.Running:
		return fmt.Sprintf("A review has been running for %s.", info.Since.Round(time.Second))
	case info.Queued:
		return "A review is queued and will start shortly."
	default:
		return "No review is queued or running for this merge request."
	}
}

func abortText(outcome AbortOutcome) string {
	switch {
	case outcome.Running:
		return fmt.Sprintf("Aborted the running review (after %s).", outcome.Since.Round(time.Second))
	case outcome.Found:
		return "Removed the queued review."
	default:
		return "No review is queued or running for this merge request."
	}
}
