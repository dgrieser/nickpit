package main

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dgrieser/nickpit/internal/model"
	ghscm "github.com/dgrieser/nickpit/internal/scm/github"
	glscm "github.com/dgrieser/nickpit/internal/scm/gitlab"
	"github.com/dgrieser/nickpit/internal/scm/reviewmd"
	"github.com/dgrieser/nickpit/internal/session"
	"github.com/dgrieser/nickpit/internal/textsan"
)

// The session picker lists the reviews NickPit published on the project's open
// merge and pull requests beside the sessions saved on this machine: a review
// the serve daemon or a colleague's `--publish` run produced lives on the
// request, not here, and is just as much a review of this branch.
const (
	// maxRemoteRequests caps how many open requests are inspected. Each one
	// costs reading its comments, and the prompt waits for all of them; the
	// newest by activity are the ones kept.
	maxRemoteRequests = 15
	// remoteFetchWorkers is how many of those are read at once — enough to hide
	// the latency of a busy project, few enough not to look like an attack on
	// the API.
	remoteFetchWorkers = 6
	// remoteFetchTimeout bounds the whole listing. The prompt is unusable while
	// it runs, so it is short: what came back in time is listed, and the status
	// line says the rest was cut off.
	remoteFetchTimeout = 8 * time.Second
)

// remoteReview is one review published on an open request: the row the picker
// shows, plus what the actions need to read it back or chat about it.
type remoteReview struct {
	info    session.Info
	mode    model.ReviewMode
	repo    string
	id      int
	branch  string
	title   string
	baseURL string
	result  *model.ReviewResult
}

// remoteSource is the platform half of the listing: how to list the open
// requests of a project, and how to read the reviews one of them carries.
type remoteSource struct {
	mode    model.ReviewMode
	repo    string
	baseURL string
	// closed says the listing reaches past the open requests to the merged and
	// closed ones, which is what the scope's own wording has to say too.
	closed  bool
	list    func(ctx context.Context) ([]model.OpenRequest, error)
	reviews func(ctx context.Context, id int) (map[string]*model.ReviewResult, error)
	// threads reads the replies NickPit's own review threads collected: the
	// answers to the summary and to each finding, which is the conversation
	// that belongs with the review. Everything else said on the request is
	// somebody else's thread and is left there.
	threads func(ctx context.Context, id int) ([]reviewThread, error)
}

// remoteFinder gathers the published reviews of the project the current
// checkout points at, once per invocation however many scopes ask for them.
type remoteFinder struct {
	once    sync.Once
	source  *remoteSource
	reviews []remoteReview
	err     error
}

// newRemoteFinder prepares the lookup for the project of the current
// directory. It resolves nothing yet: a picker whose remote scope is never
// drawn must cost no API call. closed widens the listing past the open
// requests to the merged and closed ones, whose reviews are still readable.
func newRemoteFinder(a *app, place sessionPlace, closed bool) *remoteFinder {
	return &remoteFinder{source: a.remoteSourceFor(place, closed)}
}

// available reports whether there is a project to ask at all.
func (f *remoteFinder) available() bool { return f != nil && f.source != nil }

// byID finds a fetched review by its id, nil when the id is not one of them —
// which is how a row is told apart from a saved session's.
func (f *remoteFinder) byID(id string) *remoteReview {
	if !f.available() || id == "" {
		return nil
	}
	for i := range f.reviews {
		if f.reviews[i].info.ID == id {
			return &f.reviews[i]
		}
	}
	return nil
}

// find returns the published reviews, fetching them on the first call. A
// failure is returned to the caller and remembered, so one unreachable server
// is reported once and not retried on every scope switch.
func (f *remoteFinder) find(ctx context.Context) ([]remoteReview, error) {
	if !f.available() {
		return nil, nil
	}
	f.once.Do(func() {
		bounded, cancel := context.WithTimeout(ctx, remoteFetchTimeout)
		defer cancel()
		f.reviews, f.err = f.source.collect(bounded)
	})
	return f.reviews, f.err
}

