package main

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestSnapshotFallsBackToDemoWithoutToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("PATH", t.TempDir())
	snap, err := source{limit: 8, ttl: time.Minute, decay: 45}.snapshot(context.Background())
	if err != nil {
		t.Fatalf("a missing token should fall back to the demo, got error %v", err)
	}
	if !strings.Contains(snap.Note, "no GitHub token") {
		t.Errorf("note = %q, want it to explain the missing token", snap.Note)
	}
	if len(snap.Repos) != len(demo) {
		t.Errorf("repos = %d, want the %d demo repos", len(snap.Repos), len(demo))
	}
}

func TestDemoSnapshot(t *testing.T) {
	snap, err := source{demo: true, decay: 45}.snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snap.Note != "demo garden" || len(snap.Repos) != len(demo) || snap.FetchedAt.IsZero() {
		t.Errorf("snapshot = note %q, %d repos, fetched %v", snap.Note, len(snap.Repos), snap.FetchedAt)
	}
}
