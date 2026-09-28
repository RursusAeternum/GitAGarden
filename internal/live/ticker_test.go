package live

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mattn/go-runewidth"

	"github.com/RursusAeternum/GitAGarden/internal/garden"
)

func TestMain(m *testing.M) {
	time.Local = time.UTC // daylight and sky follow the local clock; pin it
	os.Exit(m.Run())
}

var t0 = time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

// grow makes a shrub Repo whose last push was at last; merges add flowers.
func grow(name string, pushes, merges int, last time.Time) Repo {
	var ev []garden.Event
	for i := 0; i < pushes; i++ {
		ev = append(ev, garden.Event{Kind: garden.Push, At: last.Add(-time.Duration(pushes-1-i) * time.Hour)})
	}
	for i := 0; i < merges; i++ {
		ev = append(ev, garden.Event{Kind: garden.Merge, At: last.Add(-time.Duration(merges-i) * time.Minute)})
	}
	return Repo{Name: name, Plant: garden.Grow(name, garden.Shrub, ev)}
}

func TestItemsCallOutWiltingMostWiltedFirst(t *testing.T) {
	done := grow("done", 30, 0, t0.Add(-90*24*time.Hour))
	done.Finished = true
	repos := []Repo{
		grow("fresh", 30, 0, t0.Add(-24*time.Hour)),
		grow("quiet", 30, 0, t0.Add(-30*24*time.Hour)),
		grow("ancient", 30, 0, t0.Add(-60*24*time.Hour)),
		done,
	}
	items := Items(repos, t0, 45, 12)
	want := []string{"ancient: 60d quiet", "quiet: 30d quiet"}
	if len(items) != len(want) {
		t.Fatalf("items = %+v, want %v", items, want)
	}
	for i, w := range want {
		if items[i].Text != w || items[i].Icon != "🥀" {
			t.Errorf("item %d = %+v, want 🥀 %q", i, items[i], w)
		}
	}
}

func TestItemsAreCalmWhenAllIsWell(t *testing.T) {
	repos := []Repo{grow("fresh", 30, 0, t0.Add(-time.Hour))}
	for commits, want := range map[int]string{
		12: "Garden thriving: 12 commits this week",
		1:  "Garden thriving: 1 commit this week",
		0:  "All quiet in the garden",
	} {
		got := Items(repos, t0, 45, commits)
		if len(got) != 1 || got[0].Text != want || got[0].Icon != "🌱" {
			t.Errorf("commits=%d: %+v, want 🌱 %q", commits, got, want)
		}
	}
}

func TestLineCyclesAndFitsExactly(t *testing.T) {
	items := []Item{{"🥀", "a: 3d quiet"}, {"🥀", "b: 9d quiet"}}
	if l := Line(items, 0, "updated just now", 60); !strings.Contains(l, "a: 3d quiet") {
		t.Errorf("first item missing: %q", l)
	}
	if l := Line(items, 4*time.Second, "updated just now", 60); !strings.Contains(l, "b: 9d quiet") {
		t.Errorf("second item missing after 4s: %q", l)
	}
	if l := Line(items, 8*time.Second, "", 60); !strings.Contains(l, "a: 3d quiet") {
		t.Errorf("items should wrap around: %q", l)
	}
	for _, w := range []int{60, 25, 10, 3} {
		if got := runewidth.StringWidth(Line(items, 0, "updated just now", w)); got != w {
			t.Errorf("width %d: line is %d cells", w, got)
		}
	}
}

func TestLineTruncatesTheItemNotTheStatus(t *testing.T) {
	long := []Item{{"🥀", strings.Repeat("very-long-repo-name-", 5) + ": 9d quiet"}}
	l := Line(long, 0, "updated 3m ago", 50)
	if !strings.HasSuffix(l, "updated 3m ago ") {
		t.Errorf("status lost: %q", l)
	}
	if !strings.Contains(l, "…") {
		t.Errorf("long item should be truncated with …: %q", l)
	}
}

func TestStatus(t *testing.T) {
	fetched := Snapshot{FetchedAt: t0.Add(-3 * time.Minute)}
	cases := []struct{ got, want string }{
		{Status(Snapshot{}, t0, true, false), "loading…"},
		{Status(Snapshot{}, t0, false, false), ""},
		{Status(fetched, t0, false, false), "updated 3m ago"},
		{Status(fetched, t0, true, false), "refreshing…"},
		{Status(fetched, t0, false, true), "refresh failed · data from 3m ago"},
		{Status(Snapshot{FetchedAt: t0.Add(-2 * time.Hour), Offline: true}, t0, false, false), "offline · cached 2h ago"},
		{Status(Snapshot{FetchedAt: t0.Add(-20 * time.Second)}, t0, false, false), "updated just now"},
		{Status(Snapshot{Note: "demo garden", FetchedAt: t0}, t0, false, false), "demo garden"},
	}
	for i, c := range cases {
		if c.got != c.want {
			t.Errorf("case %d: %q, want %q", i, c.got, c.want)
		}
	}
}

func TestItemsFollowTheAttentionOrder(t *testing.T) {
	fresh := t0.Add(-time.Hour)
	days := func(d int) time.Time { return t0.Add(-time.Duration(d) * 24 * time.Hour) }
	ci := grow("ci", 20, 0, fresh)
	ci.CI, ci.Branch = CIFailing, "main"
	slow := grow("slow", 20, 0, fresh)
	slow.PRs = []time.Time{days(9), days(2), days(12)}
	quiet := grow("quiet", 20, 0, days(40))
	newpr := grow("newpr", 20, 0, fresh)
	newpr.PRs = []time.Time{days(1)}
	buggy := grow("buggy", 20, 0, fresh)
	buggy.NewIssues = 4
	got := Items([]Repo{buggy, newpr, quiet, slow, ci}, t0, 45, 3)
	want := []string{
		"⚡ ci: CI failing on main",
		"🌷 slow: 2 PRs waiting (12d)",
		"🥀 quiet: 40d quiet",
		"🌷 newpr: 1 PR open",
		"🐌 buggy: 4 new issues this week",
	}
	if len(got) != len(want) {
		t.Fatalf("items = %+v", got)
	}
	for i, w := range want {
		if g := got[i].Icon + " " + got[i].Text; g != w {
			t.Errorf("item %d = %q, want %q", i, g, w)
		}
	}
}

func TestTickerIconsAreTwoCellsWide(t *testing.T) {
	for _, icon := range []string{"⚡", "🌷", "🥀", "🐌", "🌱", "⭐", "💧", "🌸", "✨", "🐛", "✅", "🌈"} {
		if w := runewidth.StringWidth(icon); w != 2 {
			t.Errorf("%s is %d cells wide; ticker icons must be two-cell emoji", icon, w)
		}
	}
}
