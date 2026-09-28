package live

import (
	"math"
	"time"
)

// Detail is what a repo's detail card shows beyond its plant.
type Detail struct {
	FullName    string // owner/name
	Language    string
	LastCommit  Entry   // the default branch's newest commit; zero when none
	Daily       [14]int // commits per local day, oldest first; the last is today
	PRs         []Entry // open non-draft PRs, oldest first
	Drafts      int     // open draft PRs
	OpenIssues  int
	NewestIssue Entry // the newest open issue; zero when none
	LastMerge   Entry // the newest merged PR; zero when none
	LastClosed  Entry // the most recently closed issue; zero when none
	Release     Entry // the latest release, its tag as Title; zero when none
}

// Entry is one dated thing on the card: a commit, PR, issue or release.
// Number is 0 for commits and releases.
type Entry struct {
	Number int
	Title  string
	At     time.Time
	By     string // the AI coding agent behind a commit; "" for people
}

// DailyCommits counts the commits at times per local calendar day for the
// 14 days ending today, oldest first. Older and future commits are left out.
func DailyCommits(times []time.Time, now time.Time) [14]int {
	var days [14]int
	loc := now.Location()
	midnight := func(t time.Time) time.Time {
		y, m, d := t.In(loc).Date()
		return time.Date(y, m, d, 0, 0, 0, 0, loc)
	}
	today := midnight(now)
	for _, t := range times {
		if t.After(now) {
			continue
		}
		back := int(math.Round(today.Sub(midnight(t)).Hours() / 24)) // DST days run 23 or 25 hours
		if back < len(days) {
			days[len(days)-1-back]++
		}
	}
	return days
}
