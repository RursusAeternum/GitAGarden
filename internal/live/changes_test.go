package live

import (
	"testing"
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/scene"
)

// baseRepo is gag-core as a refresh found it: CI passing on main, a release
// and an open issue.
func baseRepo() Repo {
	r := grow("gag-core", 10, 1, t0.Add(-2*time.Hour))
	r.Branch, r.CI = "main", CIPassing
	r.Detail = Detail{
		LastCommit:  Entry{Title: "old", At: t0.Add(-2 * time.Hour)},
		Release:     Entry{Title: "v0.5.0", At: t0.Add(-48 * time.Hour)},
		NewestIssue: Entry{Number: 30, Title: "Old bug", At: t0.Add(-72 * time.Hour)},
	}
	return r
}

// regrown is r with its plant grown to pushes and merges, as of t0.
func regrown(r Repo, pushes, merges int) Repo {
	r.Plant = grow(r.Name, pushes, merges, t0).Plant
	return r
}

func TestChangesAreDetected(t *testing.T) {
	cases := []struct {
		name   string
		change func(Repo) Repo
		kind   scene.ReactKind
		note   string
		agent  string
	}{
		{"push", func(r Repo) Repo {
			r = regrown(r, 13, 1)
			r.Detail.LastCommit = Entry{Title: "Draw the card beside the plant", At: t0, By: "Claude"}
			return r
		}, scene.ReactPush, `💧 gag-core: "Draw the card beside the plant" · 3 commits · by Claude`, "Claude"},
		{"merge", func(r Repo) Repo {
			r = regrown(r, 10, 3)
			r.Detail.LastMerge = Entry{Number: 41, Title: "Add a detail card", At: t0}
			return r
		}, scene.ReactMerge, "🌸 gag-core: merged #41 Add a detail card · +1 more", ""},
		{"release", func(r Repo) Repo {
			r.Detail.Release = Entry{Title: "v0.6.0", At: t0}
			return r
		}, scene.ReactRelease, "✨ gag-core: released v0.6.0", ""},
		{"new issue", func(r Repo) Repo {
			r.Detail.NewestIssue = Entry{Number: 31, Title: "Crash on empty repo", At: t0}
			return r
		}, scene.ReactWeedIn, "🐛 gag-core: #31 Crash on empty repo", ""},
		{"closed issue", func(r Repo) Repo {
			r.Detail.LastClosed = Entry{Number: 30, Title: "Old bug", At: t0}
			return r
		}, scene.ReactWeedOut, "✅ gag-core: closed #30 Old bug", ""},
		{"CI red", func(r Repo) Repo {
			r.CI = CIFailing
			return r
		}, scene.ReactStorm, "⚡ gag-core: CI failing on main", ""},
	}
	for _, c := range cases {
		before := baseRepo()
		got := ChangesBetween([]Repo{before}, []Repo{c.change(baseRepo())})
		if len(got) != 1 {
			t.Errorf("%s: %d changes: %+v", c.name, len(got), got)
			continue
		}
		g := got[0]
		if g.Repo != "gag-core" || g.Kind != c.kind || g.Icon+" "+g.Text != c.note || g.Agent != c.agent ||
			g.Before == nil || g.Before.Pushes != before.Plant.Pushes {
			t.Errorf("%s: %+v, want kind %v, %q, agent %q", c.name, g, c.kind, c.note, c.agent)
		}
	}
	red := baseRepo()
	red.CI = CIFailing
	if got := ChangesBetween([]Repo{red}, []Repo{baseRepo()}); len(got) != 1 || got[0].Kind != scene.ReactClear ||
		got[0].Icon+" "+got[0].Text != "🌈 gag-core: CI passing again" {
		t.Errorf("CI green: %+v", got)
	}
}

func TestASquashMergeIsNotAPush(t *testing.T) {
	after := regrown(baseRepo(), 11, 2) // one new commit: the merge's own
	after.Detail.LastCommit = Entry{Title: "Add a detail card (#41)", At: t0}
	after.Detail.LastMerge = Entry{Number: 41, Title: "Add a detail card", At: t0}
	if got := ChangesBetween([]Repo{baseRepo()}, []Repo{after}); len(got) != 1 || got[0].Kind != scene.ReactMerge {
		t.Errorf("a squash merge: %+v, want just the merge", got)
	}
}

