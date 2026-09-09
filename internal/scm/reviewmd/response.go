package reviewmd

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

const (
	responseFooterStart  = MarkerOpen + "response-footer:start -->"
	responseFooterEnd    = MarkerOpen + "response-footer:end -->"
	responseCommandMuted = MarkerOpen + "response-command-muted -->"
	responsePolicyPrefix = MarkerOpen + "response-policy:"
)

// ResponseStatus describes the effective discussion-response mode rendered on
// a visible nickpit review root. CommandMuted is persisted in the root body;
// the other fields are current configuration or live reaction state.
type ResponseStatus struct {
	Enabled        bool
	OptIn          bool
	CommandMuted   bool
	MRMuted        bool
	ThreadMuted    bool
	MuteEmoji      string
	RequestEmoji   string
	CommandKeyword string
	// RequestTerm is the platform's word for the change under review: "MR" on
	// GitLab, "PR" on GitHub. Empty defaults to "MR".
	RequestTerm string
}

// ThreadCommandMuted reports whether a review root carries the bot-controlled
// persistent mute marker.
func ThreadCommandMuted(body string) bool {
	return strings.Contains(body, responseCommandMuted)
}

// HasResponseFooter reports whether a body already carries a rendered response
// section. Callers use it to reconcile only roots that have never been
// stamped, instead of re-reading reactions for every root on a merge request.
func HasResponseFooter(body string) bool {
	return strings.Contains(body, responseFooterStart)
}

// PolicyFingerprint identifies the CONFIGURATION inputs that shape a rendered
// footer's instructions. Live per-thread state (mute reactions, the command
// marker) is deliberately excluded: those change through their own events,
// which reconcile the affected root directly.
func (s ResponseStatus) PolicyFingerprint() string {
	sum := sha256.Sum256([]byte(strings.Join([]string{
		strconv.FormatBool(s.Enabled),
		strconv.FormatBool(s.OptIn),
		s.MuteEmoji,
		s.RequestEmoji,
		s.CommandKeyword,
		s.requestTerm(),
	}, "\x00")))
	return hex.EncodeToString(sum[:8])
}

// FooterMatchesPolicy reports whether a body carries a footer that was
// rendered from this configuration. A footer stamped under earlier settings —
// a renamed mute emoji, a different keyword, chat switched off — advertises
// controls the daemon no longer honors, so callers that skip already-stamped
// roots must re-stamp those.
func FooterMatchesPolicy(body string, status ResponseStatus) bool {
	return strings.Contains(body, responsePolicyMarker(status.PolicyFingerprint()))
}

// requestTerm is the platform's word for the change under review, defaulted so
// an unset field and an explicit "MR" fingerprint — and read — identically.
func (s ResponseStatus) requestTerm() string {
	if term := Sanitize(strings.TrimSpace(s.RequestTerm)); term != "" {
		return term
	}
	return "MR"
}

func responsePolicyMarker(fingerprint string) string {
	return responsePolicyPrefix + fingerprint + " -->"
}

// StripResponseFooter removes the complete visible response-mode section and
// its hidden state markers. Callers use it before any comment reaches an LLM.
func StripResponseFooter(body string) string {
	for {
		start := strings.Index(body, responseFooterStart)
		if start < 0 {
			return strings.TrimSpace(body)
		}
		endRel := strings.Index(body[start+len(responseFooterStart):], responseFooterEnd)
		if endRel < 0 {
			return strings.TrimSpace(body[:start])
		}
		end := start + len(responseFooterStart) + endRel + len(responseFooterEnd)
		body = body[:start] + body[end:]
	}
}

// UpsertResponseFooter replaces any prior response section and appends the
// current one. The hidden command marker makes command muting survive daemon
// restarts without introducing a second state store.
func UpsertResponseFooter(body string, status ResponseStatus) string {
	base := StripResponseFooter(body)
	var b strings.Builder
	b.WriteString(base)
	if base != "" {
		b.WriteString("\n\n")
	}
	b.WriteString(responseFooterStart)
	b.WriteString("\n")
	b.WriteString(responsePolicyMarker(status.PolicyFingerprint()))
	b.WriteString("\n")
	if status.CommandMuted {
		b.WriteString(responseCommandMuted)
		b.WriteString("\n")
	}
	// Disabled chat renders no visible footer at all, but the hidden markers
	// stay: they carry the persistent command-mute state and the policy
	// fingerprint that lets SyncNewRoots skip already-reconciled roots.
	resolved := false
	for _, env := range CollectFindingEnvelopes(base) {
		resolved = resolved || env.Finding.Resolution != nil
	}
	if text := responseStatusText(status); text != "" && !resolved {
		b.WriteString("---\n\n*")
		b.WriteString(text)
		b.WriteString("*\n")
	}
	b.WriteString(responseFooterEnd)
	return b.String()
}

// TransferResponseFooter keeps live command/policy metadata when a correction
// replaces the review content. Resolved findings retain hidden controls only.
func TransferResponseFooter(previous, current string) string {
	previous = StripHistory(previous)
	start := strings.Index(previous, responseFooterStart)
	if start < 0 {
		return current
	}
	end := strings.Index(previous[start:], responseFooterEnd)
	if end < 0 {
		return current
	}
	footer := previous[start : start+end+len(responseFooterEnd)]
	for _, env := range CollectFindingEnvelopes(current) {
		if env.Finding.Resolution != nil {
			if visible := strings.Index(footer, "---\n"); visible >= 0 {
				footer = footer[:visible] + responseFooterEnd
			}
		}
	}
	return StripResponseFooter(current) + "\n\n" + footer
}

func responseStatusText(status ResponseStatus) string {
	keyword := Sanitize(strings.TrimPrefix(strings.TrimSpace(status.CommandKeyword), "/"))
	if keyword == "" {
		keyword = "nickpit"
	}
	command := func(alias string) string { return fmt.Sprintf("`/%s %s`", keyword, alias) }
	muteEmoji := Sanitize(strings.TrimSpace(status.MuteEmoji))
	requestEmoji := Sanitize(strings.TrimSpace(status.RequestEmoji))
	requestTerm := status.requestTerm()

	if !status.Enabled {
		return ""
	}

	var blockers []string
	if status.MRMuted {
		blockers = append(blockers, fmt.Sprintf("remove :%s: from %s", muteEmoji, requestTerm))
	}
	if status.ThreadMuted {
		blockers = append(blockers, fmt.Sprintf("remove :%s: from thread", muteEmoji))
	}
	if status.CommandMuted {
		blockers = append(blockers, "add "+command("resume")+" on its own line to your comment")
	}
	if len(blockers) > 0 {
		return "NickPit is muted. To unmute, " + strings.Join(blockers, "; ") + "."
	}

	var muteInstructions []string
	if muteEmoji != "" {
		muteInstructions = append(muteInstructions,
			fmt.Sprintf("react with :%s: on this post to mute this thread or on %s to mute all NickPit threads", muteEmoji, requestTerm))
	}
	muteInstructions = append(muteInstructions, "add "+command("mute")+" on its own line to your comment")
	muteText := strings.Join(muteInstructions, ", or ")
	if status.OptIn {
		request := "NickPit responds if you add " + command("respond") + " on its own line to your comment"
		if requestEmoji != "" {
			request += fmt.Sprintf(" or react with :%s: on the question comment", requestEmoji)
		}
		return request + ". To mute, " + muteText + "."
	}
	return "NickPit responds to comments. To mute, " + muteText + "."
}
