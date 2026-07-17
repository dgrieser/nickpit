// Package session persists resumable discussion (chat) sessions to disk. A
// session caches the gathered review context and result for quick resume and
// records the full message transcript so a later turn replays the same
// conversation. Files are written atomically (temp + rename), one JSON file per
// session under the user cache dir, mirroring internal/modelcheck/cache.go.
package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/google/uuid"
)

// Version is the on-disk schema version. Bump it when the shape changes
// incompatibly so old files can be detected.
const Version = 1

// maxStoredSessions bounds how many session files a store keeps. Every review
// auto-saves a session (including reviews run by the serve daemon's children),
// so without a cap the directory grows one file — containing a full review
// context — per review, forever. Save prunes the oldest files past this limit.
const maxStoredSessions = 50

// Source describes where a session's review came from, with enough detail to
// recreate the diff at resume time (from a local ref range or a remote MR/PR).
type Source struct {
	Mode       string `json:"mode"` // "local" | "gitlab" | "github"
	Submode    string `json:"submode,omitempty"`
	Repo       string `json:"repo,omitempty"`
	Identifier int    `json:"identifier,omitempty"` // MR / PR number
	BaseRef    string `json:"base_ref,omitempty"`
	HeadRef    string `json:"head_ref,omitempty"`
	BaseURL    string `json:"base_url,omitempty"`
	RepoRoot   string `json:"repo_root,omitempty"`
}

// ToolCall mirrors llm.ToolCall for persistence.
type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// Message is one persisted conversation message. It mirrors llm.Message and adds
// bookkeeping (timestamp, per-turn token usage) that never reaches the model.
type Message struct {
	Role       string            `json:"role"`
	Content    string            `json:"content,omitempty"`
	Name       string            `json:"name,omitempty"`
	ToolCallID string            `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall        `json:"tool_calls,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
	Tokens     *model.TokenUsage `json:"tokens,omitempty"`
}

