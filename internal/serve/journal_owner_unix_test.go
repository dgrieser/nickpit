//go:build unix

package serve

import (
	"encoding/json"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestValidateJournalDirRejectsDifferentOwner(t *testing.T) {
	dir := privateJournalDir(t)
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		t.Skip("file ownership is unavailable")
	}
	otherUID := stat.Uid + 1
	if otherUID == stat.Uid {
		otherUID--
	}
	if err := validateJournalDirOwnerAs(info, otherUID); err == nil {
		t.Fatal("owner mismatch was accepted")
	}
}

func TestJournalPinsValidatedDirectoryAcrossPathReplacement(t *testing.T) {
	parent := privateJournalDir(t)
	dir := filepath.Join(parent, "state")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	journal, err := NewJournal(dir, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = journal.Close() })

	pinnedDir := filepath.Join(parent, "pinned-state")
	if err := os.Rename(dir, pinnedDir); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	attacker := journalEntry{
		Kind:        TriggerManual.String(),
		ProjectID:   666,
		ProjectPath: "attacker/project",
		IID:         9,
	}
	data, err := json.Marshal(attacker)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "job-666-9.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}

	trusted := Event{Kind: TriggerAuto, ProjectID: 42, ProjectPath: "platform/api", IID: 7, HeadSHA: "sha-1"}
	if !journal.persist(trusted) {
		t.Fatal("persist through pinned directory failed")
	}
	entries := journal.load()
	if len(entries) != 1 || entries[0].ProjectID != trusted.ProjectID || entries[0].IID != trusted.IID {
		t.Fatalf("entries after path replacement = %+v, want only trusted pinned-directory job", entries)
	}
	if _, err := os.Stat(filepath.Join(pinnedDir, journal.name(trusted.ProjectID, trusted.IID))); err != nil {
		t.Fatalf("trusted job missing from pinned directory: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, journal.name(trusted.ProjectID, trusted.IID))); !os.IsNotExist(err) {
		t.Fatalf("trusted job written through replacement path: %v", err)
	}
}
