// Package live is the always-on garden: an animated, fullscreen view of the
// repos that reloads in the background and runs until you quit it.
package live

import (
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/garden"
)

// Repo is one plant in the live garden, with its signals.
type Repo struct {
	Name      string
	Plant     *garden.Plant
	Finished  bool
	Branch    string      // default branch, for "CI failing on main"
	CI        CI          // CI state of the default branch's latest commit
	PRs       []time.Time // when each open, non-draft PR was opened
	NewIssues int         // issues opened in the last 7 days
	Rising    bool        // more commits in the last 14 days than in the 14 before
	Stars     int         // GitHub stars
	Detail    Detail      // what the detail card shows
}

// CI is a repo's build state on its default branch.
type CI int

const (
	CIUnknown CI = iota // no checks, or not fetched yet
	CIPassing
	CIPending
	CIFailing
)

// Snapshot is one load of the garden's data.
type Snapshot struct {
	Repos     []Repo
	Commits7d int       // commits across all repos in the last 7 days
	FetchedAt time.Time // when the data was fetched (the oldest repo's fetch)
	Offline   bool      // GitHub was unreachable; this is cached data
	Note      string    // replaces the ticker's freshness status, e.g. demo mode
	Demo      bool      // fake demo repos: their stars are no baseline for real ones
}
