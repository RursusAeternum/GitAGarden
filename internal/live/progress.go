package live

import "strings"

// Progress is how far a load has come.
type Progress struct {
	Done, Total int    // repos fetched so far, out of Total (0 while repos are still being listed)
	Current     string // the repo being fetched now
}

// Bar draws a progress bar width cells wide.
func Bar(done, total, width int) string {
	if width <= 0 {
		return ""
	}
	filled := 0
	if total > 0 {
		filled = min(width, width*done/total)
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}
