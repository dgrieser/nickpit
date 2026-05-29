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
		{"strips C1 codepoint", "ab", "ab"},
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
