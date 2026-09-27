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
	wiltThreshold = 0.5             // plants below this health get called out
	itemEvery     = 4 * time.Second // how long each ticker item shows
)

// Items lists what needs attention, most urgent first. In this phase that's
// neglect: plants whose health fell below wiltThreshold, most wilted first.
// When nothing needs attention it returns one calm line.
func Items(repos []Repo, now time.Time, decay float64, commits7d int) []Item {
	type wilting struct {
		name   string
		health float64
		idle   time.Duration
	}
	var ws []wilting
	for _, r := range repos {
		if r.Finished || r.Plant.LastTended.IsZero() {
			continue
		}
		if h := garden.Health(r.Plant, now, decay); h < wiltThreshold {
			ws = append(ws, wilting{r.Name, h, now.Sub(r.Plant.LastTended)})
		}
	}
	sort.SliceStable(ws, func(i, j int) bool { return ws[i].health < ws[j].health })
	var items []Item
	for _, w := range ws {
		items = append(items, Item{"🥀", fmt.Sprintf("%s: %dd quiet", w.name, int(w.idle.Hours()/24))})
	}
	if len(items) == 0 {
		items = append(items, calm(commits7d))
	}
	return items
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