func TestChangesThatDontCount(t *testing.T) {
	pushed := regrown(baseRepo(), 12, 1)
	pushed.Detail.LastCommit = Entry{Title: "new", At: t0}
	if got := ChangesBetween(nil, []Repo{pushed}); got != nil {
		t.Errorf("the first load is the baseline: %+v", got)
	}
	newcomer := pushed
	newcomer.Name = "newcomer"
	if got := ChangesBetween([]Repo{baseRepo()}, []Repo{baseRepo(), newcomer}); len(got) != 0 {
		t.Errorf("a repo that just joined: %+v", got)
	}
	if got := ChangesBetween([]Repo{baseRepo(), baseRepo()}, []Repo{pushed, pushed}); len(got) != 0 {
		t.Errorf("repos sharing a name: %+v", got)
	}
	done := pushed
	done.Finished = true
	if got := ChangesBetween([]Repo{baseRepo()}, []Repo{done}); len(got) != 0 {
		t.Errorf("a finished repo: %+v", got)
	}
	if got := ChangesBetween([]Repo{baseRepo()}, []Repo{baseRepo()}); len(got) != 0 {
		t.Errorf("nothing changed: %+v", got)
	}
}

func TestChangesComeInOrder(t *testing.T) {
	a := regrown(baseRepo(), 14, 2) // four commits, one of them a merge's
	a.CI = CIFailing
	a.Detail.LastCommit = Entry{Title: "Speed up", At: t0}
	a.Detail.LastMerge = Entry{Number: 41, Title: "Add a detail card", At: t0.Add(-10 * time.Minute)} // merged before the push
	other := grow("zz", 5, 0, t0.Add(-time.Hour))
	red := other
	red.CI = CIFailing
	got := ChangesBetween([]Repo{baseRepo(), other}, []Repo{a, red})
	want := []struct {
		repo string
		kind scene.ReactKind
	}{{"gag-core", scene.ReactPush}, {"gag-core", scene.ReactMerge}, {"gag-core", scene.ReactStorm}, {"zz", scene.ReactStorm}}
	if len(got) != len(want) {
		t.Fatalf("changes: %+v", got)
	}
	for i, w := range want {
		if got[i].Repo != w.repo || got[i].Kind != w.kind {
			t.Errorf("change %d = %s %v, want %s %v", i, got[i].Repo, got[i].Kind, w.repo, w.kind)
		}
	}
	if got[0].Text != `gag-core: "Speed up" · 3 commits` || got[3].Text != "zz: CI failing" {
		t.Errorf("notes: %q, %q", got[0].Text, got[3].Text)
	}
}

func TestNoteTextIsCleaned(t *testing.T) {
	a := regrown(baseRepo(), 11, 1)
	a.Detail.LastCommit = Entry{Title: "♻️ Tidy\tup\x1b[31m", At: t0}
	if got := ChangesBetween([]Repo{baseRepo()}, []Repo{a}); len(got) != 1 || got[0].Text != `gag-core: "♻ Tidy up[31m"` {
		t.Errorf("cleaned note: %+v", got)
	}
}

func TestAMergesOwnCommitsAreNotAPush(t *testing.T) {
	merged := Entry{Number: 41, Title: "Add a detail card", At: t0}
	for _, c := range []struct {
		name   string
		pushes int
		last   Entry
	}{
		{"a merge commit", 13, Entry{Title: "Merge pull request #41 from kaine/card", At: t0}}, // two PR commits and the merge's own
		{"a rebase merge", 12, Entry{Title: "Draw the card", At: t0.Add(2 * time.Second)}},     // the PR's commits, recommitted as it merges
	} {
		after := regrown(baseRepo(), c.pushes, 2)
		after.Detail.LastCommit, after.Detail.LastMerge = c.last, merged
		if got := ChangesBetween([]Repo{baseRepo()}, []Repo{after}); len(got) != 1 || got[0].Kind != scene.ReactMerge {
			t.Errorf("%s: %+v, want just the merge", c.name, got)
		}
	}
	later := regrown(baseRepo(), 13, 2) // a commit pushed after the merge
	later.Detail.LastCommit, later.Detail.LastMerge = Entry{Title: "Tidy up", At: t0.Add(10 * time.Minute)}, merged
	if got := ChangesBetween([]Repo{baseRepo()}, []Repo{later}); len(got) != 2 || got[0].Kind != scene.ReactPush {
		t.Errorf("a push after a merge: %+v, want the push and the merge", got)
	}
}
