package garden

import (
	"fmt"
	"time"
)

// Status is the short line under a plant: "3d ago · 2 issues", "finished"
// or "untended".
func Status(p *Plant, now time.Time, finished bool) string {
	switch {
	case finished:
		return "finished"
	case p.LastTended.IsZero():
		return "untended"
	}
	return ago(now.Sub(p.LastTended)) + " · " + issues(p.OpenIssues)
}

func ago(d time.Duration) string {
	switch {
	case d < time.Hour:
		return "just now"
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}

func issues(n int) string {
	switch n {
	case 0:
		return "no issues"
	case 1:
		return "1 issue"
	}
	return fmt.Sprintf("%d issues", n)
}
