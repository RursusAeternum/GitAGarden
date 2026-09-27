package main

import (
	"fmt"
	"io"

	"github.com/RursusAeternum/GitAGarden/internal/live"
)

// stderrProgress draws a one-line progress bar on w (stderr) while --once
// loads, and erases it when the load is done.
func stderrProgress(w io.Writer) func(done, total int, current string) {
	return func(done, total int, current string) {
		switch {
		case total == 0:
			fmt.Fprint(w, "\r\x1b[K🌱 finding your repos…")
		case done >= total:
			fmt.Fprint(w, "\r\x1b[K")
		default:
			fmt.Fprintf(w, "\r\x1b[K🌱 %s %d/%d  %s", live.Bar(done, total, 20), done, total, current)
		}
	}
}
