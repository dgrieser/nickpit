package model

import (
	"testing"
	"time"
)

func TestHumanTokens(t *testing.T) {
	tests := []struct {
		in   int
		want string
	}{
		{0, "0"},
		{841, "841"},
		{999, "999"},
		{1000, "1k"},
		{1500, "1.5k"},
		{38192, "38.2k"},
		{999949, "999.9k"},
		{999950, "1M"},
		{1500000, "1.5M"},
		{2000000, "2M"},
		{1230000000, "1.2G"},
		{-38192, "-38.2k"},
	}
	for _, tt := range tests {
		if got := HumanTokens(tt.in); got != tt.want {
			t.Errorf("HumanTokens(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestHumanDuration(t *testing.T) {
	tests := []struct {
		in   time.Duration
		want string
	}{
		{0, "0s"},
		{499 * time.Millisecond, "0s"},
		{12 * time.Second, "12s"},
		{4*time.Minute + 12*time.Second, "4m12s"},
		{time.Hour + 2*time.Minute + 3*time.Second, "1h2m3s"},
	}
	for _, tt := range tests {
		if got := HumanDuration(tt.in); got != tt.want {
			t.Errorf("HumanDuration(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestRuntimeSeconds(t *testing.T) {
	tests := []struct {
		in   time.Duration
		want float64
	}{
		{0, 0},
		{1500 * time.Millisecond, 1.5},
		{12345 * time.Millisecond, 12.35},
		{4*time.Minute + 12*time.Second, 252},
	}
	for _, tt := range tests {
		if got := RuntimeSeconds(tt.in); got != tt.want {
			t.Errorf("RuntimeSeconds(%v) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestNormalizeBaseURL(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"https://host/v1", "https://host/v1"},
		{"https://host/v1/", "https://host/v1"},
		{"  https://host/v1//  ", "https://host/v1"},
		{"http://localhost:10000/v1", "http://localhost:10000/v1"},
	}
	for _, tt := range tests {
		if got := NormalizeBaseURL(tt.in); got != tt.want {
			t.Errorf("NormalizeBaseURL(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestSameEndpoint(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"", "", true},
		{"https://host/v1", "https://host/v1/", true},
		{" https://host/v1 ", "https://host/v1", true},
		{"https://host/v1", "https://host/v2", false},
		{"http://localhost:10000/v1", "https://llm.example/v1", false},
		{"", "https://host/v1", false},
	}
	for _, tt := range tests {
		if got := SameEndpoint(tt.a, tt.b); got != tt.want {
			t.Errorf("SameEndpoint(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestClipLine(t *testing.T) {
	for _, tc := range []struct {
		name  string
		in    string
		limit int
		want  string
	}{
		{"collapses every whitespace run", " a\n\tb  c\n", 100, "a b c"},
		// A raw carriage return rewinds the terminal cursor over the line's own
		// prefix, and an escape sequence outlives the line it arrived in.
		{"drops control characters", "boom\rrewound", 100, "boom rewound"},
		{"drops escape sequences", "boom \x1b[31mred", 100, "boom [31mred"},
		{"clips with an ellipsis", "abcdefghij", 8, "abcde..."},
		{"keeps a result at the limit", "abcdefgh", 8, "abcdefgh"},
		{"clips hard below the ellipsis", "abcdefgh", 2, "ab"},
		{"a limit of zero clips nothing", "a\nb", 0, "a b"},
		{"whitespace only", " \n\t ", 10, ""},
	} {
		if got := ClipLine(tc.in, tc.limit); got != tc.want {
			t.Fatalf("%s: ClipLine(%q, %d) = %q, want %q", tc.name, tc.in, tc.limit, got, tc.want)
		}
	}
}
