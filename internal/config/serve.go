package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"gopkg.in/yaml.v3"
)

// DefaultServeConfigPath is the cwd-relative default for --serve-config. The
// serve config is deliberately a separate file from .nickpit.yaml: it holds
// daemon-only tenant data (group tokens, webhook secrets) that review child
// processes must never need or read.
const DefaultServeConfigPath = "server.yaml"

const (
	DefaultServeListen            = ":8080"
	DefaultServeLogDir            = "logs"
	DefaultServeReviewConcurrency = 2
	DefaultServeShutdownGrace     = "10m"
	DefaultServeTopic             = "nickpit"
	DefaultServeTriggerEmoji      = "nickpit"
	DefaultServeStartEmoji        = "eyes"
	DefaultServeCommandKeyword    = "nickpit"
	DefaultServeAckEmoji          = "eyes"
	DefaultServeAbortEmoji        = "stop_button"
)

// ServeConfig configures the `nickpit gitlab serve` webhook daemon.
type ServeConfig struct {
	Listen            string  `yaml:"listen"`
	LogDir            string  `yaml:"log_dir"`
	ReviewConcurrency int     `yaml:"review_concurrency"`
	ShutdownGrace     string  `yaml:"shutdown_grace"`
	GitLabBaseURL     string  `yaml:"gitlab_base_url"`
	Topic             string  `yaml:"topic"`
	TriggerEmoji      string  `yaml:"trigger_emoji"`
	StartEmoji        *string `yaml:"start_emoji"`
	CommandKeyword    string  `yaml:"command_keyword"`
	AckEmoji          *string `yaml:"ack_emoji"`
	AbortEmoji        *string `yaml:"abort_emoji"`
	// GroupsFile optionally names a second YAML file whose top-level `groups:`
	// list is appended to Groups. It lets the group inventory live apart from
	// the main serve config — e.g. in a Kubernetes Secret mounted next to a
	// ConfigMap-rendered server.yaml — so adding a group never touches this
	// file. Like the main file it is env-expanded before parsing. A relative
	// path is resolved against the serve config file's directory, not the
	// process working directory.
	GroupsFile string       `yaml:"groups_file"`
	Groups     []ServeGroup `yaml:"groups"`
	Review     ServeReview  `yaml:"review"`
}

// ServeGroup maps one GitLab group (path prefix) to its access token and the
// credential verifying its webhooks: a GitLab signing token (recommended,
// HMAC-SHA256) or the legacy plaintext secret token. Exactly one is required;
// SigningToken takes precedence when both are set.
type ServeGroup struct {
	Path          string `yaml:"path"`
	Token         string `yaml:"token"`
	WebhookSecret string `yaml:"webhook_secret"`
	// SigningToken is the GitLab webhook signing token ("whsec_<base64>"),
	// generated per webhook via "Generate signing token". GitLab signs each
	// delivery (Standard Webhooks: headers webhook-id/-timestamp/-signature)
	// and the daemon verifies the HMAC instead of comparing a plaintext token.
	SigningToken string `yaml:"signing_token"`
}

// signingTokenPrefix is the GitLab / Standard Webhooks marker on a signing
// token; the HMAC key is the base64 decode of everything after it.
const signingTokenPrefix = "whsec_"

// ParseSigningKey extracts the raw HMAC key from a GitLab signing token. The
// token is "whsec_<base64>" (the prefix is optional/tolerated); the key is the
// standard-base64 decode of the remainder.
func ParseSigningKey(token string) ([]byte, error) {
	raw := strings.TrimPrefix(token, signingTokenPrefix)
	if raw == "" {
		return nil, errors.New("signing token is empty")
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("signing token is not valid base64: %w", err)
	}
	if len(key) == 0 {
		return nil, errors.New("signing token decodes to an empty key")
	}
	return key, nil
}

// ServeReview holds settings forwarded to spawned review child processes.
type ServeReview struct {
	ExtraArgs []string `yaml:"extra_args"`
}

// LoadServe reads and validates a serve config file. Like the main config,
// the raw file text is env-expanded first so tokens and secrets can be
// referenced as ${VAR}. Unlike the main config, a missing file is an error:
// the daemon cannot run without group tokens.
func LoadServe(path string) (*ServeConfig, error) {
	if path == "" {
		path = DefaultServeConfigPath
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("serve config: reading %s: %w", path, err)
	}
	expanded := os.ExpandEnv(string(data))
	cfg := &ServeConfig{
		Listen:            DefaultServeListen,
		LogDir:            DefaultServeLogDir,
		ReviewConcurrency: DefaultServeReviewConcurrency,
		ShutdownGrace:     DefaultServeShutdownGrace,
		Topic:             DefaultServeTopic,
		TriggerEmoji:      DefaultServeTriggerEmoji,
		CommandKeyword:    DefaultServeCommandKeyword,
	}
	if err := yaml.Unmarshal([]byte(expanded), cfg); err != nil {
		return nil, fmt.Errorf("serve config: parsing %s: %w", path, err)
	}
	if cfg.GroupsFile != "" {
		groupsPath := cfg.GroupsFile
		if !filepath.IsAbs(groupsPath) {
			groupsPath = filepath.Join(filepath.Dir(path), groupsPath)
		}
		fileGroups, err := loadGroupsFile(groupsPath)
		if err != nil {
			return nil, fmt.Errorf("serve config: %w", err)
		}
		cfg.Groups = append(cfg.Groups, fileGroups...)
	}
	if cfg.StartEmoji == nil {
		startEmoji := DefaultServeStartEmoji
		cfg.StartEmoji = &startEmoji
	}
	if cfg.AckEmoji == nil {
		ackEmoji := DefaultServeAckEmoji
		cfg.AckEmoji = &ackEmoji
	}
	if cfg.AbortEmoji == nil {
		abortEmoji := DefaultServeAbortEmoji
		cfg.AbortEmoji = &abortEmoji
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("serve config: %s: %w", path, err)
	}
	return cfg, nil
}

