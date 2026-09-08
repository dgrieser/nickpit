package gitlab

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/scm/reviewmd"
	"github.com/google/uuid"
)

type ReviewUpdateRequest struct {
	Operation        string
	Before, After    *model.ReviewResult
	BaseSHA, HeadSHA string
	// Validate rechecks the initiating note and response policy inside the
	// publishing critical section, after all model work has completed.
	Validate func(context.Context) error
}

type updateTarget struct {
	DiscussionID string `json:"discussion_id"`
	NoteID       int    `json:"note_id"`
	Body         string `json:"body"`
}

type updateItem struct {
	Target    updateTarget  `json:"target"`
	Body      string        `json:"body"`
	Marker    string        `json:"marker"`
	Position  *position     `json:"position,omitempty"`
	Redirect  *updateTarget `json:"redirect,omitempty"`
	FindingID string        `json:"finding_id,omitempty"`
}

type reviewUpdateTransaction struct {
	ReviewID  string       `json:"rid"`
	Operation string       `json:"operation"`
	Revision  uint64       `json:"revision"`
	At        time.Time    `json:"at"`
	Items     []updateItem `json:"items"`
}

// UpdateReview stages a validated batch durably before touching visible posts.
// RecoverReviewUpdates resumes exactly these writes if the process or API fails.
func (a *Adapter) UpdateReview(ctx context.Context, project string, iid int, req ReviewUpdateRequest) (*model.ReviewResult, error) {
	if req.Before == nil || req.After == nil || req.Before.ReviewID == "" || req.Before.ReviewID != req.After.ReviewID {
		return nil, fmt.Errorf("update: invalid review identity")
	}
	ctx, unlock, err := a.client.LockMR(ctx, project, iid)
	if err != nil {
		return nil, err
	}
	defer unlock()
	if err := a.RecoverReviewUpdates(ctx, project, iid); err != nil {
		return nil, err
	}
	user, err := a.client.CurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	discussions, err := a.client.MRDiscussions(ctx, project, iid)
	if err != nil {
		return nil, err
	}
	current := reviewmd.ReviewResultsByID(ownedBodies(discussions, user.ID))[req.Before.ReviewID]
	if current == nil || reviewStateHash(current) != reviewStateHash(req.Before) {
		return nil, fmt.Errorf("update: review changed during evaluation; retry with fresh context")
	}
	info, err := a.client.FetchMRPositionInfo(ctx, project, iid)
	if err != nil {
		return nil, err
	}
	if req.HeadSHA == "" || info.DiffRefs.HeadSHA != req.HeadSHA || (req.BaseSHA != "" && info.DiffRefs.BaseSHA != req.BaseSHA) {
		return nil, fmt.Errorf("update: MR diff changed during evaluation; retry with fresh context")
	}
	if req.Validate != nil {
		if err := req.Validate(ctx); err != nil {
			return nil, err
		}
	}
	after, err := req.After.Clone()
	if err != nil {
		return nil, err
	}
	if len(after.Findings) != len(current.Findings) {
		return nil, fmt.Errorf("update cannot add or remove finding identities")
	}
	beforeByID := map[string]model.Finding{}
	for _, f := range current.Findings {
		beforeByID[f.ID] = f
	}
	var changed []model.Finding
	after.Revision = current.Revision + 1
	for i := range after.Findings {
		f := &after.Findings[i]
		before, ok := beforeByID[f.ID]
		if !ok {
			return nil, fmt.Errorf("update contains unknown or repeated finding %s", f.ID)
		}
		delete(beforeByID, f.ID)
		if !reflect.DeepEqual(before, *f) {
			if before.Resolution != nil {
				return nil, fmt.Errorf("resolved finding %s cannot change", f.ID)
			}
			f.Revision = after.Revision
			changed = append(changed, *f)
		}
	}
	rootChanged := current.OverallCorrectness != after.OverallCorrectness || current.OverallExplanation != after.OverallExplanation || current.OverallConfidenceScore != after.OverallConfidenceScore
	if len(changed) == 0 && !rootChanged {
		return current, nil
	}
	operation := req.Operation
	if operation == "" {
		operation = uuid.NewString()
	}
	if strings.IndexFunc(operation, func(r rune) bool { return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') }) >= 0 {
		return nil, fmt.Errorf("invalid update operation ID")
	}
	transaction := reviewUpdateTransaction{ReviewID: current.ReviewID, Operation: operation, Revision: after.Revision, At: time.Now().UTC()}
	render := a.render.ForReview(after.ReviewID).WithContextOptions(after.ContextOptions)
	for _, finding := range changed {
		old := findUpdateTarget(discussions, user.ID, after.ReviewID, finding.ID)
		previous := old.Body
		var original model.Finding
		for _, f := range current.Findings {
			if f.ID == finding.ID {
				original = f
			}
		}
		if previous == "" {
			previous, _ = render.FindingBodyCarried(original, locationPrefix(original))
		}
		body, carried := render.FindingBodyCarried(finding, locationPrefix(finding))
		if !carried {
			return nil, fmt.Errorf("updated finding %s exceeds current carrier budget", finding.ID)
		}
		item := updateItem{Target: old, FindingID: finding.ID}
		moved := finding.Resolution == nil && (original.CodeLocation.FilePath != finding.CodeLocation.FilePath || original.CodeLocation.LineRange != finding.CodeLocation.LineRange)
		ref := reviewmd.ReadThreadReference(old.Body)
		if moved && old.NoteID != 0 {
			copy := old
			item.Redirect = &copy
			item.Target = updateTarget{}
			ref.Previous = append(ref.Previous, old.DiscussionID)
			body += "\n\n[Previous discussion](" + a.client.noteURL(project, iid, old.NoteID) + ")."
		}
		if len(ref.Previous) > 0 {
			body += "\n\n" + reviewmd.ThreadReferenceMarker(reviewmd.ThreadReference{ReviewID: after.ReviewID, FindingID: finding.ID, Previous: ref.Previous})
		}
		if item.Target.NoteID == 0 && finding.Resolution == nil {
			for _, change := range info.Changes {
				if change.NewPath == finding.CodeLocation.FilePath {
					if pos, ok := multiLinePosition(change, info.DiffRefs, finding.CodeLocation.LineRange); ok {
						item.Position = &pos
					} else if pos, ok := bestPosition(change, info.DiffRefs, finding.CodeLocation.LineRange); ok {
						item.Position = &pos
					}
				}
			}
		}
		item.Marker = updateItemMarker(transaction.Operation, len(transaction.Items))
		body += "\n\n" + item.Marker
		body = reviewmd.TransferResponseFooter(previous, body)
		item.Body, err = reviewmd.WithHistory(previous, body, "Finding", transaction.At, true)
		if err != nil {
			return nil, err
		}
		transaction.Items = append(transaction.Items, item)
	}
	root := findUpdateTarget(discussions, user.ID, after.ReviewID, "")
	previous := root.Body
	if previous == "" {
		previous, _ = render.SummaryBodyCarried(current)
	}
	body, carried := render.SummaryBodyCarried(after)
	if !carried {
		return nil, fmt.Errorf("updated root exceeds current carrier budget")
	}
	marker := updateItemMarker(transaction.Operation, len(transaction.Items))
	body = reviewmd.TransferResponseFooter(previous, body+"\n\n"+marker)
	body, err = reviewmd.WithHistory(previous, body, "Review", transaction.At, rootChanged || req.Operation != "")
	if err != nil {
		return nil, err
	}
	transaction.Items = append(transaction.Items, updateItem{Target: root, Body: body, Marker: marker})
	if err := a.stageReviewUpdate(ctx, project, iid, transaction); err != nil {
		return nil, err
	}
	if err := a.applyReviewUpdate(ctx, project, iid, user.ID, transaction); err != nil {
		return nil, err
	}
	a.cleanupUpdateRecords(ctx, project, iid, user.ID, transaction.Operation)
	return after, nil
}

// ReviewUpdateCommitted recognizes a durable job's root commit across crashes.
func (a *Adapter) ReviewUpdateCommitted(ctx context.Context, project string, iid int, reviewID, operation string) (bool, error) {
	user, err := a.client.CurrentUser(ctx)
	if err != nil {
		return false, err
	}
	discussions, err := a.client.MRDiscussions(ctx, project, iid)
	if err != nil {
		return false, err
	}
	for _, d := range discussions {
		if len(d.Notes) == 0 || d.Notes[0].AuthorID != user.ID {
			continue
		}
		body := d.Notes[0].Body
		rid, fid, ok := reviewmd.DetectThreadReview(body)
		if !ok || rid != reviewID || fid != "" {
			continue
		}
		// Archives are inspected only by recovery code, never by agents.
		bodies := []string{reviewmd.StripHistory(body)}
		for _, entry := range reviewmd.ReadHistory(body).Entries {
			bodies = append(bodies, entry.Body)
		}
		for _, snapshot := range bodies {
			if strings.Contains(snapshot, "<!-- nickpit:update-item:"+operation+":") {
				return true, nil
			}
		}
	}
	return false, nil
}

func locationPrefix(f model.Finding) string {
	return fmt.Sprintf("`%s:%d`", reviewmd.Sanitize(f.CodeLocation.FilePath), f.CodeLocation.LineRange.Start)
}

func updateItemMarker(operation string, i int) string {
	return fmt.Sprintf("<!-- nickpit:update-item:%s:%d -->", operation, i)
}

func ownedBodies(discussions []MRDiscussion, userID int) []string {
	var bodies []string
	for _, d := range discussions {
		for _, n := range d.Notes {
			if n.AuthorID == userID {
				bodies = append(bodies, n.Body)
			}
		}
	}
	return bodies
}

func findUpdateTarget(discussions []MRDiscussion, userID int, rid, fid string) updateTarget {
	var out updateTarget
	var revision uint64
	for _, d := range discussions {
		if len(d.Notes) == 0 {
			continue
		}
		n := d.Notes[0]
		if n.AuthorID != userID || reviewmd.StripMarkers(n.Body) == "" || reviewmd.ReadThreadReference(n.Body).Next != "" {
			continue
		}
		r, f, ok := reviewmd.DetectThreadReview(n.Body)
		if !ok || r != rid || f != fid {
			continue
		}
		var rev uint64
		if fid == "" {
			for _, e := range reviewmd.CollectReviewEnvelopes(n.Body) {
				rev = max(rev, e.Revision)
			}
		} else {
			for _, e := range reviewmd.CollectFindingEnvelopes(n.Body) {
				rev = max(rev, e.Finding.Revision)
			}
		}
		if out.NoteID == 0 || rev > revision {
			out = updateTarget{DiscussionID: d.ID, NoteID: n.ID, Body: n.Body}
			revision = rev
		}
	}
	return out
}

func reviewStateHash(r *model.ReviewResult) [32]byte {
	copy, _ := r.Clone()
	sort.Slice(copy.Findings, func(i, j int) bool { return copy.Findings[i].ID < copy.Findings[j].ID })
	// Only fields reconstructed from carriers participate; local telemetry does not.
	raw, _ := json.Marshal(struct {
		Revision                 uint64
		Findings                 []model.Finding
		Correctness, Explanation string
		Confidence               float64
		Context                  *model.ContextOptions
	}{copy.Revision, copy.Findings, copy.OverallCorrectness, copy.OverallExplanation, copy.OverallConfidenceScore, copy.ContextOptions})
	return sha256.Sum256(raw)
}

func (a *Adapter) stageReviewUpdate(ctx context.Context, project string, iid int, transaction reviewUpdateTransaction) error {
	raw, err := json.Marshal(transaction)
	if err != nil {
		return err
	}
	if len(raw) > 8<<20 {
		return fmt.Errorf("update batch exceeds 8 MiB recovery budget")
	}
	const chunk = 24_000
	parts := (len(raw) + chunk - 1) / chunk
	for part := 0; part < parts; part++ {
		record := reviewmd.UpdateRecord{ReviewID: transaction.ReviewID, Operation: transaction.Operation, Revision: transaction.Revision, Part: part, Parts: parts, Data: raw[part*chunk : min((part+1)*chunk, len(raw))]}
		marker, err := reviewmd.UpdateRecordMarker(record)
		if err != nil {
			return err
		}
		if err := a.client.CreateMRNotePath(ctx, project, iid, marker); err != nil {
			return err
		}
	}
	marker, err := reviewmd.UpdateRecordMarker(reviewmd.UpdateRecord{ReviewID: transaction.ReviewID, Operation: transaction.Operation, Revision: transaction.Revision, Parts: parts, Active: true})
	if err != nil {
		return err
	}
	return a.client.CreateMRNotePath(ctx, project, iid, marker)
}

// RecoverReviewUpdates completes activated transactions. Unactivated fragments
// cannot have changed visible posts and do not suppress normal reassembly.
func (a *Adapter) RecoverReviewUpdates(ctx context.Context, project string, iid int) error {
	ctx, unlock, err := a.client.LockMR(ctx, project, iid)
	if err != nil {
		return err
	}
	defer unlock()
	user, err := a.client.CurrentUser(ctx)
	if err != nil {
		return err
	}
	discussions, err := a.client.MRDiscussions(ctx, project, iid)
	if err != nil {
		return err
	}
	groups := map[string][]reviewmd.UpdateRecord{}
	for _, body := range ownedBodies(discussions, user.ID) {
		for _, record := range reviewmd.CollectUpdateRecords(body) {
			groups[record.Operation] = append(groups[record.Operation], record)
		}
	}
	var operations []string
	for op := range groups {
		operations = append(operations, op)
	}
	sort.Strings(operations)
	for _, op := range operations {
		records := groups[op]
		var active *reviewmd.UpdateRecord
		for i := range records {
			if records[i].Active {
				active = &records[i]
			}
		}
		if active == nil {
			a.cleanupUpdateRecords(ctx, project, iid, user.ID, op)
			continue
		}
		committed := false
		for _, body := range ownedBodies(discussions, user.ID) {
			for _, env := range reviewmd.CollectReviewEnvelopes(body) {
				if !env.Ref && env.ReviewID == active.ReviewID && env.Revision >= active.Revision {
					committed = true
				}
			}
		}
		if committed {
			a.cleanupUpdateRecords(ctx, project, iid, user.ID, op)
			continue
		}
		if active.Parts <= 0 || active.Parts > 350 {
			return fmt.Errorf("invalid pending update chunk count")
		}
		chunks := make([][]byte, active.Parts)
		for _, r := range records {
			if !r.Active && r.ReviewID == active.ReviewID && r.Revision == active.Revision && r.Parts == active.Parts && r.Part >= 0 && r.Part < active.Parts {
				chunks[r.Part] = r.Data
			}
		}
		var raw []byte
		for _, chunk := range chunks {
			if len(chunk) == 0 {
				return fmt.Errorf("pending update %s is incomplete", op)
			}
			raw = append(raw, chunk...)
			if len(raw) > 8<<20 {
				return fmt.Errorf("pending update exceeds recovery budget")
			}
		}
		var transaction reviewUpdateTransaction
		if err := json.Unmarshal(raw, &transaction); err != nil {
			return err
		}
		if transaction.Operation != op || transaction.ReviewID != active.ReviewID || transaction.Revision != active.Revision {
			return fmt.Errorf("pending update identity mismatch")
		}
		if err := a.applyReviewUpdate(ctx, project, iid, user.ID, transaction); err != nil {
			return err
		}
		a.cleanupUpdateRecords(ctx, project, iid, user.ID, op)
	}
	return nil
}

func (a *Adapter) applyReviewUpdate(ctx context.Context, project string, iid, userID int, transaction reviewUpdateTransaction) error {
	for _, item := range transaction.Items {
		target, err := a.applyUpdateItem(ctx, project, iid, userID, item)
		if err != nil {
			return err
		}
		if item.Redirect != nil {
			link := a.client.noteURL(project, iid, target.NoteID)
			body := reviewmd.FindingReferenceMarker(transaction.ReviewID, item.FindingID) + "\n\nFinding moved to the [updated discussion](" + link + ").\n\n" + reviewmd.ThreadReferenceMarker(reviewmd.ThreadReference{ReviewID: transaction.ReviewID, FindingID: item.FindingID, Next: target.DiscussionID})
			marker := item.Marker + "\n<!-- nickpit:redirect -->"
			body = reviewmd.TransferResponseFooter(item.Redirect.Body, body+"\n\n"+marker)
			body, err = reviewmd.WithHistory(item.Redirect.Body, body, "Finding", transaction.At, true)
			if err != nil {
				return err
			}
			_, err = a.applyUpdateItem(ctx, project, iid, userID, updateItem{Target: *item.Redirect, Body: body, Marker: marker})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (a *Adapter) applyUpdateItem(ctx context.Context, project string, iid, userID int, item updateItem) (updateTarget, error) {
	discussions, err := a.client.MRDiscussions(ctx, project, iid)
	if err != nil {
		return updateTarget{}, err
	}
	for _, d := range discussions {
		for _, n := range d.Notes {
			if n.AuthorID != userID {
				continue
			}
			matches := item.Target.NoteID != 0 && n.ID == item.Target.NoteID && d.ID == item.Target.DiscussionID
			if item.Target.NoteID == 0 {
				matches = strings.Contains(reviewmd.StripHistory(n.Body), item.Marker)
			}
			if !matches {
				continue
			}
			body := reviewmd.TransferResponseFooter(n.Body, item.Body)
			if body != n.Body {
				if reviewmd.StripResponseFooter(n.Body) != reviewmd.StripResponseFooter(item.Target.Body) {
					return updateTarget{}, fmt.Errorf("update target %d changed outside this operation", n.ID)
				}
				if len(body) > reviewmd.NoteMaxBytes {
					return updateTarget{}, fmt.Errorf("updated note exceeds size budget")
				}
				if err := a.client.UpdateMRDiscussionNote(ctx, project, iid, d.ID, n.ID, body); err != nil {
					return updateTarget{}, err
				}
			}
			return updateTarget{DiscussionID: d.ID, NoteID: n.ID, Body: body}, nil
		}
	}
	if item.Target.NoteID != 0 {
		return updateTarget{}, fmt.Errorf("update target note %d disappeared", item.Target.NoteID)
	}
	payload := map[string]any{"body": item.Body}
	if item.Position != nil {
		payload["position"] = item.Position
	}
	var created struct {
		ID    string `json:"id"`
		Notes []struct {
			ID int `json:"id"`
		} `json:"notes"`
	}
	endpoint := fmt.Sprintf("/projects/%s/merge_requests/%d/discussions", escapeProject(project), iid)
	err = a.client.Post(ctx, endpoint, payload, &created)
	if isUnprocessable(err) && item.Position != nil && item.Position.LineRange != nil {
		pos := *item.Position
		pos.LineRange = nil
		payload["position"] = &pos
		err = a.client.Post(ctx, endpoint, payload, &created)
	}
	if isUnprocessable(err) && item.Position != nil {
		delete(payload, "position")
		err = a.client.Post(ctx, endpoint, payload, &created)
	}
	if err != nil {
		return updateTarget{}, err
	}
	if created.ID == "" || len(created.Notes) == 0 {
		return updateTarget{}, fmt.Errorf("GitLab returned no created discussion identity")
	}
	return updateTarget{DiscussionID: created.ID, NoteID: created.Notes[0].ID, Body: item.Body}, nil
}

func (a *Adapter) cleanupUpdateRecords(ctx context.Context, project string, iid, userID int, operation string) {
	discussions, err := a.client.MRDiscussions(ctx, project, iid)
	if err != nil {
		return
	}
	for _, d := range discussions {
		for _, n := range d.Notes {
			if n.AuthorID != userID || reviewmd.StripMarkers(n.Body) != "" {
				continue
			}
			for _, r := range reviewmd.CollectUpdateRecords(n.Body) {
				if r.Operation == operation {
					_ = a.client.Delete(ctx, fmt.Sprintf("/projects/%s/merge_requests/%d/notes/%d", escapeProject(project), iid, n.ID))
					break
				}
			}
		}
	}
}

func (c *Client) noteURL(project string, iid, noteID int) string {
	base := strings.TrimSuffix(c.baseURL, "/api/v4")
	parts := strings.Split(project, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return fmt.Sprintf("%s/%s/-/merge_requests/%d#note_%d", base, strings.Join(parts, "/"), iid, noteID)
}
