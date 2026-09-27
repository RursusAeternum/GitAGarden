package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/github"
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

func TestCachedFallbackOnlyShowsYourOwnRepos(t *testing.T) {
	day := func(d int) time.Time { return time.Date(2026, 9, d, 0, 0, 0, 0, time.UTC) }
	store := &github.Store{Viewer: "me", Repos: map[string]*github.Repo{
		"me/a":    {NameWithOwner: "me/a", PushedAt: day(1)},
		"other/b": {NameWithOwner: "other/b", PushedAt: day(9)}, // cached by -user once
		"me/c":    {NameWithOwner: "me/c", PushedAt: day(5)},
	}}
	repos, offline, err := cachedRepos(store, nil, "", 2, errors.New("no network"))
	if err != nil || !offline {
		t.Fatalf("offline=%v err=%v, want cached repos", offline, err)
	}
	if len(repos) != 2 || repos[0].NameWithOwner != "me/c" || repos[1].NameWithOwner != "me/a" {
		t.Errorf("got %v, want only your own repos, newest first", names(repos))
	}
	store.Viewer = "" // never listed online: whose repos are yours is unknown
	if repos, _, err := cachedRepos(store, nil, "", 2, errors.New("no network")); err == nil {
		t.Errorf("unknown viewer should not guess, got %v", names(repos))
	}
}

func names(repos []*github.Repo) []string {
	var out []string
	for _, r := range repos {
		out = append(out, r.NameWithOwner)
	}
	return out
}

func TestTokenLossAfterARealLoadIsAnError(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("PATH", t.TempDir())
	src := source{limit: 8, ttl: time.Minute, decay: 45, state: &sourceState{real: true}}
	if _, err := src.snapshot(context.Background()); !errors.Is(err, github.ErrNoToken) {
		t.Errorf("err = %v; a token hiccup mid-session must not swap in the demo garden", err)
	}
}
