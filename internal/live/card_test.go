package live

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/garden"
	"github.com/RursusAeternum/GitAGarden/internal/scene"
)

func fullRepo() Repo {
	r := grow("gag-core", 60, 4, t0.Add(-2*time.Hour))
	r.Branch, r.CI, r.Stars = "main", CIPassing, 31
	r.Detail = Detail{FullName: "RursusAeternum/gag-core", Language: "Go",
		LastCommit: Entry{Title: "Make storms readable", At: t0.Add(-2 * time.Hour)},
		Daily:      [14]int{0, 1, 2, 4, 1, 0, 0, 1, 3, 4, 1, 0, 2, 4},
		PRs: []Entry{
			{Number: 12, Title: "Add sparkline", At: t0.Add(-2 * 24 * time.Hour)},
			{Number: 9, Title: "Bump deps", At: t0.Add(-10 * 24 * time.Hour)},
			{Number: 15, Title: "Fix typo", At: t0.Add(-time.Hour)},
		},
		Drafts: 1, OpenIssues: 4,
		NewestIssue: Entry{Number: 31, Title: "Crash on empty repo", At: t0.Add(-2 * 24 * time.Hour)},
		Release:     Entry{Title: "v0.5.0", At: t0.Add(-time.Hour)}}
	return r
}

func TestCardForAFullRepo(t *testing.T) {
	c := CardFor(fullRepo(), t0, 45, 40)
	if c.Title != "gag-core" || c.Note != "Go · shrub" {
		t.Errorf("title row = %q / %q", c.Title, c.Note)
	}
	want := []scene.CardLine{
		{Value: "RursusAeternum/gag-core", Right: "★ 31"},
		{Label: "state", Value: "thriving · tended 2h ago"},
		{Label: "last", Value: "Make storms readable"},
		{Label: "14 days", Value: "·▂▄█▂··▂▆█▂·▄█ 23 commits"},
		{Label: "CI", Value: "✓ passing on main", Tone: scene.ToneGood},
		{Label: "PRs", Value: "3 open · oldest 10d"},
		{Sub: true, Value: "#9 Bump deps", Right: "10d"},
		{Sub: true, Value: "#12 Add sparkline", Right: "2d"},
		{Sub: true, Value: "+1 more · +1 draft"},
		{Label: "issues", Value: "4 open · newest 2d ago"},
		{Sub: true, Value: "#31 Crash on empty repo"},
		{Label: "release", Value: "v0.5.0 · 1h ago", Tone: scene.ToneGold},
	}
	if !reflect.DeepEqual(c.Lines, want) {
		t.Errorf("lines:\n%+v\nwant\n%+v", c.Lines, want)
	}
}

func TestCardForSaysWhenThereIsNothing(t *testing.T) {
	c := CardFor(grow("seed", 1, 0, t0.Add(-time.Hour)), t0, 45, 40)
	want := []scene.CardLine{
		{Value: "seed"},
		{Label: "state", Value: "thriving · tended 1h ago"},
		{Label: "last", Value: "no commits yet"},
		{Label: "14 days", Value: "·············· 0 commits"},
		{Label: "CI", Value: "– no checks"},
		{Label: "PRs", Value: "none open"},
		{Label: "issues", Value: "none open"},
		{Label: "release", Value: "none yet"},
	}
	if c.Note != "shrub" || !reflect.DeepEqual(c.Lines, want) {
		t.Errorf("note %q, lines:\n%+v\nwant\n%+v", c.Note, c.Lines, want)
	}
}

func TestCardStates(t *testing.T) {
	wilting := grow("quiet", 40, 0, t0.Add(-40*24*time.Hour))
	done := grow("old", 30, 0, t0.Add(-90*24*time.Hour))
	done.Finished = true
	fresh := Repo{Name: "new", Plant: garden.Grow("new", garden.Shrub, nil)}
	failing := grow("red", 10, 0, t0)
	failing.CI, failing.Branch = CIFailing, "main"
	for _, c := range []struct {
		r    Repo
		line int
		want scene.CardLine
	}{
		{wilting, 1, scene.CardLine{Label: "state", Value: "wilting · 40d quiet"}},
		{done, 1, scene.CardLine{Label: "state", Value: "under glass · tended 90d ago"}},
		{fresh, 1, scene.CardLine{Label: "state", Value: "no activity yet"}},
		{failing, 4, scene.CardLine{Label: "CI", Value: "✗ failing on main", Tone: scene.ToneBad}},
	} {
		if got := CardFor(c.r, t0, 45, 40).Lines[c.line]; got != c.want {
			t.Errorf("%s: %+v, want %+v", c.r.Name, got, c.want)
		}
	}
}

func TestCardDropsLinesInOrder(t *testing.T) {
	labels := func(c scene.Card) string {
		var s []string
		for _, l := range c.Lines {
			switch {
			case l.Sub:
				s = append(s, "+")
			case l.Label == "":
				s = append(s, "name")
			default:
				s = append(s, l.Label)
			}
		}
		return strings.Join(s, " ")
	}
	for rows, want := range map[int]string{
		14: "name state last 14 days CI PRs + + + issues + release",
		13: "name state last 14 days CI PRs + + issues + release",
		11: "name state last 14 days CI PRs issues + release",
		10: "name state last 14 days CI PRs issues release",
		9:  "name state last 14 days CI PRs issues",
		8:  "name state last CI PRs issues",
		7:  "state last CI PRs issues",
		6:  "state CI PRs issues",
		5:  "state CI PRs",
		4:  "state CI",
		3:  "state CI",
	} {
		if got := labels(CardFor(fullRepo(), t0, 45, rows)); got != want {
			t.Errorf("%d rows: %s\nwant %s", rows, got, want)
		}
	}
}

func TestCardCountsDrafts(t *testing.T) {
	onlyDrafts := grow("wip", 10, 0, t0)
	onlyDrafts.Detail.Drafts = 2
	one := grow("one", 10, 0, t0)
	one.Detail.PRs = []Entry{{Number: 3, Title: "Fix it", At: t0.Add(-48 * time.Hour)}}
	one.Detail.Drafts = 1
	lines := func(r Repo) []scene.CardLine { return CardFor(r, t0, 45, 40).Lines[5:] }
	if got := lines(onlyDrafts)[0]; got.Value != "2 drafts" {
		t.Errorf("only drafts: PRs line %+v, want \"2 drafts\"", got)
	}
	want := []scene.CardLine{
		{Label: "PRs", Value: "1 open · oldest 2d"},
		{Sub: true, Value: "#3 Fix it", Right: "2d"},
		{Sub: true, Value: "+1 draft"},
	}
	if got := lines(one)[:3]; !reflect.DeepEqual(got, want) {
		t.Errorf("one PR and a draft:\n%+v\nwant\n%+v", got, want)
	}
}