// loadGroupsFile reads a groups_file: a YAML document whose top-level
// `groups:` list has the same shape as the serve config's inline groups. The
// raw text is env-expanded first, matching the main file. A file that yields
// no groups is an error — a configured groups_file that contributes nothing is
// almost certainly a mis-mounted or mis-indented document, and silently
// ignoring it would surface later as an unrelated "at least one group" error.
func loadGroupsFile(path string) ([]ServeGroup, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("groups file: reading %s: %w", path, err)
	}
	expanded := os.ExpandEnv(string(data))
	var doc struct {
		Groups []ServeGroup `yaml:"groups"`
	}
	if err := yaml.Unmarshal([]byte(expanded), &doc); err != nil {
		return nil, fmt.Errorf("groups file: parsing %s: %w", path, err)
	}
	if len(doc.Groups) == 0 {
		return nil, fmt.Errorf("groups file: %s: no groups defined", path)
	}
	return doc.Groups, nil
}

// StartEmojiName returns the emoji awarded when a review starts; empty means
// disabled.
func (c *ServeConfig) StartEmojiName() string {
	if c.StartEmoji == nil {
		return DefaultServeStartEmoji
	}
	return *c.StartEmoji
}

// AckEmojiName returns the emoji awarded on a command comment to acknowledge
// it; empty means disabled. Unlike start_emoji it needs no anti-loop check
// against trigger_emoji: it is awarded on a Note, and only MergeRequest
// awardables can trigger reviews.
func (c *ServeConfig) AckEmojiName() string {
	if c.AckEmoji == nil {
		return DefaultServeAckEmoji
	}
	return *c.AckEmoji
}

// AbortEmojiName returns the emoji awarded on a /<keyword> abort command note to
// acknowledge it; empty means disabled. Like AckEmoji it is awarded on a Note,
// so it needs no anti-loop check against trigger_emoji.
func (c *ServeConfig) AbortEmojiName() string {
	if c.AbortEmoji == nil {
		return DefaultServeAbortEmoji
	}
	return *c.AbortEmoji
}

// ShutdownGraceDuration parses the configured shutdown grace period. Validate
// guarantees it parses.
func (c *ServeConfig) ShutdownGraceDuration() time.Duration {
	d, _ := time.ParseDuration(c.ShutdownGrace)
	return d
}

func (c *ServeConfig) Validate() error {
	var errs []error
	if len(c.Groups) == 0 {
		errs = append(errs, errors.New("at least one group must be configured"))
	}
	seen := make(map[string]bool, len(c.Groups))
	for i, group := range c.Groups {
		if group.Path == "" {
			errs = append(errs, fmt.Errorf("groups[%d]: path must not be empty", i))
			continue
		}
		if seen[group.Path] {
			errs = append(errs, fmt.Errorf("groups[%d]: duplicate path %q", i, group.Path))
		}
		seen[group.Path] = true
		if group.Token == "" {
			errs = append(errs, fmt.Errorf("groups[%d] (%s): token must not be empty", i, group.Path))
		}
		switch {
		case group.SigningToken == "" && group.WebhookSecret == "":
			errs = append(errs, fmt.Errorf("groups[%d] (%s): either signing_token or webhook_secret must be set", i, group.Path))
		case group.SigningToken != "":
			if _, err := ParseSigningKey(group.SigningToken); err != nil {
				errs = append(errs, fmt.Errorf("groups[%d] (%s): %w", i, group.Path, err))
			}
		}
	}
	if c.ReviewConcurrency < 1 {
		errs = append(errs, fmt.Errorf("review_concurrency must be >= 1, got %d", c.ReviewConcurrency))
	}
	if _, err := time.ParseDuration(c.ShutdownGrace); err != nil {
		errs = append(errs, fmt.Errorf("shutdown_grace: %w", err))
	}
	if c.Topic == "" {
		errs = append(errs, errors.New("topic must not be empty"))
	}
	if c.TriggerEmoji == "" {
		errs = append(errs, errors.New("trigger_emoji must not be empty"))
	}
	switch {
	case c.CommandKeyword == "":
		errs = append(errs, errors.New("command_keyword must not be empty"))
	case strings.HasPrefix(c.CommandKeyword, "/"):
		errs = append(errs, fmt.Errorf("command_keyword must not start with '/' (got %q): the slash is implied", c.CommandKeyword))
	case strings.ContainsFunc(c.CommandKeyword, unicode.IsSpace):
		errs = append(errs, fmt.Errorf("command_keyword must not contain whitespace (got %q)", c.CommandKeyword))
	}
	if c.LogDir == "" {
		errs = append(errs, errors.New("log_dir must not be empty"))
	}
	// The daemon awards start_emoji on every review it launches; if that were
	// also the trigger emoji, each award would fire an emoji webhook that
	// requests the next review.
	if c.StartEmojiName() != "" && c.StartEmojiName() == c.TriggerEmoji {
		errs = append(errs, fmt.Errorf("start_emoji must differ from trigger_emoji (%q): the daemon's own award would trigger another review", c.TriggerEmoji))
	}
	return errors.Join(errs...)
}
