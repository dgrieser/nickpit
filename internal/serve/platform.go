package serve

import (
	"context"
	"net/http"
	"time"

	"github.com/dgrieser/nickpit/internal/scm/forge"
)

// Platform is the forge a daemon serves: how its webhook deliveries are decoded
// and authenticated, and which nickpit command reviews one of its requests. The
// review path (handler, dispatcher, worker, runner) only talks to a forge
// through Platform and Remote; chat, emoji triggers, and comment templates stay
// GitLab features and use the GitLab client directly (gitlabClient).
type Platform interface {
	// Forge names the review child's command and its credential environment.
	Forge() forge.Forge
	// WebhookPath is the HTTP route deliveries arrive on.
	WebhookPath() string
	// Decode parses one delivery. It runs BEFORE authentication, because the
	// project path selects the group whose credential is checked, so it must
	// have no side effects.
	Decode(header http.Header, body []byte) (Delivery, error)
	// Authenticate verifies a delivery against the group's credential.
	Authenticate(group *Group, header http.Header, body []byte, now time.Time) bool
	// AuthMethod labels the group's verification method for logs.
	AuthMethod(group *Group) string
}

// Delivery is one decoded webhook: the project it belongs to and the pure
// trigger policy over its payload.
type Delivery interface {
	ProjectID() int
	ProjectPath() string
	Decide(policy Policy) Decision
}

// Policy is the configuration Delivery.Decide classifies against.
type Policy struct {
	// TriggerEmoji is the reaction requesting a manual review.
	TriggerEmoji string
	// MuteEmoji is the reaction muting a discussion thread.
	MuteEmoji string
	// CommandKeyword is the "/<keyword> <command>" comment-command keyword.
	CommandKeyword string
	SkipPhrases    []string
	// BotIDs are the daemon's own users, whose events are ignored.
	BotIDs map[int]bool
}

// Request addresses one merge/pull request. Forges differ in which half they
// key their API on, so both the numeric id and the path travel together.
type Request struct {
	ProjectID   int
	ProjectPath string
	IID         int
}

// RequestStatus is the authoritative state re-read before a review runs. State
// is the forge's own word for it, kept for logs; Open is the policy input.
type RequestStatus struct {
	Open    bool
	State   string
	Draft   bool
	HeadSHA string
	// BaseSHA is the diff base (merge base); with HeadSHA it identifies the
	// request's current diff.
	BaseSHA string
}

// Remote is what the review path asks of a group's forge account.
type Remote interface {
	RequestStatus(ctx context.Context, req Request) (RequestStatus, error)
	// AckComment adds emoji to a command comment.
	AckComment(ctx context.Context, req Request, commentID int, emoji string) error
	// SetRequestReaction replaces the bot's own reactions on the request with
	// add ("" revokes only), leaving the keep names in place.
	SetRequestReaction(ctx context.Context, req Request, add string, keep ...string) error
	// SetCommentReaction is SetRequestReaction for one comment.
	SetCommentReaction(ctx context.Context, req Request, commentID int, add string) error
	// Reply posts a command reply, under threadID where the forge threads
	// comments and the id is known.
	Reply(ctx context.Context, req Request, threadID, body string) error
	// ReactionFailure classifies an error from the reaction calls above.
	ReactionFailure(err error) ReactionFailure
}

// ReactionFailure is how a failed reaction update should be handled. The zero
// value is a plain retryable failure.
type ReactionFailure struct {
	// Rejected: the forge permanently refused the reaction being added; the
	// caller falls back to revoke-only cleanup.
	Rejected bool
	// TargetGone: the request or comment no longer exists; retrying is useless.
	TargetGone bool
	// RetryAfter is the earliest time the forge asked to be called again.
	RetryAfter time.Time
}
