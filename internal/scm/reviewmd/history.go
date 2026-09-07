package reviewmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const historyStart = MarkerOpen + "history:start -->"
const historyEnd = MarkerOpen + "history:end -->"
const historyPrefix = MarkerOpen + "history:"
const HistoryOmittedNotice = "Earlier history entries were omitted to fit this comment."
const NoteMaxBytes = carrierNoteMaxBytes

type HistoryEntry struct {
	At   time.Time `json:"at"`
	Body string    `json:"body"`
}

type CommentHistory struct {
	Entries []HistoryEntry `json:"entries"`
	Omitted bool           `json:"omitted,omitempty"`
}

// StripHistory recognizes only our fenced block, so nested user <details> and
// Suggestions blocks cannot prematurely terminate it.
func StripHistory(body string) string {
	for {
		start := strings.Index(body, historyStart)
		if start < 0 {
			return strings.TrimSpace(body)
		}
		end := strings.Index(body[start:], historyEnd)
		if end < 0 {
			return strings.TrimSpace(body[:start])
		}
		body = body[:start] + body[start+end+len(historyEnd):]
	}
}

func ReadHistory(body string) CommentHistory {
	var history CommentHistory
	start := strings.Index(body, historyStart)
	end := strings.Index(body, historyEnd)
	if start < 0 || end < start {
		return history
	}
	budget := &carrierBudget{}
	scanMarkers(body[start+len(historyStart):end], historyPrefix, func(raw string) bool {
		if !budget.allow() {
			return false
		}
		var h CommentHistory
		n, ok := decodeMarker(raw, &h)
		budget.spend(n)
		if ok {
			history = h
			return false
		}
		return true
	})
	return history
}

// WithHistory stores only the previous current body, never a recursive copy of
// its history. Each entry's hidden metadata is encoded once in the archive;
// only sanitized visible markdown is rendered in the collapsible section.
func WithHistory(previous, current, label string, at time.Time, archive bool) (string, error) {
	history := ReadHistory(previous)
	if archive && strings.TrimSpace(previous) != "" {
		history.Entries = append([]HistoryEntry{{At: at.UTC(), Body: StripHistory(previous)}}, history.Entries...)
	}
	current = StripHistory(current)
	if len(current) > NoteMaxBytes {
		return "", fmt.Errorf("current comment exceeds %d bytes", NoteMaxBytes)
	}
	for {
		block := renderHistory(history, label)
		if block == "" && len(history.Entries) == 0 && !history.Omitted {
			return current, nil
		}
		if (block != "" || (len(history.Entries) == 0 && !history.Omitted)) && len(block)+len(current)+2 <= NoteMaxBytes {
			if block == "" {
				return current, nil
			}
			return block + "\n\n" + current, nil
		}
		if len(history.Entries) == 0 {
			return "", fmt.Errorf("comment leaves no room for history omission notice")
		}
		history.Entries = history.Entries[:len(history.Entries)-1]
		history.Omitted = true
	}
}

