package live

import (
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/RursusAeternum/GitAGarden/internal/garden"
	"github.com/RursusAeternum/GitAGarden/internal/scene"
)

// mergeSlack is how much newer than a merge its own commits can be: GitHub
// commits a merge within moments of it.
const mergeSlack = time.Minute

// Change is something that happened to a repo between two online loads,
// with the ticker note that says so.
type Change struct {
	Repo   string
	Kind   scene.ReactKind
	Icon   string        // the note's icon
	Text   string        // and its text
	Before *garden.Plant // the plant before the change
	Agent  string        // the AI agent behind a push; "" for people
	Weeds  int           // how many weeds a new or closed issue grows or pulls
}

// ChangesBetween lists what happened from before to after: repos in after's
// order and, per repo, push, merge, release, closed issue, new issue, then
// CI. There are none without a before, and none for repos that just joined,
// repos sharing a short name, or finished repos.
func ChangesBetween(before, after []Repo) []Change {
	if before == nil {
		return nil
	}
	was, now := unique(before), unique(after)
	var out []Change
	for _, r := range after {
		b, known := was[r.Name]
		if _, single := now[r.Name]; !known || !single || r.Finished || b.Finished || r.Plant == nil || b.Plant == nil {
			continue
		}
		out = append(out, changesOf(b, r)...)
	}
	return out
}

// settled is after as the next load compares with it. A repo whose CI is
// running, or unknown for a moment, keeps the passing or failing state it
// last settled on: a fix that runs and then passes still clears the storm,
// and a failed check fetch doesn't roll it in again.
func settled(before, after []Repo) []Repo {
	was := unique(before)
	out := slices.Clone(after)
	for i, r := range out {
		if b, ok := was[r.Name]; ok && (r.CI == CIPending || r.CI == CIUnknown) && (b.CI == CIPassing || b.CI == CIFailing) {
			out[i].CI = b.CI
		}
	}
	return out
}

// unique maps repo names to repos, leaving out names that appear twice: a
// fork and its upstream can't be told apart.
func unique(repos []Repo) map[string]Repo {
	count := map[string]int{}
	for _, r := range repos {
		count[r.Name]++
	}
	out := make(map[string]Repo, len(repos))
	for _, r := range repos {
		if count[r.Name] == 1 {
			out[r.Name] = r
		}
	}
	return out
}

// changesOf is what happened to one repo from b to r.
func changesOf(b, r Repo) []Change {
	var out []Change
	add := func(k scene.ReactKind, icon, text, agent string) {
		out = append(out, Change{Repo: r.Name, Kind: k, Icon: icon, Text: r.Name + ": " + text, Before: b.Plant, Agent: agent})
	}
	d, bd := r.Detail, b.Detail
	pushes, merges := r.Plant.Pushes-b.Plant.Pushes, r.Plant.Merges-b.Plant.Merges
	// Not a push when the new commits are the merges' own: a squash, a merge
	// commit, or a PR's commits recommitted as it merged. Those are no newer
	// than the merge.
	mergesOwn := merges > 0 && !d.LastCommit.At.After(d.LastMerge.At.Add(mergeSlack))
	if d.LastCommit.At.After(bd.LastCommit.At) && pushes > merges && !mergesOwn {
		text := "pushed"
		if title := tickerText(d.LastCommit.Title); title != "" {
			text = `"` + title + `"`
		}
		if n := pushes - merges; n > 1 {
			text += fmt.Sprintf(" · %d commits", n)
		}
		if d.LastCommit.By != "" {
			text += " · by " + d.LastCommit.By
		}
		add(scene.ReactPush, "💧", text, d.LastCommit.By)
	}
	if merges > 0 {
		text := "merged a PR"
		if d.LastMerge.Number != 0 {
			text = fmt.Sprintf("merged #%d %s", d.LastMerge.Number, tickerText(d.LastMerge.Title))
		}
		if merges > 1 {
			text += fmt.Sprintf(" · +%d more", merges-1)
		}
		add(scene.ReactMerge, "🌸", text, "")
	}
	if d.Release.Title != "" && d.Release.Title != bd.Release.Title {
		add(scene.ReactRelease, "✨", "released "+tickerText(d.Release.Title), "")
	}
	// A closed issue before a new one: its weed is pulled first, so a new
	// weed can come up where it stood.
	closed := d.LastClosed.Number != 0 && d.LastClosed.At.After(bd.LastClosed.At)
	opened := d.NewestIssue.Number != 0 && d.NewestIssue.At.After(bd.NewestIssue.At)
	pulled, grown := weedCounts(b.Plant.OpenIssues, r.Plant.OpenIssues, closed, opened)
	if closed {
		add(scene.ReactWeedOut, "✅", fmt.Sprintf("closed #%d %s", d.LastClosed.Number, tickerText(d.LastClosed.Title)), "")
		out[len(out)-1].Weeds = pulled
	}
	if opened {
		add(scene.ReactWeedIn, "🐛", fmt.Sprintf("#%d %s", d.NewestIssue.Number, tickerText(d.NewestIssue.Title)), "")
		out[len(out)-1].Weeds = grown
	}
	switch {
	case r.CI == CIFailing && b.CI != CIFailing:
		text := "CI failing"
		if r.Branch != "" {
			text += " on " + r.Branch
		}
		add(scene.ReactStorm, "⚡", text, "")
	case b.CI == CIFailing && r.CI == CIPassing:
		add(scene.ReactClear, "🌈", "CI passing again", "")
	}
	return out
}

// weedCounts splits a change in open issues, before to after, into the weeds
// the closed issues pull and the new ones grow. Each reaction moves at least
// one weed, when there is one to move; with both, they add up to the change.
func weedCounts(before, after int, closed, opened bool) (pulled, grown int) {
	switch {
	case closed && opened:
		pulled = min(before, max(1, before-after+1))
		grown = after - before + pulled
	case closed:
		pulled = min(before, max(1, before-after))
	case opened:
		grown = max(1, after-before)
	}
	return pulled, grown
}

// tickerText cleans text for a ticker note. Tabs and newlines become spaces,
// and control characters, format runes (joiners, tags), enclosing marks and
// variation selectors go, so a Gitmoji or a stray escape code can't shift
// the line. Combining marks stay: they take no cell, and scripts such as
// Devanagari and Thai need them.
func tickerText(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\t' || r == '\n':
			return ' '
		case unicode.IsControl(r), unicode.In(r, unicode.Me, unicode.Cf, unicode.Variation_Selector):
			return -1
		}
		return r
	}, s)
}
