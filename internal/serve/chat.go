package serve

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/retrieval"
	"github.com/dgrieser/nickpit/internal/review"
	gitlab "github.com/dgrieser/nickpit/internal/scm/gitlab"
	"github.com/dgrieser/nickpit/internal/scm/reviewmd"
)

// errNotNickpitThread marks a discussion whose root note carries no nickpit
// review marker, so it is not a thread the daemon started and must be ignored.
var errNotNickpitThread = errors.New("serve: not a nickpit discussion thread")

// ChatService answers discussion-thread replies with the discussion agent,
// in-process. It is built once from the daemon's LLM profile and reused across
// groups; each call is given the group's GitLab client and bot user id.
type ChatService struct {
	profile config.Profile
	log     *slog.Logger
}

// NewChatService builds a chat service from the daemon's LLM profile.
func NewChatService(profile config.Profile, log *slog.Logger) *ChatService {
	return &ChatService{profile: profile, log: log}
}

// Reply answers a discussion-thread reply. It confirms the thread is a nickpit
// thread (root note carries a review marker), reassembles the review it belongs
// to from the MR's hidden markers, rebuilds the review context from the live MR,
// runs one discussion turn seeded with the thread's messages, and posts the
// answer back into the thread. A non-nickpit thread returns errNotNickpitThread
// and posts nothing.
func (s *ChatService) Reply(ctx context.Context, client *gitlab.Client, projectID int, projectPath string, iid int, discussionID string, botUserID int) error {
	notes, err := client.DiscussionNoteBodies(ctx, projectPath, iid, discussionID)
	if err != nil {
		return fmt.Errorf("serve chat: reading thread: %w", err)
	}
	if len(notes) == 0 {
		return errNotNickpitThread
	}
	reviewID, findingID, ok := detectThreadReview(notes[0].Body)
	if !ok {
		return errNotNickpitThread
	}

	bodies, err := client.MRNoteBodies(ctx, projectPath, iid)
	if err != nil {
		return fmt.Errorf("serve chat: reading MR notes: %w", err)
	}
	result := reviewmd.ReviewResultsByID(bodies)[reviewID]
	if result == nil {
		return fmt.Errorf("serve chat: review %q not found on MR", reviewID)
	}

	reviewCtx, err := client.FetchMR(ctx, projectPath, iid, true)
	if err != nil {
		return fmt.Errorf("serve chat: resolving MR context: %w", err)
	}

	history := chatThreadToMessages(notes, botUserID)
	if len(history) == 0 {
		return nil // nothing new to answer (e.g. only the root note)
	}

	engine := s.engine(client)
	res, err := engine.Discuss(ctx, review.DiscussRequest{
		ReviewCtx:             reviewCtx,
		Result:                result,
		PinnedFindingID:       findingID,
		Messages:              history,
		DiffFormat:            s.profile.DiffFormat,
		DisableSuggestions:    s.profile.DisableSuggestions,
		MaxToolCalls:          s.profile.MaxToolCalls,
		MaxDuplicateToolCalls: s.profile.MaxDuplicateToolCalls,
		MaxOutputRetries:      s.profile.MaxOutputRetries,
		MaxReasoningSeconds:   s.profile.MaxReasoningSeconds,
	})
	if err != nil {
		return fmt.Errorf("serve chat: discussion agent: %w", err)
	}
	reply := strings.TrimSpace(res.Reply)
	if reply == "" {
		return nil
	}
	if err := client.ReplyToMRDiscussion(ctx, projectID, iid, discussionID, reviewmd.Sanitize(reply)); err != nil {
		return fmt.Errorf("serve chat: posting reply: %w", err)
	}
	return nil
}

// engine builds a review engine for the discussion agent from the daemon's
// profile and the group's GitLab client.
func (s *ChatService) engine(client *gitlab.Client) *review.Engine {
	llmClient := llm.NewOpenAIClient(s.profile.BaseURL, s.profile.APIKey, s.profile.Model)
	source := gitlab.NewAdapter(client, s.profile.AssetBaseURL)
	engine := review.NewEngine(source, llmClient, retrieval.NewLocalEngine(), s.profile)
	engine.SetDisabledStyleGuides(s.profile.DisableStyleGuides)
	return engine
}

// detectThreadReview inspects a discussion's root note for a nickpit carrier
// marker. A finding carrier pins the chat to that finding; a review carrier means
// a whole-review chat. Reports ok=false when the note carries no nickpit marker.
func detectThreadReview(rootBody string) (reviewID, findingID string, ok bool) {
	if fes := reviewmd.CollectFindingEnvelopes(rootBody); len(fes) > 0 {
		return fes[0].ReviewID, fes[0].Finding.ID, true
	}
	if res := reviewmd.CollectReviewEnvelopes(rootBody); len(res) > 0 {
		return res[0].ReviewID, "", true
	}
	return "", "", false
}

// chatThreadToMessages maps a discussion's notes to conversation messages for the
// discussion agent. The root note (the finding or summary comment) is skipped —
// it is the review context, represented by the agent's own opener. The daemon's
// own prior replies (author == botUserID) become assistant turns; everyone
// else's notes become user turns. System notes are dropped.
func chatThreadToMessages(notes []gitlab.DiscussionNote, botUserID int) []llm.Message {
	var msgs []llm.Message
	for i, note := range notes {
		if i == 0 || note.System {
			continue
		}
		body := strings.TrimSpace(note.Body)
		if body == "" {
			continue
		}
		role := "user"
		if botUserID != 0 && note.AuthorID == botUserID {
			role = "assistant"
		}
		msgs = append(msgs, llm.Message{Role: role, Content: body})
	}
	return msgs
}