// remoteSourceFor builds the platform half for the project the checkout points
// at: GitHub when its remote says so, GitLab otherwise, and nothing at all
// outside a checkout, without a project, or without the token that platform
// needs — a picker must still open where no request can be read.
func (a *app) remoteSourceFor(place sessionPlace, closed bool) *remoteSource {
	if place.repo == "" {
		return nil
	}
	profile, err := a.loadProfileWithoutLLM()
	if err != nil {
		return nil
	}
	repo := place.repo
	if isGitHubRemote(place.remoteURL) {
		if profile.GitHubToken == "" {
			return nil
		}
		client := ghscm.NewClient("", profile.GitHubToken)
		adapter := ghscm.NewAdapter(client, profile.AssetBaseURL)
		return &remoteSource{
			mode:   model.ModeGitHub,
			repo:   repo,
			closed: closed,
			list: func(ctx context.Context) ([]model.OpenRequest, error) {
				if closed {
					return client.ListPRs(ctx, repo)
				}
				return client.ListOpenPRs(ctx, repo)
			},
			reviews: func(ctx context.Context, id int) (map[string]*model.ReviewResult, error) {
				// Read without the author check: a project reviewed by another
				// group's bot (or by a colleague's token) carries markers this
				// token cannot claim, and the scope exists to show exactly
				// those. Nothing here writes back.
				return adapter.ReviewResultsAnyAuthor(ctx, repo, id)
			},
			// GitHub's comment API is read here without thread structure, so a
			// pull request's review prints without the replies to it.
			threads: func(context.Context, int) ([]reviewThread, error) { return nil, nil },
		}
	}
	if profile.GitLabToken == "" || !sameHost(place.remoteURL, profile.GitLabBaseURL) {
		// A remote on a host this profile has no credentials for is not this
		// profile's GitLab: asking it would send the token to the wrong server.
		return nil
	}
	client := glscm.NewClient(profile.GitLabBaseURL, profile.GitLabToken)
	adapter := glscm.NewAdapter(client, profile.AssetBaseURL)
	return &remoteSource{
		mode:    model.ModeGitLab,
		repo:    repo,
		baseURL: profile.GitLabBaseURL,
		closed:  closed,
		list: func(ctx context.Context) ([]model.OpenRequest, error) {
			if closed {
				return client.ListMRs(ctx, repo)
			}
			return client.ListOpenMRs(ctx, repo)
		},
		reviews: func(ctx context.Context, id int) (map[string]*model.ReviewResult, error) {
			// Read without the author check: see the GitHub branch above.
			return adapter.ReviewResultsAnyAuthor(ctx, repo, id)
		},
		threads: func(ctx context.Context, id int) ([]reviewThread, error) {
			discussions, err := client.MRDiscussions(ctx, repo, id)
			if err != nil {
				return nil, err
			}
			return gitlabReviewThreads(discussions), nil
		},
	}
}

// collect lists the open requests and reads the reviews each of them carries.
// Requests are read in parallel because the listing is one round trip and the
// reviews are one per request; a request that cannot be read is skipped rather
// than failing the listing, since the others are still worth showing.
func (s *remoteSource) collect(ctx context.Context) ([]remoteReview, error) {
	requests, err := s.list(ctx)
	if err != nil {
		if ctx.Err() != nil {
			// The bound fired: say that in a line that fits, rather than
			// showing a truncated API URL with a deadline error inside it.
			return nil, fmt.Errorf("the server did not list the %s within %s", s.noun(), remoteFetchTimeout)
		}
		return nil, fmt.Errorf("listing the %s: %w", s.noun(), err)
	}
	if len(requests) > maxRemoteRequests {
		requests = requests[:maxRemoteRequests]
	}
	var (
		mu      sync.Mutex
		wg      sync.WaitGroup
		out     []remoteReview
		work    = make(chan model.OpenRequest)
		workers = min(remoteFetchWorkers, len(requests))
	)
	for range workers {
		wg.Go(func() {
			for request := range work {
				reviews, err := s.reviews(ctx, request.Identifier)
				if err != nil {
					// One request that cannot be read (a permission, a rate
					// limit) must not cost the listing every other one.
					continue
				}
				rows := s.rowsOf(request, reviews)
				mu.Lock()
				out = append(out, rows...)
				mu.Unlock()
			}
		})
	}
	for _, request := range requests {
		select {
		case work <- request:
		case <-ctx.Done():
		}
	}
	close(work)
	wg.Wait()
	if ctx.Err() != nil {
		// Partial is not failed: the requests that answered in time are listed,
		// with a note that the server did not finish answering.
		return out, fmt.Errorf("%d open %s read before the server ran out of time", len(out), s.noun())
	}
	return out, nil
}

