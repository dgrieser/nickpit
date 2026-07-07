package versionmatch

import "testing"

func TestMatches(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		detected string
		want     bool
	}{
		{"bare minor exact", "1.19", "1.19", true},
		{"bare minor matches patch", "1.19", "1.19.3", true},
		{"bare minor strips go prefix", "1.19", "go1.19.5", true},
		{"bare minor rejects other minor", "1.19", "1.20", false},
		{"bare minor matches docker tag", "1.19", "1.19-alpine", true},
		{"full version exact", "1.19.3", "1.19.3", true},
		{"full version rejects other patch", "1.19.3", "1.19.4", false},
		{"range ge matches", ">=1.22", "1.22.1", true},
		{"range ge rejects lower", ">=1.22", "1.19", false},
		{"range ge matches docker tag", ">=3.10", "3.11-slim", true},
		{"caret range matches", "^1.19", "1.20.1", true},
		{"caret range rejects major bump", "^1.19", "2.0.0", false},
		{"non-semver detected no match", "1.19", "latest", false},
		{"non-semver both exact string", "latest", "latest", true},
		{"detected range-only unmatched by numeric key", "18", "^18", false},
		{"go toolchain directive", "1.21", "go1.21.0", true},
		{"empty detected", "1.19", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Matches(tt.key, tt.detected); got != tt.want {
				t.Errorf("Matches(%q, %q) = %v, want %v", tt.key, tt.detected, got, tt.want)
			}
		})
	}
}

func TestSelectLowest(t *testing.T) {
	tests := []struct {
		name     string
		detected []string
		keys     []string
		wantKey  string
		wantOK   bool
	}{
		{"lowest matches exact key", []string{"1.19", "1.22"}, []string{"1.19", ">=1.22"}, "1.19", true},
		{"order independent", []string{"1.22", "1.19"}, []string{"1.19", ">=1.22"}, "1.19", true},
		{"lowest unmatched falls to higher", []string{"1.20", "1.22"}, []string{"1.19", ">=1.22"}, ">=1.22", true},
		{"no match", []string{"1.20"}, []string{"1.19", ">=1.22"}, "", false},
		{"go prefix sorts numerically", []string{"go1.19.5", "1.22"}, []string{"1.19"}, "1.19", true},
		{"overlap deterministic lowest key", []string{"1.19.3"}, []string{">=1.19", "1.19"}, "1.19", true},
		{"empty detected", nil, []string{"1.19"}, "", false},
		{"empty keys", []string{"1.19"}, nil, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotKey, gotOK := SelectLowest(tt.detected, tt.keys)
			if gotKey != tt.wantKey || gotOK != tt.wantOK {
				t.Errorf("SelectLowest(%v, %v) = (%q, %v), want (%q, %v)", tt.detected, tt.keys, gotKey, gotOK, tt.wantKey, tt.wantOK)
			}
		})
	}
}
