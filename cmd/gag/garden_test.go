package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/config"
	"github.com/RursusAeternum/GitAGarden/internal/github"
	"github.com/RursusAeternum/GitAGarden/internal/live"
	"github.com/RursusAeternum/GitAGarden/internal/scene"
)

func TestSnapshotFallsBackToDemoWithoutToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("PATH", t.TempDir())
	snap, err := source{limit: 8, ttl: time.Minute, decay: 45}.snapshot(context.Background(), nil)
	if err != nil {
		t.Fatalf("a missing token should fall back to the demo, got error %v", err)
	}
	if !strings.Contains(snap.Note, "no GitHub token") {
		t.Errorf("note = %q, want it to explain the missing token", snap.Note)
	}
	if !snap.Demo {
		t.Error("the demo fallback should be marked Demo, so its stars are no baseline")
	}
	if len(snap.Repos) != len(demo) {
		t.Errorf("repos = %d, want the %d demo repos", len(snap.Repos), len(demo))
	}
}

func TestDemoSnapshot(t *testing.T) {
	snap, err := source{demo: true, decay: 45}.snapshot(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if snap.Note != "demo garden" || len(snap.Repos) != len(demo) || snap.FetchedAt.IsZero() {
		t.Errorf("snapshot = note %q, %d repos, fetched %v", snap.Note, len(snap.Repos), snap.FetchedAt)
	}
	if !snap.Demo {
		t.Error("the demo garden should be marked Demo")
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
	if _, err := src.snapshot(context.Background(), nil); !errors.Is(err, github.ErrNoToken) {
		t.Errorf("err = %v; a token hiccup mid-session must not swap in the demo garden", err)
	}
}

func TestCIForMapsGitHubStates(t *testing.T) {
	cases := map[string]live.CI{
		"SUCCESS":       live.CIPassing,
		"FAILURE":       live.CIFailing,
		"ERROR":         live.CIFailing,
		"PENDING":       live.CIPending,
		"EXPECTED":      live.CIPending,
		"":              live.CIUnknown,
		"SOMETHING_NEW": live.CIUnknown,
	}
	for state, want := range cases {
		if got := ciFor(state); got != want {
			t.Errorf("ciFor(%q) = %v, want %v", state, got, want)
		}
	}
}

func TestRepoForComputesSignals(t *testing.T) {
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	day := func(d int) time.Time { return now.Add(-time.Duration(d) * 24 * time.Hour) }
	r := &github.Repo{NameWithOwner: "me/x", Branch: "main", CI: "FAILURE",
		OpenPRs: []github.OpenPR{{Number: 1, CreatedAt: day(9)}, {Number: 2, CreatedAt: day(1), Draft: true}},
		Issues: []github.Issue{{Number: 1, CreatedAt: day(1)}, {Number: 2, CreatedAt: day(2)},
			{Number: 3, CreatedAt: day(3)}, {Number: 4, CreatedAt: day(20)}},
		Commits: []github.Commit{{At: day(1)}, {At: day(2)}, {At: day(3)}, {At: day(20)}},
	}
	lr := repoFor(r, now)
	if lr.Name != "x" || lr.CI != live.CIFailing || lr.Branch != "main" {
		t.Errorf("name %q, ci %v, branch %q", lr.Name, lr.CI, lr.Branch)
	}
	if len(lr.PRs) != 1 || !lr.PRs[0].Equal(day(9)) {
		t.Errorf("PRs = %v, want only the non-draft one", lr.PRs)
	}
	if lr.NewIssues != 3 || !lr.Rising {
		t.Errorf("new issues %d (want 3), rising %v (want true: 3 commits vs 1)", lr.NewIssues, lr.Rising)
	}
}

func TestDemoShowsTheSignals(t *testing.T) {
	var storms, buds, snails, rising int
	for _, r := range demoGarden(time.Now()) {
		if r.CI == live.CIFailing {
			storms++
		}
		buds += len(r.PRs)
		if r.NewIssues >= 3 {
			snails++
		}
		if r.Rising {
			rising++
		}
	}
	if storms == 0 || buds == 0 || snails == 0 || rising == 0 {
		t.Errorf("demo should show every signal: storms %d, buds %d, snails %d, rising %d", storms, buds, snails, rising)
	}
}

func TestMergePrefersFlagsThenTheFile(t *testing.T) {
	file := config.Defaults()
	file.Sky, file.Limit, file.Refresh, file.User = "stars", 12, 2*time.Minute, "octocat"
	flags := settings{limit: 3, refresh: 5 * time.Minute, decay: 45}
	got := merge(file, map[string]bool{"limit": true}, flags)
	if got.sky != scene.SkyStars || got.limit != 3 || got.refresh != 2*time.Minute || got.user != "octocat" || got.decay != 45 {
		t.Errorf("merged = %+v; want stars, limit 3 (flag), refresh 2m and user octocat (file), decay 45", got)
	}
}

func TestUserFlagBeatsTheFilesRepoList(t *testing.T) {
	file := config.Defaults()
	file.Repos = []string{"me/a"}
	got := merge(file, map[string]bool{"user": true}, settings{user: "octocat", limit: 8, refresh: 5 * time.Minute, decay: 45})
	if got.user != "octocat" || got.repos != nil {
		t.Errorf("merged = %+v; -user should show octocat's garden, not the file's repos", got)
	}
}

func TestRepoForCarriesStars(t *testing.T) {
	if lr := repoFor(&github.Repo{NameWithOwner: "me/x", Stars: 42}, time.Now()); lr.Stars != 42 {
		t.Errorf("stars = %d, want 42", lr.Stars)
	}
}

func TestDemoHasStars(t *testing.T) {
	total := 0
	for _, r := range demoGarden(time.Now()) {
		total += r.Stars
	}
	if total == 0 {
		t.Error("the demo should have stars for the star sky")
	}
}