// noun is the platform's word for what was being read.
func (s *remoteSource) noun() string {
	if s.mode == model.ModeGitHub {
		return "pull requests"
	}
	return "merge requests"
}

// scopeNoun names what the remote scope is listing, which the flag widens.
func (f *remoteFinder) scopeNoun() string {
	if !f.available() {
		return "open merge and pull requests"
	}
	if f.source.closed {
		return "the " + f.source.noun() + " of the project"
	}
	return "the open " + f.source.noun() + " of the project"
}

// rowsOf turns the reviews of one request into rows: one per review, since a
// re-reviewed request carries several and each is its own conversation. The
// carrier already collapses a corrected review onto its newest revision, so
// what is listed is the current state of each.
func (s *remoteSource) rowsOf(request model.OpenRequest, reviews map[string]*model.ReviewResult) []remoteReview {
	rows := make([]remoteReview, 0, len(reviews))
	for id, result := range reviews {
		if result == nil {
			continue
		}
		updated := result.CreatedAt
		if updated.IsZero() {
			updated = request.UpdatedAt
		}
		rows = append(rows, remoteReview{
			info: session.Info{
				ID:       id,
				ReviewID: id,
				Source: session.Source{
					Mode:       string(s.mode),
					Repo:       s.repo,
					Identifier: request.Identifier,
					Branch:     request.SourceBranch,
					BaseURL:    s.baseURL,
				},
				Model:     result.Model,
				HasResult: true,
				Revision:  result.Revision,
				Verdict:   result.OverallCorrectness,
				Findings:  len(result.Findings),
				CreatedAt: result.CreatedAt,
				UpdatedAt: updated,
			},
			mode:    s.mode,
			repo:    s.repo,
			id:      request.Identifier,
			branch:  request.SourceBranch,
			title:   request.Title,
			baseURL: s.baseURL,
			result:  result,
		})
	}
	return rows
}

// sortRemoteReviews puts what the user is working on first: the reviews of the
// checked-out branch, newest first, then everything else, newest first.
func sortRemoteReviews(rows []remoteReview, branch string) {
	sort.SliceStable(rows, func(i, j int) bool {
		mine, theirs := rows[i].branch == branch && branch != "", rows[j].branch == branch && branch != ""
		if mine != theirs {
			return mine
		}
		return rows[i].info.UpdatedAt.After(rows[j].info.UpdatedAt)
	})
}

// isGitHubRemote reports whether a remote URL points at github.com. Anything
// else is treated as the profile's GitLab, which is what NickPit's own commands
// assume.
func isGitHubRemote(remote string) bool {
	host := remoteHost(remote)
	return host == "github.com" || strings.HasSuffix(host, ".github.com")
}

// sameHost reports whether a git remote and an API base URL name the same
// server, so a token is only ever offered to the host it belongs to. An
// unreadable remote counts as the same host: the git remote is then no evidence
// either way, and the profile's own host is what the rest of NickPit uses.
func sameHost(remote, baseURL string) bool {
	host := remoteHost(remote)
	if host == "" {
		return true
	}
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed.Hostname() == "" {
		return false
	}
	return strings.EqualFold(host, parsed.Hostname())
}

