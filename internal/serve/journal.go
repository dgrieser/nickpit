package serve

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
)

// Journal persists accepted-but-unfinished review jobs as one small JSON file
// per merge request, so a restarted daemon (crash, upgrade, reschedule) resumes
// them instead of losing the queue and stranding acknowledged command notes in
// their in-progress state. It is best effort: a write failure degrades to the
// journal-less behavior and is logged, never fatal. All methods are nil-safe —
// a nil *Journal is the disabled state.
//
// The files survive exactly as long as their directory does: point it at
// durable storage (a Kubernetes PVC, a host path) to survive pod replacement;
// on ephemeral storage it still covers plain process restarts.
type Journal struct {
	dir string
	log *slog.Logger
}

// journalEntry is the persisted form of an Event. The group is not stored —
// tokens must never land on disk — but re-resolved from the project path at
// restore time, exactly like the webhook handler routes a delivery.
type journalEntry struct {
	Kind        string `json:"kind"`
	ProjectID   int    `json:"project_id"`
	ProjectPath string `json:"project_path"`
	IID         int    `json:"iid"`
	HeadSHA     string `json:"head_sha,omitempty"`
	AckNoteIDs  []int  `json:"ack_note_ids,omitempty"`
	// StartEmojis and AckEmojis are the configured names under which this job
	// may already have placed managed reactions. They must survive config
	// changes so a resumed job can revoke the old markers.
	StartEmojis []string `json:"start_emojis,omitempty"`
	AckEmojis   []string `json:"ack_emojis,omitempty"`
}

// NewJournal opens (creating if needed) the state directory. An empty dir
// returns a nil journal: journaling disabled.
func NewJournal(dir string, log *slog.Logger) (*Journal, error) {
	if dir == "" {
		return nil, nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("state dir: %w", err)
	}
	return &Journal{dir: dir, log: log}, nil
}

// Dir returns the journal's directory (empty when disabled).
func (j *Journal) Dir() string {
	if j == nil {
		return ""
	}
	return j.dir
}

func (j *Journal) path(projectID, iid int) string {
	return filepath.Join(j.dir, fmt.Sprintf("job-%d-%d.json", projectID, iid))
}

// persist records the event as the job's would-be re-run: what a restarted
// daemon must enqueue so the review still happens and every acknowledged note
// still gets its outcome flip. One file per (project, iid); newer state
// overwrites older via an atomic rename.
func (j *Journal) persist(event Event) bool {
	if j == nil {
		return false
	}
	path := j.path(event.ProjectID, event.IID)
	tmp := path + ".tmp"
	entry := journalEntry{
		Kind:        event.Kind.String(),
		ProjectID:   event.ProjectID,
		ProjectPath: event.ProjectPath,
		IID:         event.IID,
		HeadSHA:     event.HeadSHA,
		AckNoteIDs:  event.AckNoteIDs,
		StartEmojis: event.StartEmojis,
		AckEmojis:   event.AckEmojis,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		j.log.Warn("journal: encoding job failed", "project", event.ProjectPath, "iid", event.IID, "error", err)
		j.invalidate(path, tmp, event.ProjectID, event.IID)
		return false
	}
	err = os.WriteFile(tmp, data, 0o600)
	if err == nil {
		err = os.Rename(tmp, path)
	}
	if err != nil {
		j.log.Warn("journal: persisting job failed", "project", event.ProjectPath, "iid", event.IID, "error", err)
		j.invalidate(path, tmp, event.ProjectID, event.IID)
		return false
	}
	return true
}

// invalidate removes both the uncommitted temporary file and any older
// canonical entry. Once an update fails, the old snapshot is unsafe to resume:
// it may omit a newer manual trigger or acknowledgements already awarded.
func (j *Journal) invalidate(path, tmp string, projectID, iid int) {
	if err := os.Remove(tmp); err != nil && !errors.Is(err, fs.ErrNotExist) {
		j.log.Warn("journal: removing failed temporary job file", "file", tmp, "error", err)
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		j.log.Warn("journal: invalidating stale job failed", "project_id", projectID, "iid", iid, "error", err)
	}
}

// remove deletes the job's file once nothing is left to resume (settled,
// aborted, or dropped on purpose). A missing file is fine.
func (j *Journal) remove(projectID, iid int) {
	if j == nil {
		return
	}
	if err := os.Remove(j.path(projectID, iid)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		j.log.Warn("journal: removing job failed", "project_id", projectID, "iid", iid, "error", err)
	}
}

// load reads every journaled job. Unreadable or malformed files are removed
// (they can never be resumed) and logged.
func (j *Journal) load() []journalEntry {
	if j == nil {
		return nil
	}
	paths, err := filepath.Glob(filepath.Join(j.dir, "job-*.json"))
	if err != nil {
		j.log.Warn("journal: listing state dir failed", "dir", j.dir, "error", err)
		return nil
	}
	entries := make([]journalEntry, 0, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		var entry journalEntry
		if err == nil {
			err = json.Unmarshal(data, &entry)
		}
		if err != nil {
			j.log.Warn("journal: dropping unreadable job file", "file", path, "error", err)
			_ = os.Remove(path)
			continue
		}
		entries = append(entries, entry)
	}
	return entries
}

// parseTriggerKind is the inverse of TriggerKind.String for journal entries.
// Unknown values fall back to auto: it re-passes every opt-in check, so a
// corrupted kind can never grant manual bypasses.
func parseTriggerKind(kind string) TriggerKind {
	if kind == TriggerManual.String() {
		return TriggerManual
	}
	return TriggerAuto
}