func renderHistory(h CommentHistory, label string) string {
	if len(h.Entries) == 0 && !h.Omitted {
		return ""
	}
	marker, _ := encodeMarker(historyPrefix, h)
	if marker == "" {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n<details>\n<summary>:scroll: %s History</summary>\n\n", historyStart, Sanitize(label))
	for _, entry := range h.Entries {
		fmt.Fprintf(&b, "**%s**\n\n%s\n\n---\n\n", entry.At.Format(time.RFC3339), EscapeQuickActions(Sanitize(StripMarkers(entry.Body))))
	}
	if h.Omitted {
		b.WriteString(HistoryOmittedNotice + "\n\n")
	}
	b.WriteString(marker + "\n</details>\n" + historyEnd)
	return b.String()
}

// UpdateRecord is a bounded chunk of a durable pending GitLab update. Activation
// is written only after every chunk exists. Current-state readers reject an
// activated batch until its root revision commits.
type UpdateRecord struct {
	ReviewID  string `json:"rid"`
	Operation string `json:"op"`
	Revision  uint64 `json:"revision"`
	Part      int    `json:"part"`
	Parts     int    `json:"parts"`
	Data      []byte `json:"data,omitempty"`
	Active    bool   `json:"active,omitempty"`
}

const updatePrefix = MarkerOpen + "update:"

type ThreadReference struct {
	ReviewID  string   `json:"rid"`
	FindingID string   `json:"fid"`
	Previous  []string `json:"previous,omitempty"`
	Next      string   `json:"next,omitempty"`
}

func ThreadReferenceMarker(ref ThreadReference) string {
	marker, _ := encodeMarker(MarkerOpen+"thread:", ref)
	return marker
}

func ReadThreadReference(body string) ThreadReference {
	var ref ThreadReference
	budget := &carrierBudget{}
	scanMarkers(StripHistory(body), MarkerOpen+"thread:", func(raw string) bool {
		if !budget.allow() {
			return false
		}
		n, ok := decodeMarker(raw, &ref)
		budget.spend(n)
		return !ok
	})
	return ref
}

func FindingReferenceMarker(reviewID, findingID string) string {
	return findingRefMarker(reviewID, findingID)
}

func UpdateRecordMarker(record UpdateRecord) (string, error) {
	marker, _ := encodeMarker(updatePrefix, record)
	if marker == "" || len(marker) > NoteMaxBytes {
		return "", fmt.Errorf("update record exceeds carrier budget")
	}
	return marker, nil
}

func CollectUpdateRecords(body string) []UpdateRecord {
	var out []UpdateRecord
	budget := &carrierBudget{}
	scanMarkers(StripHistory(body), updatePrefix, func(raw string) bool {
		if !budget.allow() {
			return false
		}
		var record UpdateRecord
		n, ok := decodeMarker(raw, &record)
		budget.spend(n)
		if ok {
			out = append(out, record)
		}
		return true
	})
	return out
}

// Select one highest revision of each live carrier before legacy reassembly.
// Archived snapshots are never candidates, and a routing reference cannot win.
func currentCarrierBodies(bodies []string) []string {
	reviews := map[string]ReviewEnvelope{}
	findings := map[string]FindingEnvelope{}
	var findingOrder []string
	var reviewOrder []string
	var pending []UpdateRecord
	for _, body := range bodies {
		pending = append(pending, CollectUpdateRecords(body)...)
		for _, env := range CollectReviewEnvelopes(body) {
			if env.Ref || env.ReviewID == "" {
				continue
			}
			prior, ok := reviews[env.ReviewID]
			if !ok {
				reviewOrder = append(reviewOrder, env.ReviewID)
			}
			if !ok || env.Revision >= prior.Revision {
				reviews[env.ReviewID] = env
			}
		}
		for _, env := range CollectFindingEnvelopes(body) {
			if env.Ref || env.ReviewID == "" {
				continue
			}
			key := env.ReviewID + "\x00" + env.Finding.ID
			if env.Finding.ID == "" {
				raw, _ := json.Marshal(env.Finding)
				key += string(raw)
			}
			prior, ok := findings[key]
			if !ok {
				findingOrder = append(findingOrder, key)
			}
			if !ok || env.Finding.Revision > prior.Finding.Revision {
				findings[key] = env
			}
		}
	}
	for _, record := range pending {
		if record.Active && reviews[record.ReviewID].Revision < record.Revision {
			delete(reviews, record.ReviewID)
		}
	}
	var out []string
	for _, id := range reviewOrder {
		if env, ok := reviews[id]; ok {
			marker, _ := encodeMarker(ReviewMarkerPrefix, env)
			out = append(out, marker)
		}
	}
	for _, key := range findingOrder {
		marker, _ := encodeMarker(FindingMarkerPrefix, findings[key])
		out = append(out, marker)
	}
	return out
}
