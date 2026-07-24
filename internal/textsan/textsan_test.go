package textsan

import (
	"strings"
	"testing"
)

func TestStripControl(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "hello world", "hello world"},
		{"keeps newline and tab", "a\nb\tc", "a\nb\tc"},
		{"strips esc csi", "\x1b[31mred\x1b[0m", "[31mred[0m"},
		{"strips alt screen switch", "\x1b[?1049hX", "[?1049hX"},
		{"strips bare control bytes", "a\x07\x00\x08b", "ab"},
		{"strips DEL", "a\x7fb", "ab"},
		{"strips C1 codepoint", "a\u009bb", "ab"},
		{"keeps unicode", "héllo → 世界", "héllo → 世界"},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := StripControl(tc.in)
			if got != tc.want {
				t.Fatalf("StripControl(%q) = %q, want %q", tc.in, got, tc.want)
			}
			if strings.ContainsRune(got, 0x1b) {
				t.Fatalf("output still contains ESC: %q", got)
			}
		})
	}
}

func TestRedactSecrets(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"json api key", `{"api_key":"sk-secretvalue"}`, `{"api_key":"[redacted]"}`},
		{"yaml token", "github_token: ghp_abcdefghijk", `github_token: "[redacted]"`},
		{"env password", "DB_PASSWORD=hunter123", `DB_PASSWORD="[redacted]"`},
		{"bearer token", "Authorization header: Bearer abcdefgh.ijklmnop", "Authorization header: Bearer [redacted]"},
		{"openai token", "request failed for sk-abcdefghijklmnop", "request failed for [redacted]"},
		{"gitlab token", "request failed for glpat-abcdefghijklmnop", "request failed for [redacted]"},
		{"aws access key", "request failed for AKIAABCDEFGHIJKLMNOP", "request failed for [redacted]"},
		{"mixed-case prefixed tokens", "SK-ABCDEFGHIJK GHP_ABCDEFGHIJK GlPaT-AbCdEfGhIjKl", "[redacted] [redacted] [redacted]"},
		{"ampersand in unquoted value", "api_key=abc&def", `api_key="[redacted]"`},
		{"query-like unquoted value", "api_key=sk-abcdefgh&session=tok-secret", `api_key="[redacted]"`},
		{"multiple credentials", `api_key="sk-abcdefgh" and token=ghp_abcdefghijk`, `api_key="[redacted]" and token="[redacted]"`},
		{"newline-separated credentials", "password=hunter123\naccess_token=secret456", "password=\"[redacted]\"\naccess_token=\"[redacted]\""},
		{"keeps unicode around credential", "世界 api_key=secretvalue τέλος", `世界 api_key="[redacted]" τέλος`},
		{"keeps ordinary text", "token count: 123", "token count: 123"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RedactSecrets(tt.in); got != tt.want {
				t.Fatalf("RedactSecrets(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
