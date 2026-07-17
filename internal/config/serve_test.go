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
	if cfg.CommandKeyword != "nickpit" || cfg.AckEmojiName() != "eyes" || cfg.AbortEmojiName() != "stop_button" {
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

func TestLoadServeGroupsFile(t *testing.T) {
	t.Setenv("NICKPIT_TEST_GF_TOKEN", "file-token")
	groupsPath := filepath.Join(t.TempDir(), "groups.yaml")
	if err := os.WriteFile(groupsPath, []byte(`
groups:
  - path: "platform"
    token: "${NICKPIT_TEST_GF_TOKEN}"
    webhook_secret: "sec"
`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadServe(writeServeConfig(t, "groups_file: "+groupsPath+"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Groups) != 1 || cfg.Groups[0].Path != "platform" {
		t.Fatalf("groups = %+v", cfg.Groups)
	}
	if cfg.Groups[0].Token != "file-token" {
		t.Fatalf("token = %q, want env-expanded", cfg.Groups[0].Token)
	}
}

func TestLoadServeGroupsFileRelativePath(t *testing.T) {
	// A relative groups_file resolves against the serve config's directory,
	// not the process cwd (the test cwd is the package dir, so a cwd-based
	// lookup would fail here).
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "groups.yaml"), []byte(`
groups:
  - path: "platform"
    token: "tok"
    webhook_secret: "sec"
`), 0o600); err != nil {
		t.Fatal(err)
	}
	serverPath := filepath.Join(dir, "server.yaml")
	if err := os.WriteFile(serverPath, []byte("groups_file: groups.yaml\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadServe(serverPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Groups) != 1 || cfg.Groups[0].Path != "platform" {
		t.Fatalf("groups = %+v", cfg.Groups)
	}
}

func TestLoadServeGroupsFileMergesWithInline(t *testing.T) {
	groupsPath := filepath.Join(t.TempDir(), "groups.yaml")
	if err := os.WriteFile(groupsPath, []byte(`
groups:
  - path: "extra"
    token: "tok2"
    webhook_secret: "sec2"
`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadServe(writeServeConfig(t, `
groups_file: `+groupsPath+`
groups:
  - path: "platform"
    token: "tok"
    webhook_secret: "sec"
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Groups) != 2 || cfg.Groups[0].Path != "platform" || cfg.Groups[1].Path != "extra" {
		t.Fatalf("groups = %+v", cfg.Groups)
	}
}

func TestLoadServeGroupsFileErrors(t *testing.T) {
	dir := t.TempDir()
	emptyPath := filepath.Join(dir, "empty.yaml")
	if err := os.WriteFile(emptyPath, []byte("# no groups here\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	dupPath := filepath.Join(dir, "dup.yaml")
	if err := os.WriteFile(dupPath, []byte("groups:\n  - path: p\n    token: t2\n    webhook_secret: s2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name    string
		content string
		wantErr string
	}{
		{"missing file", "groups_file: " + filepath.Join(dir, "absent.yaml") + "\n", "groups file: reading"},
		{"empty file", "groups_file: " + emptyPath + "\n", "no groups defined"},
		{"duplicate across files", "groups_file: " + dupPath + "\ngroups:\n  - path: p\n    token: t\n    webhook_secret: s\n", "duplicate path"},
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

func TestLoadServeMissingFile(t *testing.T) {
	_, err := LoadServe(filepath.Join(t.TempDir(), "absent.yaml"))
	if err == nil {
		t.Fatal("expected error for missing serve config")
	}
}

func TestLoadServeLokiDisabledByDefault(t *testing.T) {
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
	if cfg.Loki.Enabled() {
		t.Fatal("loki must be disabled when url is unset")
	}
}

func TestLoadServeLokiConfig(t *testing.T) {
	t.Setenv("NICKPIT_TEST_LOKI_PASS", "loki-secret")
	path := writeServeConfig(t, `
loki:
  url: "http://loki:3100"
  tenant_id: "team-a"
  basic_auth_user: "svc"
  basic_auth_pass: "${NICKPIT_TEST_LOKI_PASS}"
  labels:
    env: "prod"
  batch_wait: "2s"
  batch_max_lines: 250
  gzip: true
groups:
  - path: "platform"
    token: "tok"
    webhook_secret: "sec"
`)
	cfg, err := LoadServe(path)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Loki.Enabled() {
		t.Fatal("loki should be enabled")
	}
	if cfg.Loki.BasicAuthPass != "loki-secret" {
		t.Fatalf("basic_auth_pass not env-expanded: %q", cfg.Loki.BasicAuthPass)
	}
	if cfg.Loki.TenantID != "team-a" || cfg.Loki.Labels["env"] != "prod" || !cfg.Loki.Gzip {
		t.Fatalf("loki fields = %+v", cfg.Loki)
	}
	if cfg.Loki.BatchWaitDuration() != 2*time.Second || cfg.Loki.BatchMaxLinesOrDefault() != 250 {
		t.Fatalf("loki batch = %v / %d", cfg.Loki.BatchWaitDuration(), cfg.Loki.BatchMaxLinesOrDefault())
	}
	// Unset numeric/duration fields fall back to defaults.
	if cfg.Loki.TimeoutDuration() != 10*time.Second || cfg.Loki.BufferLinesOrDefault() != DefaultLokiBufferLines {
		t.Fatalf("loki defaults = %v / %d", cfg.Loki.TimeoutDuration(), cfg.Loki.BufferLinesOrDefault())
	}
}

// TestLoadServeLokiHelmRenderedShape feeds the exact server.yaml the Helm
// chart's serverYaml helper emits for a fully-populated serve.loki block
// (${ENV} refs for secrets, nindent-style labels, all tunables). It guards
// against the chart and the Go schema drifting apart, since `helm template`
// cannot run in this environment.
func TestLoadServeLokiHelmRenderedShape(t *testing.T) {
	t.Setenv("LOKI_TENANT", "team-a")
	t.Setenv("LOKI_USER", "svc")
	t.Setenv("LOKI_PASS", "p@ss")
	// Mirrors the "loki:" section rendered by templates/_helpers.tpl (indented
	// like the ConfigMap's nindent 4 output, minus that outer indent).
	rendered := `listen: ":8080"
log_dir: "/work/logs"
review_concurrency: 2
shutdown_grace: "10m"
gitlab_base_url: "https://gitlab.example.com"
topic: "nickpit"
trigger_emoji: "nickpit"
groups:
  - path: "platform"
    token: "tok"
    signing_token: "` + "whsec_" + base64.StdEncoding.EncodeToString([]byte("key")) + `"
review:
  extra_args: []
loki:
  url: "http://loki-gateway.monitoring:80"
  tenant_id: "${LOKI_TENANT}"
  basic_auth_user: "${LOKI_USER}"
  basic_auth_pass: "${LOKI_PASS}"
  labels:
    env: "prod"
  batch_wait: "1s"
  batch_max_lines: 1000
  timeout: "10s"
  buffer_lines: 4096
  gzip: false
`
	cfg, err := LoadServe(writeServeConfig(t, rendered))
	if err != nil {
		t.Fatalf("helm-rendered server.yaml must parse and validate: %v", err)
	}
	if !cfg.Loki.Enabled() || cfg.Loki.TenantID != "team-a" || cfg.Loki.BasicAuthUser != "svc" || cfg.Loki.BasicAuthPass != "p@ss" {
		t.Fatalf("loki = %+v", cfg.Loki)
	}
	if cfg.Loki.Labels["env"] != "prod" || cfg.Loki.BufferLines != 4096 {
		t.Fatalf("loki labels/buffer = %+v", cfg.Loki)
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
		{"loki bad scheme", "loki:\n  url: \"ftp://loki:3100\"\ngroups:\n  - path: p\n    token: t\n    webhook_secret: s\n", "loki.url must be an http(s) URL"},
		{"loki bad batch_wait", "loki:\n  url: \"http://loki:3100\"\n  batch_wait: nope\ngroups:\n  - path: p\n    token: t\n    webhook_secret: s\n", "loki.batch_wait"},
		{"loki half auth", "loki:\n  url: \"http://loki:3100\"\n  basic_auth_user: u\ngroups:\n  - path: p\n    token: t\n    webhook_secret: s\n", "must be set together"},
		{"loki reserved label", "loki:\n  url: \"http://loki:3100\"\n  labels:\n    project: nope\ngroups:\n  - path: p\n    token: t\n    webhook_secret: s\n", "is reserved"},
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
