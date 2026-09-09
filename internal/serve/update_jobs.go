package serve

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/dgrieser/nickpit/internal/model"
)

// UpdateJob contains no credentials or checkout paths. Workers resolve current
// credentials/config and rebuild context. A prepared publication is checkpointed
// before SCM writes, so recovery never needs to replay model decisions.
type UpdateJob struct {
	ID           string             `json:"id"`
	ProjectPath  string             `json:"project_path"`
	BaseURL      string             `json:"base_url"`
	IID          int                `json:"iid"`
	ReviewID     string             `json:"review_id"`
	DiscussionID string             `json:"discussion_id"`
	NoteID       int                `json:"note_id"`
	Requested    bool               `json:"requested"`
	FindingIDs   []string           `json:"finding_ids"`
	Reason       string             `json:"reason"`
	Question     string             `json:"question"`
	Evidence     string             `json:"evidence,omitempty"`
	Created      time.Time          `json:"created"`
	Attempts     int                `json:"attempts"`
	NextAttempt  time.Time          `json:"next_attempt"`
	Done         bool               `json:"done"`
	Followup     string             `json:"followup,omitempty"`
	Plan         *UpdatePublication `json:"plan,omitempty"`
}

type UpdatePublication struct {
	Evidence string              `json:"evidence"`
	Before   *model.ReviewResult `json:"before"`
	After    *model.ReviewResult `json:"after"`
	HeadSHA  string              `json:"head_sha"`
	BaseSHA  string              `json:"base_sha"`
	Followup string              `json:"followup"`
	Staged   bool                `json:"staged,omitempty"`
}

func (j *UpdateJob) SetID() {
	// One accepted correction job per original question. Re-delivery and a
	// model retry cannot multiply jobs or broaden the first accepted signal.
	j.ID = fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%d\x00%s\x00%d", j.BaseURL, j.ProjectPath, j.IID, j.DiscussionID, j.NoteID))))
}

// UpdateStore uses the journal's pinned, owner-checked root and atomic fsynced
// writes, but deliberately never degrades to best-effort persistence. Callers
// serialize read/modify/write operations with a process-safe per-job lock.
type UpdateStore struct{ journal *Journal }

func NewUpdateStore(dir string) (*UpdateStore, error) {
	if dir == "" {
		return nil, fmt.Errorf("durable update jobs require a state directory")
	}
	j, err := NewJournal(dir, slog.Default())
	if err != nil {
		return nil, err
	}
	return &UpdateStore{journal: j}, nil
}

func (s *UpdateStore) Close() error { return s.journal.Close() }

func updateJobName(id string) (string, error) {
	if len(id) != 64 || strings.IndexFunc(id, func(r rune) bool { return (r < '0' || r > '9') && (r < 'a' || r > 'f') }) >= 0 {
		return "", fmt.Errorf("invalid update job ID")
	}
	return "update-" + id + ".json", nil
}

func (s *UpdateStore) Save(job *UpdateJob) error {
	name, err := updateJobName(job.ID)
	if err != nil {
		return err
	}
	data, err := json.Marshal(job)
	if err != nil {
		return err
	}
	if len(data) > 32<<20 {
		return fmt.Errorf("update job exceeds 32 MiB checkpoint budget")
	}
	j := s.journal
	j.mu.Lock()
	defer j.mu.Unlock()
	tmp, file, err := j.createTemp("." + name)
	if err != nil {
		return err
	}
	defer func() { _ = j.root.Remove(tmp) }()
	_, err = file.Write(data)
	if err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = j.root.Rename(tmp, name)
	}
	if err == nil {
		err = j.sync()
	}
	return err // Never delete an earlier durable checkpoint on failure.
}

func (s *UpdateStore) Load(id string) (*UpdateJob, error) {
	name, err := updateJobName(id)
	if err != nil {
		return nil, err
	}
	f, err := s.journal.root.Open(name)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 32<<20 {
		return nil, fmt.Errorf("invalid update job file")
	}
	data, err := io.ReadAll(io.LimitReader(f, (32<<20)+1))
	if err != nil {
		return nil, err
	}
	var job UpdateJob
	if err := json.Unmarshal(data, &job); err != nil {
		return nil, err
	}
	if job.ID != id || job.IID <= 0 || job.NoteID <= 0 || job.ProjectPath == "" || job.ReviewID == "" || job.DiscussionID == "" {
		return nil, fmt.Errorf("invalid update job identity")
	}
	if plan := job.Plan; plan != nil && (plan.Before == nil || plan.After == nil || plan.Before.ReviewID != job.ReviewID || plan.After.ReviewID != job.ReviewID || plan.HeadSHA == "") {
		return nil, fmt.Errorf("invalid update publication checkpoint")
	}
	return &job, nil
}

func (s *UpdateStore) Pending() ([]UpdateJob, error) {
	dir, err := s.journal.root.Open(".")
	if err != nil {
		return nil, err
	}
	defer func() { _ = dir.Close() }()
	entries, err := dir.ReadDir(-1)
	if err != nil {
		return nil, err
	}
	var jobs []UpdateJob
	var failures []error
	now := time.Now()
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "update-") || !strings.HasSuffix(name, ".json") {
			continue
		}
		job, err := s.Load(strings.TrimSuffix(strings.TrimPrefix(name, "update-"), ".json"))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", name, err))
			continue
		}
		if !job.Done && !job.NextAttempt.After(now) {
			jobs = append(jobs, *job)
		}
	}
	sort.Slice(jobs, func(i, j int) bool { return jobs[i].Created.Before(jobs[j].Created) })
	return jobs, errors.Join(failures...)
}
