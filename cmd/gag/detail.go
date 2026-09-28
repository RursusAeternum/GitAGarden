package main

import (
	"sort"
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/github"
	"github.com/RursusAeternum/GitAGarden/internal/live"
)

// detailFor is what r's detail card shows, as of now.
func detailFor(r *github.Repo, now time.Time) live.Detail {
	d := live.Detail{FullName: r.NameWithOwner, Language: r.Language}
	var times []time.Time
	for _, c := range r.Commits {
		times = append(times, c.At)
		if c.At.After(d.LastCommit.At) {
			d.LastCommit = live.Entry{Title: c.Message, At: c.At, By: c.Agent}
		}
	}
	d.Daily = live.DailyCommits(times, now)
	for _, pr := range r.OpenPRs {
		if pr.Draft {
			d.Drafts++
			continue
		}
		d.PRs = append(d.PRs, live.Entry{Number: pr.Number, Title: pr.Title, At: pr.CreatedAt})
	}
	sort.SliceStable(d.PRs, func(i, j int) bool { return d.PRs[i].At.Before(d.PRs[j].At) })
	for _, pr := range r.PRs {
		if pr.MergedAt.After(d.LastMerge.At) {
			d.LastMerge = live.Entry{Number: pr.Number, Title: pr.Title, At: pr.MergedAt}
		}
	}
	for _, is := range r.Issues {
		if is.ClosedAt != nil {
			if is.ClosedAt.After(d.LastClosed.At) {
				d.LastClosed = live.Entry{Number: is.Number, Title: is.Title, At: *is.ClosedAt}
			}
			continue
		}
		d.OpenIssues++
		if is.CreatedAt.After(d.NewestIssue.At) {
			d.NewestIssue = live.Entry{Number: is.Number, Title: is.Title, At: is.CreatedAt}
		}
	}
	for _, rel := range r.Releases {
		if rel.CreatedAt.After(d.Release.At) {
			d.Release = live.Entry{Title: rel.Tag, At: rel.CreatedAt}
		}
	}
	return d
}