// Session is one resumable discussion.
type Session struct {
	Version         int       `json:"version"`
	ID              string    `json:"id"`
	ReviewID        string    `json:"review_id,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	Source          Source    `json:"source"`
	PinnedFindingID string    `json:"pinned_finding_id,omitempty"`
	Model           string    `json:"model,omitempty"`
	Profile         string    `json:"profile,omitempty"`

	// Context and Result are the cached review context and findings, so a resume
	// can build the agent prompt without re-fetching. Context may be rebuilt from
	// Source when stale (e.g. the MR gained commits).
	Context        *model.ReviewContext `json:"context,omitempty"`
	ContextHeadSHA string               `json:"context_head_sha,omitempty"`
	Result         *model.ReviewResult  `json:"result,omitempty"`

	// Messages is the full conversation transcript (user, assistant, and tool
	// messages), in order.
	Messages []Message `json:"messages"`
}

// New creates an empty session with a fresh id and timestamps.
func New() *Session {
	now := time.Now().UTC()
	return &Session{
		Version:   Version,
		ID:        uuid.NewString(),
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// Append adds messages to the transcript.
func (s *Session) Append(msgs ...Message) {
	s.Messages = append(s.Messages, msgs...)
}

// Conversation returns the transcript as llm messages for the discussion agent.
func (s *Session) Conversation() []llm.Message {
	out := make([]llm.Message, 0, len(s.Messages))
	for _, m := range s.Messages {
		out = append(out, m.LLM())
	}
	return out
}

// LLM converts a persisted message to an llm.Message.
func (m Message) LLM() llm.Message {
	out := llm.Message{Role: m.Role, Content: m.Content, Name: m.Name, ToolCallID: m.ToolCallID}
	if len(m.ToolCalls) > 0 {
		out.ToolCalls = make([]llm.ToolCall, len(m.ToolCalls))
		for i, tc := range m.ToolCalls {
			out.ToolCalls[i] = llm.ToolCall{ID: tc.ID, Name: tc.Name, Arguments: tc.Arguments}
		}
	}
	return out
}

// FromLLM converts an llm.Message to a persisted message, stamping it now.
func FromLLM(m llm.Message) Message {
	out := Message{Role: m.Role, Content: m.Content, Name: m.Name, ToolCallID: m.ToolCallID, CreatedAt: time.Now().UTC()}
	if len(m.ToolCalls) > 0 {
		out.ToolCalls = make([]ToolCall, len(m.ToolCalls))
		for i, tc := range m.ToolCalls {
			out.ToolCalls[i] = ToolCall{ID: tc.ID, Name: tc.Name, Arguments: tc.Arguments}
		}
	}
	return out
}

// UserMessage builds a user-role persisted message.
func UserMessage(content string) Message {
	return Message{Role: "user", Content: content, CreatedAt: time.Now().UTC()}
}

// Store reads and writes sessions under a directory.
type Store struct {
	dir string
}

// DefaultDir resolves the session directory: $NICKPIT_CACHE_DIR/sessions when
// set, else <user cache dir>/nickpit/sessions.
func DefaultDir() (string, error) {
	if dir := strings.TrimSpace(os.Getenv("NICKPIT_CACHE_DIR")); dir != "" {
		return filepath.Join(dir, "sessions"), nil
	}
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("session: resolving user cache dir: %w", err)
	}
	return filepath.Join(dir, "nickpit", "sessions"), nil
}

// NewStore builds a store rooted at dir; an empty dir uses DefaultDir.
func NewStore(dir string) (*Store, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		resolved, err := DefaultDir()
		if err != nil {
			return nil, err
		}
		dir = resolved
	}
	return &Store{dir: dir}, nil
}

// Dir returns the store's directory.
func (s *Store) Dir() string { return s.dir }

// Path returns the file path for a session id.
func (s *Store) Path(id string) (string, error) {
	if err := validateID(id); err != nil {
		return "", err
	}
	return filepath.Join(s.dir, id+".json"), nil
}

// Load reads a session by id.
func (s *Store) Load(id string) (*Session, error) {
	path, err := s.Path(id)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("session: %q not found", id)
		}
		return nil, fmt.Errorf("session: reading %s: %w", path, err)
	}
	var sess Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, fmt.Errorf("session: parsing %s: %w", path, err)
	}
	return &sess, nil
}

// Save writes the session atomically, refreshing UpdatedAt.
func (s *Store) Save(sess *Session) error {
	if sess == nil {
		return fmt.Errorf("session: nil session")
	}
	if err := validateID(sess.ID); err != nil {
		return err
	}
	if sess.Version == 0 {
		sess.Version = Version
	}
	if sess.CreatedAt.IsZero() {
		sess.CreatedAt = time.Now().UTC()
	}
	sess.UpdatedAt = time.Now().UTC()

	path, err := s.Path(sess.ID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("session: creating directory: %w", err)
	}
	data, err := json.MarshalIndent(sess, "", "  ")
	if err != nil {
		return fmt.Errorf("session: encoding: %w", err)
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(s.dir, ".session-*.tmp")
	if err != nil {
		return fmt.Errorf("session: creating temp file: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("session: writing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("session: closing temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("session: replacing %s: %w", path, err)
	}
	s.prune()
	return nil
}

// prune deletes the oldest session files beyond maxStoredSessions, judged by
// file modification time so no session needs to be decoded. Best-effort: a
// prune failure never fails the save that triggered it.
func (s *Store) prune() {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return
	}
	type fileAge struct {
		name string
		mod  time.Time
	}
	var files []fileAge
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, fileAge{name: entry.Name(), mod: info.ModTime()})
	}
	if len(files) <= maxStoredSessions {
		return
	}
	sort.Slice(files, func(i, j int) bool { return files[i].mod.Before(files[j].mod) })
	for _, file := range files[:len(files)-maxStoredSessions] {
		_ = os.Remove(filepath.Join(s.dir, file.name))
	}
}

// Info is a lightweight session listing entry.
type Info struct {
	ID              string
	ReviewID        string
	Repo            string
	PinnedFindingID string
	UpdatedAt       time.Time
}

// List returns known sessions, newest first. A missing directory yields an empty
// list rather than an error.
func (s *Store) List() ([]Info, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("session: listing %s: %w", s.dir, err)
	}
	var infos []Info
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		sess, err := s.Load(id)
		if err != nil {
			continue // skip unreadable/corrupt files
		}
		infos = append(infos, Info{
			ID:              sess.ID,
			ReviewID:        sess.ReviewID,
			Repo:            sess.Source.Repo,
			PinnedFindingID: sess.PinnedFindingID,
			UpdatedAt:       sess.UpdatedAt,
		})
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].UpdatedAt.After(infos[j].UpdatedAt) })
	return infos, nil
}

// Latest returns the most recently updated session, or (nil, nil) when none
// exist.
func (s *Store) Latest() (*Session, error) {
	infos, err := s.List()
	if err != nil {
		return nil, err
	}
	if len(infos) == 0 {
		return nil, nil
	}
	return s.Load(infos[0].ID)
}

// validateID rejects ids that could escape the store directory.
func validateID(id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("session: empty id")
	}
	if strings.ContainsAny(id, `/\`) || id == "." || id == ".." || strings.Contains(id, "..") {
		return fmt.Errorf("session: invalid id %q", id)
	}
	return nil
}
