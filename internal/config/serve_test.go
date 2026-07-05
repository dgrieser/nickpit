package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeServeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "server.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadServeDefaults(t *testing.T) {
	path := writeServeConfig(t, `
groups:
  - path: "platform"
    token: "tok"
    webhook_secret: "sec"
`)
	cfg, err := LoadServe(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Listen != ":8080" || cfg.LogDir != "logs" || cfg.ReviewConcurrency != 2 {
		t.Fatalf("defaults = %+v", cfg)
	}
	if cfg.Topic != "nickpit" || cfg.TriggerEmoji != "nickpit" || cfg.StartEmojiName() != "eyes" {
		t.Fatalf("emoji/topic defaults = %+v", cfg)
	}
	if cfg.ShutdownGraceDuration() != 10*time.Minute {
		t.Fatalf("shutdown grace = %v", cfg.ShutdownGraceDuration())
	}
}

func TestLoadServeEnvExpansion(t *testing.T) {
	t.Setenv("NICKPIT_TEST_GL_TOKEN", "secret-token")
	t.Setenv("NICKPIT_TEST_GL_SECRET", "hook-secret")
	path := writeServeConfig(t, `
groups:
  - path: "platform"
    token: "${NICKPIT_TEST_GL_TOKEN}"
    webhook_secret: "${NICKPIT_TEST_GL_SECRET}"
`)
	cfg, err := LoadServe(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Groups[0].Token != "secret-token" || cfg.Groups[0].WebhookSecret != "hook-secret" {
		t.Fatalf("groups = %+v", cfg.Groups)
	}
}

func TestLoadServeStartEmojiDisabled(t *testing.T) {
	path := writeServeConfig(t, `
start_emoji: ""
groups:
  - path: "platform"
    token: "tok"
    webhook_secret: "sec"
`)
	cfg, err := LoadServe(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.StartEmojiName() != "" {
		t.Fatalf("start emoji = %q, want disabled", cfg.StartEmojiName())
	}
}

func TestLoadServeMissingFile(t *testing.T) {
	_, err := LoadServe(filepath.Join(t.TempDir(), "absent.yaml"))
	if err == nil {
		t.Fatal("expected error for missing serve config")
	}
}

func TestServeConfigValidate(t *testing.T) {
	cases := []struct {
		name    string
		content string
		wantErr string
	}{
		{"no groups", `listen: ":8080"`, "at least one group"},
		{"empty token", "groups:\n  - path: p\n    webhook_secret: s\n", "token must not be empty"},
		{"empty secret", "groups:\n  - path: p\n    token: t\n", "webhook_secret must not be empty"},
		{"duplicate path", "groups:\n  - path: p\n    token: t\n    webhook_secret: s\n  - path: p\n    token: t2\n    webhook_secret: s2\n", "duplicate path"},
		{"bad duration", "shutdown_grace: nope\ngroups:\n  - path: p\n    token: t\n    webhook_secret: s\n", "shutdown_grace"},
		{"bad concurrency", "review_concurrency: 0\ngroups:\n  - path: p\n    token: t\n    webhook_secret: s\n", "review_concurrency"},
		{"empty topic", "topic: \"\"\ngroups:\n  - path: p\n    token: t\n    webhook_secret: s\n", "topic must not be empty"},
		{"empty log dir", "log_dir: \"\"\ngroups:\n  - path: p\n    token: t\n    webhook_secret: s\n", "log_dir must not be empty"},
		{"start equals trigger emoji", "start_emoji: nickpit\ngroups:\n  - path: p\n    token: t\n    webhook_secret: s\n", "start_emoji must differ from trigger_emoji"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := LoadServe(writeServeConfig(t, tc.content))
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want containing %q", err, tc.wantErr)
			}
		})
	}
}
