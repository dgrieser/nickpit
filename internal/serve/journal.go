package serve

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
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
	Kind                       string `json:"kind"`
	ProjectID                  int    `json:"project_id"`
	ProjectPath                string `json:"project_path"`
	IID                        int    `json:"iid"`
	HeadSHA                    string `json:"head_sha,omitempty"`
	AckNoteIDs                 []int  `json:"ack_note_ids,omitempty"`
	UncertainAckNoteIDs        []int  `json:"uncertain_ack_note_ids,omitempty"`
	AckCleanupUntilUnixMilli   int64  `json:"ack_cleanup_until_unix_milli,omitempty"`
	StartCleanupUntilUnixMilli int64  `json:"start_cleanup_until_unix_milli,omitempty"`
	// Reaction names and MR settlement mode must survive config changes so a
	// resumed job revokes old markers without changing revoke-only work into an
	// outcome award.
	StartEmojis  []string `json:"start_emojis,omitempty"`
	AckEmojis    []string `json:"ack_emojis,omitempty"`
	SettleMR     bool     `json:"settle_mr,omitempty"`
	RevokeMROnly bool     `json:"revoke_mr_only,omitempty"`
	// CleanupOutcome marks an entry that must only settle reactions, never
	// rerun the review. Pending carries a review accepted while that cleanup
	// was retrying. Aborted carries acknowledgements from a pending review that
	// was cancelled behind cleanup; those must be revoked without an outcome.
	CleanupOutcome string        `json:"cleanup_outcome,omitempty"`
	Pending        *journalEntry `json:"pending,omitempty"`
	Aborted        *journalEntry `json:"aborted,omitempty"`
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
	if err := validateJournalDir(dir); err != nil {
		return nil, fmt.Errorf("state dir: %w", err)
	}
	if err := probeJournalDir(dir); err != nil {
		return nil, fmt.Errorf("state dir: %w", err)
	}
	return &Journal{dir: dir, log: log}, nil
}

// validateJournalDir keeps untrusted local users from replacing predictable
// job paths and injecting work for the daemon. Read-only access is harmless:
// journal files themselves are mode 0600 and contain no credentials.
func validateJournalDir(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("inspect permissions: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("not a directory")
	}
	if info.Mode().Perm()&0o022 != 0 {
		return fmt.Errorf("permissions %04o allow group or world writes", info.Mode().Perm())
	}
	return nil
}

// probeJournalDir verifies the operations persistence depends on before intake
// starts. MkdirAll alone succeeds for an existing read-only directory.
func probeJournalDir(dir string) error {
	probe, err := os.CreateTemp(dir, ".nickpit-journal-probe-*.tmp")
	if err != nil {
		return fmt.Errorf("write probe: %w", err)
	}
	tmp := probe.Name()
	committed := tmp + ".ready"
	defer func() { _ = os.Remove(tmp) }()
	defer func() { _ = os.Remove(committed) }()
	if _, err := probe.Write([]byte("ok")); err != nil {
		_ = probe.Close()
		return fmt.Errorf("write probe: %w", err)
	}
	if err := probe.Sync(); err != nil {
		_ = probe.Close()
		return fmt.Errorf("sync probe: %w", err)
	}
	if err := probe.Close(); err != nil {
		return fmt.Errorf("close probe: %w", err)
	}
	if err := os.Rename(tmp, committed); err != nil {
		return fmt.Errorf("rename probe: %w", err)
	}
	if err := syncJournalDir(dir); err != nil {
		return fmt.Errorf("sync directory: %w", err)
	}
	if err := os.Remove(committed); err != nil {
		return fmt.Errorf("remove probe: %w", err)
	}
	return nil
}

func syncJournalDir(dir string) error {
	directory, err := os.Open(dir)
	if err != nil {
		return err
	}
	err = directory.Sync()
	if closeErr := directory.Close(); err == nil {
		err = closeErr
	}
	return err
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
	return j.persistEntry(entryFromEvent(event))
}

// persistCleanup records remote reaction work that must survive until every
// replacement succeeds. A pending review remains behind that cleanup and is
// promoted only after the old reactions have settled.
func (j *Journal) persistCleanup(cleanup reactionCleanup, pending *Event) bool {
	entry := entryFromEvent(cleanup.event)
	entry.CleanupOutcome = cleanupOutcomeName(cleanup.outcome)
	if cleanup.aborted != nil {
		abortedEntry := entryFromEvent(*cleanup.aborted)
		entry.Aborted = &abortedEntry
	}
	if pending != nil {
		pendingEntry := entryFromEvent(*pending)
		entry.Pending = &pendingEntry
	}
	return j.persistEntry(entry)
}

