package live

import (
	"fmt"
	"sort"
	"time"

	"github.com/mattn/go-runewidth"

	"github.com/RursusAeternum/GitAGarden/internal/garden"
)

// Item is one message the ticker cycles through.
type Item struct{ Icon, Text string }

const (
	wiltThreshold = 0.5                // plants below this health get called out
	itemEvery     = 4 * time.Second    // how long each ticker item shows
	waitingAge    = 7 * 24 * time.Hour // PRs open longer than this are waiting on you
)

// Items lists what needs attention, most urgent first: failing CI, then PRs
// waiting over a week, then wilting plants (most wilted first), then other
// open PRs, then bursts of new issues. When nothing needs attention it
// returns one calm line.
func Items(repos []Repo, now time.Time, decay float64, commits7d int) []Item {
	type wilting struct {
		item   Item
		health float64
	}
	var failing, waiting, open, weeds []Item
	var ws []wilting
	for _, r := range repos {
		if r.Finished {
			continue
		}
		if r.CI == CIFailing {
			failing = append(failing, Item{"⚡", r.Name + ": CI failing" + onBranch(r.Branch)})
		}
		if n, oldest := waitingPRs(r.PRs, now); n > 0 {
			waiting = append(waiting, Item{"🌷", fmt.Sprintf("%s: %s waiting (%dd)", r.Name, plural(n, "PR"), int(oldest.Hours()/24))})
		} else if len(r.PRs) > 0 {
			open = append(open, Item{"🌷", fmt.Sprintf("%s: %s open", r.Name, plural(len(r.PRs), "PR"))})
		}
		if !r.Plant.LastTended.IsZero() {
			if h := garden.Health(r.Plant, now, decay); h < wiltThreshold {
				idle := int(now.Sub(r.Plant.LastTended).Hours() / 24)
				ws = append(ws, wilting{Item{"🥀", fmt.Sprintf("%s: %dd quiet", r.Name, idle)}, h})
			}
		}
		if r.NewIssues >= snailIssues {
			weeds = append(weeds, Item{"🐌", fmt.Sprintf("%s: %d new issues this week", r.Name, r.NewIssues)})
		}
	}
	sort.SliceStable(ws, func(i, j int) bool { return ws[i].health < ws[j].health })
	items := append(failing, waiting...)
	for _, w := range ws {
		items = append(items, w.item)
	}
	items = append(append(items, open...), weeds...)
	if len(items) == 0 {
		items = append(items, calm(commits7d))
	}
	return items
}

// waitingPRs counts PRs open longer than waitingAge, and the oldest one's age.
func waitingPRs(opened []time.Time, now time.Time) (n int, oldest time.Duration) {
	for _, at := range opened {
		if age := now.Sub(at); age > waitingAge {
			n++
			oldest = max(oldest, age)
		}
	}
	return n, oldest
}

func onBranch(branch string) string {
	if branch == "" {
		return ""
	}
	return " on " + branch
}

func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

func calm(commits7d int) Item {
	switch commits7d {
	case 0:
		return Item{"🌱", "All quiet in the garden"}
	case 1:
		return Item{"🌱", "Garden thriving: 1 commit this week"}
	}
	return Item{"🌱", fmt.Sprintf("Garden thriving: %d commits this week", commits7d)}
}

// Line renders the ticker in exactly width cells: the current item on the
// left (a new one every itemEvery) and status on the right.
func Line(items []Item, elapsed time.Duration, status string, width int) string {
	left := ""
	if len(items) > 0 {
		it := items[int(elapsed/itemEvery)%len(items)]
		left = " " + it.Icon + " " + it.Text
	}
	right := ""
	if status != "" {
		right = status + " "
	}
	return fit(left, right, width)
}

// fit lays out left and right text in exactly width terminal cells,
// truncating the left side first.
func fit(left, right string, width int) string {
	if width <= 0 {
		return ""
	}
	rw := runewidth.StringWidth(right)
	if rw >= width {
		return runewidth.FillRight(runewidth.Truncate(right, width, ""), width)
	}
	left = runewidth.Truncate(left, width-rw, "…")
	return runewidth.FillRight(left, width-rw) + right
}

// Status is the ticker's right side: how fresh the data is, or why it's
// stale.
func Status(s Snapshot, now time.Time, loading, failed bool) string {
	switch {
	case s.Note != "":
		return s.Note
	case s.FetchedAt.IsZero():
		if loading {
			return "loading…"
		}
		return ""
	case failed:
		return "refresh failed · data from " + ago(now.Sub(s.FetchedAt))
	case s.Offline:
		return "offline · cached " + ago(now.Sub(s.FetchedAt))
	case loading:
		return "refreshing…"
	}
	return "updated " + ago(now.Sub(s.FetchedAt))
}

func ago(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}
