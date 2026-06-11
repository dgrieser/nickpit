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
