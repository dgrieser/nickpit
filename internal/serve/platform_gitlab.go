package serve

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/dgrieser/nickpit/internal/scm/forge"
	gitlab "github.com/dgrieser/nickpit/internal/scm/gitlab"
)

// GitLab is the daemon's GitLab platform: group webhooks on /webhooks/gitlab,
// reviewed by `nickpit gitlab mr` children.
var GitLab Platform = gitlabPlatform{}

type gitlabPlatform struct{}

func (gitlabPlatform) Forge() forge.Forge { return gitlab.Forge }

func (gitlabPlatform) WebhookPath() string { return "/webhooks/gitlab" }

func (gitlabPlatform) Decode(_ http.Header, body []byte) (Delivery, error) {
	event, err := ParseEvent(body)
	if err != nil {
		return nil, err
	}
	return event, nil
}

// Authenticate verifies the webhook against the group's configured credential:
// a GitLab signing token (HMAC over the raw body) when present, otherwise the
// legacy plaintext secret token.
func (gitlabPlatform) Authenticate(group *Group, header http.Header, body []byte, now time.Time) bool {
	if group.UsesSigning() {
		return group.CheckSignature(
			header.Get("Webhook-Id"),
			header.Get("Webhook-Timestamp"),
			header.Get("Webhook-Signature"),
			body,
			now,
		)
	}
	return group.CheckSecret(header.Get("X-Gitlab-Token"))
}

func (gitlabPlatform) AuthMethod(group *Group) string {
	if group.UsesSigning() {
		return "signing_token"
	}
	return "secret_token"
}

func (e *WebhookEvent) ProjectID() int { return e.Project.ID }

func (e *WebhookEvent) ProjectPath() string { return e.Project.PathWithNamespace }

// Decide makes the GitLab payload a Delivery; the policy itself is the
// package-level Decide.
func (e *WebhookEvent) Decide(policy Policy) Decision {
	return Decide(e, policy.TriggerEmoji, policy.MuteEmoji, policy.CommandKeyword, policy.SkipPhrases, policy.BotIDs)
}

// gitlabRemote adapts a group's GitLab client to Remote. It holds the group
// rather than its bot user id, which is resolved after construction.
type gitlabRemote struct {
	group  *Group
	client *gitlab.Client
}

// gitlabClient returns the group's GitLab API client for the features that
// exist only on GitLab (chat, response policy, topics). Nil for a group of
// another forge.
func gitlabClient(group *Group) *gitlab.Client {
	if group == nil {
		return nil
	}
	remote, _ := group.Remote.(gitlabRemote)
	return remote.client
}

func (r gitlabRemote) RequestStatus(ctx context.Context, req Request) (RequestStatus, error) {
	status, err := r.client.FetchMRStatus(ctx, req.ProjectID, req.IID)
	if err != nil {
		return RequestStatus{}, err
	}
	return RequestStatus{
		Open:    status.State == "opened",
		State:   status.State,
		Draft:   status.Draft,
		HeadSHA: status.HeadSHA,
	}, nil
}

func (r gitlabRemote) AckComment(ctx context.Context, req Request, commentID int, emoji string) error {
	return r.client.AwardNoteEmoji(ctx, req.ProjectID, req.IID, commentID, emoji)
}

func (r gitlabRemote) SetRequestReaction(ctx context.Context, req Request, add string, keep ...string) error {
	return r.client.ReplaceOwnMREmoji(ctx, req.ProjectID, req.IID, r.group.BotUserID, add, keep...)
}

func (r gitlabRemote) SetCommentReaction(ctx context.Context, req Request, commentID int, add string) error {
	return r.client.ReplaceOwnNoteEmoji(ctx, req.ProjectID, req.IID, commentID, r.group.BotUserID, add)
}

// Reply answers under the command note's discussion when the payload carried
// one and GitLab accepts the reply, as a plain MR note otherwise.
func (r gitlabRemote) Reply(ctx context.Context, req Request, threadID, body string) error {
	if threadID != "" {
		err := r.client.ReplyToMRDiscussion(ctx, req.ProjectID, req.IID, threadID, body)
		if err == nil {
			return nil
		}
		// Some GitLab versions reject replies to individual-note discussions
		// with a 4xx; fall back to a plain note. 5xx and transport errors are
		// not retried against another endpoint.
		var apiErr *gitlab.APIError
		if !errors.As(err, &apiErr) || apiErr.Status >= 500 {
			return err
		}
	}
	return r.client.CreateMRNote(ctx, req.ProjectID, req.IID, body)
}

func (gitlabRemote) ReactionFailure(err error) ReactionFailure {
	return ReactionFailure{
		Rejected:   reactionOutcomeRejected(err),
		TargetGone: terminalReactionTargetError(err),
		RetryAfter: reactionRetryAfter(err),
	}
}

// reactionOutcomeRejected identifies GitLab's permanent validation responses
// for an award POST. Callers can then fall back to revoke-only cleanup instead
// of either preserving the in-progress marker or retrying an invalid outcome.
func reactionOutcomeRejected(err error) bool {
	return matchingAPIError(err, func(apiErr *gitlab.APIError) bool {
		return apiErr.Method == http.MethodPost &&
			(apiErr.Status == http.StatusBadRequest || apiErr.Status == http.StatusUnprocessableEntity)
	})
}

// matchingAPIError walks both ordinary wrapped errors and errors.Join trees.
// Replacement can report a list failure and an award failure together, so
// errors.As alone may stop at an unrelated first API response.
func matchingAPIError(err error, match func(*gitlab.APIError) bool) bool {
	if err == nil {
		return false
	}
	if apiErr, ok := err.(*gitlab.APIError); ok {
		return match(apiErr)
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		for _, child := range joined.Unwrap() {
			if matchingAPIError(child, match) {
				return true
			}
		}
		return false
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		return matchingAPIError(wrapped.Unwrap(), match)
	}
	return false
}

// reactionRetryAfter returns the latest server-requested retry time from an
// ordinary wrapped error or errors.Join tree.
func reactionRetryAfter(err error) time.Time {
	if err == nil {
		return time.Time{}
	}
	var retryAfter time.Time
	if apiErr, ok := err.(*gitlab.APIError); ok && apiErr.RetryAfter.After(retryAfter) {
		retryAfter = apiErr.RetryAfter
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		for _, child := range joined.Unwrap() {
			if childRetryAfter := reactionRetryAfter(child); childRetryAfter.After(retryAfter) {
				retryAfter = childRetryAfter
			}
		}
		return retryAfter
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		if childRetryAfter := reactionRetryAfter(wrapped.Unwrap()); childRetryAfter.After(retryAfter) {
			retryAfter = childRetryAfter
		}
	}
	return retryAfter
}

// terminalReactionTargetError identifies a target that no longer exists. Other
// responses remain retryable: notably 400/422 can mean an invalid outcome POST,
// while the old marker still exists and needs revoke-only cleanup.
func terminalReactionTargetError(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var apiErr *gitlab.APIError
	return errors.As(err, &apiErr) &&
		(apiErr.Status == http.StatusNotFound || apiErr.Status == http.StatusGone)
}