func entryFromEvent(event Event) journalEntry {
	entry := journalEntry{
		Kind:                event.Kind.String(),
		ProjectID:           event.ProjectID,
		ProjectPath:         event.ProjectPath,
		IID:                 event.IID,
		HeadSHA:             event.HeadSHA,
		AckNoteIDs:          event.AckNoteIDs,
		UncertainAckNoteIDs: event.UncertainAckNoteIDs,
		StartEmojis:         event.StartEmojis,
		AckEmojis:           event.AckEmojis,
		SettleMR:            event.SettleMR,
		RevokeMROnly:        event.RevokeMROnly,
	}
	if !event.AckCleanupUntil.IsZero() {
		entry.AckCleanupUntilUnixMilli = event.AckCleanupUntil.UnixMilli()
	}
	if !event.StartCleanupUntil.IsZero() {
		entry.StartCleanupUntilUnixMilli = event.StartCleanupUntil.UnixMilli()
	}
	return entry
}

func (j *Journal) persistEntry(entry journalEntry) bool {
	if j == nil {
		return false
	}
	path := j.path(entry.ProjectID, entry.IID)
	data, err := json.Marshal(entry)
	if err != nil {
		j.log.Warn("journal: encoding job failed", "project", entry.ProjectPath, "iid", entry.IID, "error", err)
		j.invalidate(path, "", entry.ProjectID, entry.IID)
		return false
	}

	// CreateTemp uses O_EXCL and a randomized name. A writable ancestor cannot
	// pre-place a symlink that makes this write follow an attacker-chosen path.
	tmpFile, err := os.CreateTemp(j.dir, "."+filepath.Base(path)+"-*.tmp")
	tmp := ""
	if err == nil {
		tmp = tmpFile.Name()
		_, err = tmpFile.Write(data)
		if err == nil {
			err = tmpFile.Sync()
		}
		if closeErr := tmpFile.Close(); err == nil {
			err = closeErr
		}
	}
	if err == nil {
		err = os.Rename(tmp, path)
	}
	if err == nil {
		err = syncJournalDir(j.dir)
	}
	if err != nil {
		j.log.Warn("journal: persisting job failed", "project", entry.ProjectPath, "iid", entry.IID, "error", err)
		j.invalidate(path, tmp, entry.ProjectID, entry.IID)
		return false
	}
	return true
}

func cleanupOutcomeName(outcome reviewOutcome) string {
	switch outcome {
	case outcomeDone:
		return "done"
	case outcomeFailed:
		return "failed"
	case outcomeAborted:
		return "aborted"
	default:
		return ""
	}
}

func parseCleanupOutcome(name string) (reviewOutcome, bool) {
	switch name {
	case "done":
		return outcomeDone, true
	case "failed":
		return outcomeFailed, true
	case "aborted":
		return outcomeAborted, true
	default:
		return outcomeAborted, false
	}
}

// invalidate removes both the uncommitted temporary file and any older
// canonical entry. Once an update fails, the old snapshot is unsafe to resume:
// it may omit a newer manual trigger or acknowledgements already awarded.
func (j *Journal) invalidate(path, tmp string, projectID, iid int) {
	if tmp != "" {
		if err := os.Remove(tmp); err != nil && !errors.Is(err, fs.ErrNotExist) {
			j.log.Warn("journal: removing failed temporary job file", "file", tmp, "error", err)
		}
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

// load reads every journaled job. Read failures leave the file for a later
// restart; files successfully read and confirmed malformed are removed.
func (j *Journal) load() []journalEntry {
	if j == nil {
		return nil
	}
	files, err := os.ReadDir(j.dir)
	if err != nil {
		j.log.Warn("journal: listing state dir failed", "dir", j.dir, "error", err)
		return nil
	}
	entries := make([]journalEntry, 0, len(files))
	for _, file := range files {
		name := file.Name()
		if file.IsDir() || !strings.HasPrefix(name, "job-") || !strings.HasSuffix(name, ".json") {
			continue
		}
		path := filepath.Join(j.dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			j.log.Warn("journal: reading job file failed; preserving for retry", "file", path, "error", err)
			continue
		}
		var entry journalEntry
		if err := json.Unmarshal(data, &entry); err != nil {
			j.log.Warn("journal: dropping malformed job file", "file", path, "error", err)
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
