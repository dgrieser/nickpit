// Package chatgpt implements Sign in with ChatGPT for open-source tools: the
// OAuth loopback sign-in that registers nickpit with the user's ChatGPT
// account, the protected credential file, token refresh, and the account's
// model catalog. Inference itself goes through the llm package's Responses
// transport with the access token this package supplies.
//
// See https://developers.openai.com/siwc/token-sharing-open-source/sign-in.
package chatgpt

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// AuthFileEnv overrides where the credential file lives, so a machine that
// cannot open a browser (a serve daemon, CI) can use a file signed in
// elsewhere.
const AuthFileEnv = "NICKPIT_CHATGPT_AUTH_FILE"

// Credentials is the protected record of one signed-in ChatGPT account. It
// stays on disk with 0600 permissions and is never logged.
type Credentials struct {
	Version  int    `json:"version"`
	ClientID string `json:"client_id"`
	Issuer   string `json:"issuer,omitempty"`
	Subject  string `json:"subject,omitempty"`
	Email    string `json:"email,omitempty"`
	Name     string `json:"name,omitempty"`

	IDToken           string    `json:"id_token,omitempty"`
	AccessToken       string    `json:"access_token,omitempty"`
	RefreshToken      string    `json:"refresh_token,omitempty"`
	TokenType         string    `json:"token_type,omitempty"`
	ExpiresAt         time.Time `json:"expires_at,omitzero"`
	EarliestRefreshAt time.Time `json:"earliest_refresh_at,omitzero"`
	Scopes            []string  `json:"scopes,omitempty"`
	SavedAt           time.Time `json:"saved_at,omitzero"`
}

// SignedIn reports whether the record holds usable (possibly expired but
// refreshable) credentials.
func (c *Credentials) SignedIn() bool {
	return c != nil && c.AccessToken != "" && c.RefreshToken != ""
}

// PlanUsage reports whether the grant lets nickpit spend the ChatGPT plan.
func (c *Credentials) PlanUsage() bool {
	if c == nil {
		return false
	}
	return slices.Contains(c.Scopes, ScopePlanUsage)
}

// Store locates the credential file and the per-installation host ID.
type Store struct {
	Dir      string
	AuthFile string
}

// DefaultStore returns the store under the user's config directory
// (~/.config/nickpit on Linux), honouring NICKPIT_CHATGPT_AUTH_FILE.
func DefaultStore() (*Store, error) {
	if path := strings.TrimSpace(os.Getenv(AuthFileEnv)); path != "" {
		return &Store{Dir: filepath.Dir(path), AuthFile: path}, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("chatgpt: locating config directory: %w", err)
	}
	dir := filepath.Join(base, "nickpit")
	return &Store{Dir: dir, AuthFile: filepath.Join(dir, "chatgpt-auth.json")}, nil
}

func (s *Store) hostFile() string { return filepath.Join(s.Dir, "chatgpt-host.json") }
func (s *Store) lockFile() string { return s.AuthFile + ".lock" }

// Load reads the credential file. A missing file returns (nil, nil).
func (s *Store) Load() (*Credentials, error) {
	data, err := os.ReadFile(s.AuthFile)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("chatgpt: reading %s: %w", s.AuthFile, err)
	}
	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("chatgpt: parsing %s: %w", s.AuthFile, err)
	}
	return &creds, nil
}

// Save writes the credential file atomically with owner-only permissions.
func (s *Store) Save(creds *Credentials) error {
	creds.Version = 1
	creds.SavedAt = time.Now().UTC()
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(s.AuthFile, append(data, '\n'))
}

// HostID returns this installation's stable ext_agent_host_id, creating it on
// first use. It survives sign-out so every sign-in from this machine presents
// the same host.
func (s *Store) HostID() (string, error) {
	var record struct {
		HostID string `json:"host_id"`
	}
	data, err := os.ReadFile(s.hostFile())
	if err == nil && json.Unmarshal(data, &record) == nil && strings.HasPrefix(record.HostID, "urn:uuid:") {
		return record.HostID, nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("chatgpt: reading host id: %w", err)
	}
	record.HostID = "urn:uuid:" + newUUID()
	data, err = json.MarshalIndent(record, "", "  ")
	if err != nil {
		return "", err
	}
	if err := writeFileAtomic(s.hostFile(), append(data, '\n')); err != nil {
		return "", err
	}
	return record.HostID, nil
}

func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("chatgpt: creating %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*")
	if err != nil {
		return fmt.Errorf("chatgpt: writing %s: %w", path, err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chatgpt: writing %s: %w", path, err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chatgpt: writing %s: %w", path, err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chatgpt: writing %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("chatgpt: writing %s: %w", path, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("chatgpt: writing %s: %w", path, err)
	}
	return nil
}

func newUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
