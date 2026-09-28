package live

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/garden"
	"github.com/RursusAeternum/GitAGarden/internal/scene"
)

// How a card sheds lines when the window is too short: the highest rank goes
// first, its last line first. Lines ranked keep always stay.
const (
	keep    = iota
	dropPRs // the PR summary: dropped last
	dropIssues
	dropLast
	dropName // the owner/name line
	dropDays // the 14-day sparkline
	dropRelease
	dropIssueTitle
	dropPRTitles // PR title lines and "+N more": dropped first
)

type rankedLine struct {
	scene.CardLine
	rank int
}

// CardFor is r's detail card at time now, at most maxRows terminal rows tall
// with its border. scene cleans and trims the text when it draws the card.
func CardFor(r Repo, now time.Time, decay float64, maxRows int) scene.Card {
	if r.Plant == nil {
		r.Plant = &garden.Plant{}
	}
	d := r.Detail
	card := scene.Card{Title: r.Name, Note: string(r.Plant.Species)}
	if d.Language != "" {
		card.Note = strings.TrimSuffix(d.Language+" · "+card.Note, " · ")
	}
	name := scene.CardLine{Value: d.FullName}
	if name.Value == "" {
		name.Value = r.Name
	}
	if r.Stars > 0 {
		name.Right = fmt.Sprintf("★ %d", r.Stars)
	}
	last := d.LastCommit.Title
	if last == "" {
		last = "no commits yet"
	}
	lines := []rankedLine{
		{name, dropName},
		{scene.CardLine{Label: "state", Value: stateOf(r, now, decay)}, keep},
		{scene.CardLine{Label: "last", Value: last}, dropLast},
		{scene.CardLine{Label: "14 days", Value: Sparkline(d.Daily) + " " + plural(total(d.Daily), "commit")}, dropDays},
		{ciLine(r), keep},
	}
	lines = append(lines, prLines(d, now)...)
	lines = append(lines, issueLines(d, now)...)
	rel := scene.CardLine{Label: "release", Value: "none yet"}
	if d.Release.Title != "" {
		rel.Value, rel.Tone = d.Release.Title+" · "+ago(now.Sub(d.Release.At)), scene.ToneGold
	}
	lines = append(lines, rankedLine{rel, dropRelease})
	for len(lines)+2 > maxRows && drop(&lines) {
	}
	for _, l := range lines {
		card.Lines = append(card.Lines, l.CardLine)
	}
	return card
}

// stateOf is the card's state line: thriving, wilting or under glass.
func stateOf(r Repo, now time.Time, decay float64) string {
	p := r.Plant
	switch {
	case r.Finished && p.LastTended.IsZero():
		return "under glass"
	case r.Finished:
		return "under glass · tended " + ago(now.Sub(p.LastTended))
	case p.LastTended.IsZero():
		return "no activity yet"
	case garden.Health(p, now, decay) < wiltThreshold:
		return fmt.Sprintf("wilting · %dd quiet", int(now.Sub(p.LastTended).Hours()/24))
	}
	return "thriving · tended " + ago(now.Sub(p.LastTended))
}

func ciLine(r Repo) scene.CardLine {
	on := ""
	if r.Branch != "" {
		on = " on " + r.Branch
	}
	l := scene.CardLine{Label: "CI"}
	switch r.CI {
	case CIPassing:
		l.Value, l.Tone = "✓ passing"+on, scene.ToneGood
	case CIFailing:
		l.Value, l.Tone = "✗ failing"+on, scene.ToneBad
	case CIPending:
		l.Value = "● running" + on
	default:
		l.Value = "– no checks"
	}
	return l
}

// prLines are the PR summary and up to two PRs, oldest first.
func prLines(d Detail, now time.Time) []rankedLine {
	prs := slices.Clone(d.PRs)
	sort.SliceStable(prs, func(i, j int) bool { return prs[i].At.Before(prs[j].At) })
	sum := scene.CardLine{Label: "PRs", Value: "none open"}
	if len(prs) > 0 {
		sum.Value = fmt.Sprintf("%d open · oldest %s", len(prs), age(now.Sub(prs[0].At)))
	}
	if d.Drafts > 0 {
		sum.Value += fmt.Sprintf(" · +%d draft", d.Drafts)
	}
	out := []rankedLine{{sum, dropPRs}}
	for i, pr := range prs {
		if i == 2 {
			out = append(out, rankedLine{scene.CardLine{Sub: true, Value: fmt.Sprintf("+%d more", len(prs)-2)}, dropPRTitles})
			break
		}
		out = append(out, rankedLine{scene.CardLine{Sub: true, Value: fmt.Sprintf("#%d %s", pr.Number, pr.Title),
			Right: age(now.Sub(pr.At))}, dropPRTitles})
	}
	return out
}

// issueLines are the issue summary and the newest open issue.
func issueLines(d Detail, now time.Time) []rankedLine {
	l := scene.CardLine{Label: "issues", Value: "none open"}
	if d.OpenIssues == 0 {
		return []rankedLine{{l, dropIssues}}
	}
	l.Value = fmt.Sprintf("%d open", d.OpenIssues)
	if d.NewestIssue.Number == 0 {
		return []rankedLine{{l, dropIssues}}
	}
	l.Value += " · newest " + ago(now.Sub(d.NewestIssue.At))
	title := scene.CardLine{Sub: true, Value: fmt.Sprintf("#%d %s", d.NewestIssue.Number, d.NewestIssue.Title)}
	return []rankedLine{{l, dropIssues}, {title, dropIssueTitle}}
}

// drop removes the last of the highest-ranked lines that may go, and
// reports false when only lines that always stay are left.
func drop(lines *[]rankedLine) bool {
	top, at := keep, -1
	for i, l := range *lines {
		if l.rank > keep && l.rank >= top {
			top, at = l.rank, i
		}
	}
	if at < 0 {
		return false
	}
	*lines = append((*lines)[:at], (*lines)[at+1:]...)
	return true
}

// Sparkline draws daily counts as block heights scaled to the busiest day;
// a day without commits is a dot.
func Sparkline(days [14]int) string {
	blocks := []rune("▁▂▃▄▅▆▇█")
	top := 0
	for _, n := range days {
		top = max(top, n)
	}
	var b strings.Builder
	for _, n := range days {
		if n <= 0 {
			b.WriteRune('·')
			continue
		}
		b.WriteRune(blocks[(n*len(blocks)-1)/top])
	}
	return b.String()
}

func total(days [14]int) int {
	n := 0
	for _, d := range days {
		n += d
	}
	return n
}

// age is how long ago, in the ticker's short form: 2h, 10d, just now.
func age(d time.Duration) string { return strings.TrimSuffix(ago(d), " ago") }
