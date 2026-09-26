package main

import (
	"testing"
	"time"
)

func TestParseSpan(t *testing.T) {
	day := 24 * time.Hour
	good := map[string]time.Duration{
		"30d":    30 * day,
		"2w":     14 * day,
		"1w3d":   10 * day,
		"36h":    36 * time.Hour,
		"1d12h":  36 * time.Hour,
		"90m":    90 * time.Minute,
		" 7d ":   7 * day,
		"2w1d6h": 15*day + 6*time.Hour,
	}
	for in, want := range good {
		got, err := parseSpan(in)
		if err != nil || got != want {
			t.Errorf("parseSpan(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, in := range []string{"d", "xd", "30", "3y", "abc"} {
		if got, err := parseSpan(in); err == nil {
			t.Errorf("parseSpan(%q) = %v, want error", in, got)
		}
	}
}