// remoteHost is the host of a git remote URL, in either of the two shapes git
// writes: a URL with a scheme, or the scp-style "git@host:group/project.git".
func remoteHost(remote string) string {
	remote = strings.TrimSpace(remote)
	if remote == "" {
		return ""
	}
	if strings.Contains(remote, "://") {
		parsed, err := url.Parse(remote)
		if err != nil {
			return ""
		}
		return parsed.Hostname()
	}
	before, _, ok := strings.Cut(remote, ":")
	if !ok {
		return ""
	}
	if _, host, found := strings.Cut(before, "@"); found {
		return host
	}
	return before
}

// actOnRemoteReview carries out the chosen action on a review that lives on an
// open request: print it (with the conversation around it), copy that same text
// unstyled, or chat about it — which resumes the review from the request's own
// markers and puts the discussion into the conversation.
func (a *app) actOnRemoteReview(ctx context.Context, remote *remoteFinder, row remoteReview,
	action sessionAction, opts sessionOptions, w io.Writer) error {
	if action == sessionActionChat {
		return a.chatAboutRemoteReview(ctx, row)
	}
	if opts.history {
		return fmt.Errorf("session: --history reads a saved session's archived revisions; %s carries only its current review",
			remoteRequestLabel(row))
	}
	render := func(out io.Writer) error { return a.writeRemoteReview(ctx, remote.source, row, out) }
	if opts.warnings {
		render = func(out io.Writer) error { return a.formatWarnings(out, row.result) }
	}
	subject := "review"
	if opts.warnings {
		subject = "warnings"
	}
	if err := a.emitReview(ctx, w, action == sessionActionCopy, subject, remoteOriginLabel(row), render); err != nil {
		return fmt.Errorf("session: %w", err)
	}
	return nil
}

// chatAboutRemoteReview opens a chat about a review that lives on a request.
// The chat front-end reassembles it from the request's own markers, so the
// conversation starts from what is published there rather than from anything
// saved here.
func (a *app) chatAboutRemoteReview(ctx context.Context, row remoteReview) error {
	if row.mode != model.ModeGitLab {
		return fmt.Errorf("session: chatting about a %s is not supported yet; print or copy it instead",
			remoteRequestLabel(row))
	}
	return a.runChat(ctx, chatOptions{
		gitlab:   true,
		repo:     row.repo,
		mrID:     row.id,
		reviewID: row.result.ReviewID,
		// The row was listed without the author check, so the chat has to
		// reassemble it the same way or it would find nothing.
		anyMarkerAuthor: true,
		// The request's discussion belongs in the conversation: what was
		// already said about a finding is half of what there is to discuss.
		withComments: true,
	}, nil)
}

// remoteOriginLabel names where a copied review came from, the way the feedback
// command names it in its clipboard confirmation.
func remoteOriginLabel(row remoteReview) string {
	if row.mode == model.ModeGitHub {
		return fmt.Sprintf("GitHub PR %s#%d", textsan.StripControl(row.repo), row.id)
	}
	return fmt.Sprintf("GitLab MR %s!%d", textsan.StripControl(row.repo), row.id)
}

// writeRemoteReview writes a published review with the replies its own threads
// collected — each under what it answers, a finding's under that finding and
// the summary's under the summary. It is one document either way: the JSON form
// carries the replies as fields, the markdown and terminal forms as text inside
// the review.
func (a *app) writeRemoteReview(ctx context.Context, source *remoteSource, row remoteReview, out io.Writer) error {
	threads, err := source.threads(ctx, row.id)
	if err != nil {
		// The review is what was asked for; replies that could not be read are
		// reported as a warning rather than losing the review with them.
		a.warnf("could not read the replies on %s: %v", remoteRequestLabel(row), err)
	}
	withReplies, err := reviewWithReplies(row.result, threads)
	if err != nil {
		return err
	}
	return a.formatReview(out, withReplies)
}

