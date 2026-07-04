package config

import (
	"errors"
	"fmt"
	"os"
	"time"

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
)

// ServeConfig configures the `nickpit gitlab serve` webhook daemon.
type ServeConfig struct {
	Listen            string       `yaml:"listen"`
	LogDir            string       `yaml:"log_dir"`
	ReviewConcurrency int          `yaml:"review_concurrency"`
	ShutdownGrace     string       `yaml:"shutdown_grace"`
	GitLabBaseURL     string       `yaml:"gitlab_base_url"`
	Topic             string       `yaml:"topic"`
	TriggerEmoji      string       `yaml:"trigger_emoji"`
	StartEmoji        *string      `yaml:"start_emoji"`
	Groups            []ServeGroup `yaml:"groups"`
	Review            ServeReview  `yaml:"review"`
}

// ServeGroup maps one GitLab group (path prefix) to its access token and
// webhook secret.
type ServeGroup struct {
	Path          string `yaml:"path"`
	Token         string `yaml:"token"`
	WebhookSecret string `yaml:"webhook_secret"`
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
	}
	if err := yaml.Unmarshal([]byte(expanded), cfg); err != nil {
		return nil, fmt.Errorf("serve config: parsing %s: %w", path, err)
	}
	if cfg.StartEmoji == nil {
		startEmoji := DefaultServeStartEmoji
		cfg.StartEmoji = &startEmoji
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("serve config: %s: %w", path, err)
	}
	return cfg, nil
}

// StartEmojiName returns the emoji awarded when a review starts; empty means
// disabled.
func (c *ServeConfig) StartEmojiName() string {
	if c.StartEmoji == nil {
		return DefaultServeStartEmoji
	}
	return *c.StartEmoji
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
		if group.WebhookSecret == "" {
			errs = append(errs, fmt.Errorf("groups[%d] (%s): webhook_secret must not be empty", i, group.Path))
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
	return errors.Join(errs...)
}
