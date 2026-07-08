package config

import (
	"encoding/base64"
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
	if cfg.CommandKeyword != "nickpit" || cfg.AckEmojiName() != "white_check_mark" {
		t.Fatalf("command defaults = %+v", cfg)
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

func TestLoadServeSigningToken(t *testing.T) {
	// A group with only a signing token (no webhook_secret) is valid, and the
	// token decodes to a non-empty HMAC key.
	token := "whsec_" + base64.StdEncoding.EncodeToString([]byte("super-secret-key"))
	path := writeServeConfig(t, `
groups:
  - path: "platform"
    token: "tok"
    signing_token: "`+token+`"
`)
	cfg, err := LoadServe(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Groups[0].SigningToken != token {
		t.Fatalf("signing token = %q", cfg.Groups[0].SigningToken)
	}
	key, err := ParseSigningKey(cfg.Groups[0].SigningToken)
	if err != nil {
		t.Fatalf("ParseSigningKey: %v", err)
	}
	if string(key) != "super-secret-key" {
		t.Fatalf("key = %q", key)
	}
}

func TestParseSigningKey(t *testing.T) {
	cases := []struct {
		name    string
		token   string
		wantErr bool
	}{
		{"with prefix", "whsec_" + base64.StdEncoding.EncodeToString([]byte("k")), false},
		{"without prefix", base64.StdEncoding.EncodeToString([]byte("k")), false},
		{"empty", "", true},
		{"prefix only", "whsec_", true},
		{"bad base64", "whsec_not!!base64", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseSigningKey(tc.token)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tc.wantErr)
			}
		})
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

func TestLoadServeAckEmojiDisabled(t *testing.T) {
	path := writeServeConfig(t, `
ack_emoji: ""
groups:
  - path: "platform"
    token: "tok"
    webhook_secret: "sec"
`)
	cfg, err := LoadServe(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AckEmojiName() != "" {
		t.Fatalf("ack emoji = %q, want disabled", cfg.AckEmojiName())
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
		{"no credential", "groups:\n  - path: p\n    token: t\n", "either signing_token or webhook_secret must be set"},
		{"bad signing token", "groups:\n  - path: p\n    token: t\n    signing_token: \"whsec_not!!base64\"\n", "not valid base64"},
		{"duplicate path", "groups:\n  - path: p\n    token: t\n    webhook_secret: s\n  - path: p\n    token: t2\n    webhook_secret: s2\n", "duplicate path"},
		{"bad duration", "shutdown_grace: nope\ngroups:\n  - path: p\n    token: t\n    webhook_secret: s\n", "shutdown_grace"},
		{"bad concurrency", "review_concurrency: 0\ngroups:\n  - path: p\n    token: t\n    webhook_secret: s\n", "review_concurrency"},
		{"empty topic", "topic: \"\"\ngroups:\n  - path: p\n    token: t\n    webhook_secret: s\n", "topic must not be empty"},
		{"empty log dir", "log_dir: \"\"\ngroups:\n  - path: p\n    token: t\n    webhook_secret: s\n", "log_dir must not be empty"},
		{"start equals trigger emoji", "start_emoji: nickpit\ngroups:\n  - path: p\n    token: t\n    webhook_secret: s\n", "start_emoji must differ from trigger_emoji"},
		{"empty command keyword", "command_keyword: \"\"\ngroups:\n  - path: p\n    token: t\n    webhook_secret: s\n", "command_keyword must not be empty"},
		{"slash command keyword", "command_keyword: \"/nickpit\"\ngroups:\n  - path: p\n    token: t\n    webhook_secret: s\n", "command_keyword must not start with '/'"},
		{"whitespace command keyword", "command_keyword: \"nick pit\"\ngroups:\n  - path: p\n    token: t\n    webhook_secret: s\n", "command_keyword must not contain whitespace"},
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