// reviewThread is one of NickPit's own threads on a request: the review or one
// finding, and what was said in reply to it.
type reviewThread struct {
	// reviewID says which review the thread belongs to, and findingID which
	// finding — empty for the thread the summary opened.
	reviewID  string
	findingID string
	replies   []model.Reply
}

// gitlabReviewThreads reads the replies out of a project's discussions. A
// thread counts as NickPit's when its ROOT note carries a review or finding
// marker: replies under someone else's comment are their conversation, not an
// answer to the review. The root itself is the review text the document
// already holds, so only what follows it is kept.
func gitlabReviewThreads(discussions []glscm.MRDiscussion) []reviewThread {
	var threads []reviewThread
	for _, discussion := range discussions {
		if len(discussion.Notes) == 0 {
			continue
		}
		root := discussion.Notes[0]
		reviewID, findingID, ok := reviewmd.DetectThreadReview(root.Body)
		if !ok || reviewID == "" {
			continue
		}
		thread := reviewThread{reviewID: reviewID, findingID: findingID}
		for _, note := range discussion.Notes[1:] {
			if note.System {
				continue
			}
			body := strings.TrimSpace(reviewmd.StripMarkers(note.Body))
			if body == "" {
				continue
			}
			thread.replies = append(thread.replies, model.Reply{
				Author:    replyAuthor(note, root),
				CreatedAt: note.CreatedAt,
				Body:      textsan.StripControl(body),
			})
		}
		if len(thread.replies) > 0 {
			threads = append(threads, thread)
		}
	}
	return threads
}

// replyAuthor is the name a reply is read by. GitLab gives a person their
// display name, but a group or project access token's is masked to "****" and
// its username is a hash — unreadable either way. A reply from the account that
// opened the thread is NickPit answering in its own thread, so it says so.
func replyAuthor(note, root glscm.DiscussionNote) string {
	if name := strings.TrimSpace(note.AuthorDisplay); name != "" && strings.Trim(name, "*") != "" {
		return textsan.StripControl(name)
	}
	if note.AuthorID != 0 && note.AuthorID == root.AuthorID {
		return "NickPit"
	}
	return textsan.StripControl(note.AuthorName)
}

// reviewWithReplies is the review as it is printed and copied: the published
// findings with the answers their own threads collected hanging off them, the
// summary's under the summary. One document, whichever format the run asked
// for — the JSON carries the replies as fields of the review, the markdown and
// terminal forms as the messages under what they answer.
//
// The result is a copy: the one the picker row points at stays as published.
func reviewWithReplies(result *model.ReviewResult, threads []reviewThread) (*model.ReviewResult, error) {
	if len(threads) == 0 {
		return result, nil
	}
	byFinding := map[string][]model.Reply{}
	var summary []model.Reply
	for _, thread := range threads {
		if result.ReviewID != "" && thread.reviewID != result.ReviewID {
			// A request carries one thread set per review it has had; the
			// others answer a review this is not.
			continue
		}
		if thread.findingID == "" {
			summary = append(summary, thread.replies...)
			continue
		}
		byFinding[thread.findingID] = append(byFinding[thread.findingID], thread.replies...)
	}
	if len(summary) == 0 && len(byFinding) == 0 {
		return result, nil
	}
	withReplies, err := result.Clone()
	if err != nil {
		return nil, err
	}
	withReplies.Replies = summary
	for i := range withReplies.Findings {
		withReplies.Findings[i].Replies = byFinding[withReplies.Findings[i].ID]
	}
	return withReplies, nil
}

// remoteRequestLabel names a request the way its platform writes it.
func remoteRequestLabel(row remoteReview) string {
	marker := "!"
	if row.mode == model.ModeGitHub {
		marker = "#"
	}
	return fmt.Sprintf("%s%s%d", row.repo, marker, row.id)
}
