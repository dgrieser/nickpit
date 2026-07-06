package serve

import (
	"fmt"
	"strings"
	"time"
)

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
	default:
		return "none"
	}
}

// ParseCommand parses the first line of a note body for
// "/<keyword> <command>". Keyword and command match case-insensitively; extra
// fields after the command are ignored so future arguments stay
// backward-compatible. A bare "/<keyword>" asks for help. Notes that do not
// address the keyword return CommandNone. arg carries the unrecognized
// subcommand for CommandUnknown error replies.
func ParseCommand(body, keyword string) (kind CommandKind, arg string) {
	firstLine, _, _ := strings.Cut(body, "\n")
	fields := strings.Fields(firstLine)
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

// The reply builders start with plain text, never with "/<keyword>", so the
// daemon's own replies can never read as commands (belt and braces on top of
// the bot-user guard in Decide).

func helpText(keyword string) string {
	return fmt.Sprintf(`NickPit commands (as a merge request comment):

- `+"`/%s review`"+` — request a review (works on drafts and non-opted-in projects)
- `+"`/%s abort`"+` — cancel the queued or running review for this merge request
- `+"`/%s status`"+` — show the review state for this merge request
- `+"`/%s help`"+` — show this help`, keyword, keyword, keyword, keyword)
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
