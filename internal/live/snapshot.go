// Package live is the always-on garden: an animated, fullscreen view of the
// repos that reloads in the background and runs until you quit it.
package live

import (
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/garden"
)

// Repo is one plant in the live garden.
type Repo struct {
	Name     string
	Plant    *garden.Plant
	Finished bool
}

// Snapshot is one load of the garden's data.
type Snapshot struct {
	Repos     []Repo
	Commits7d int       // commits across all repos in the last 7 days
	FetchedAt time.Time // when the data was fetched (the oldest repo's fetch)
	Offline   bool      // GitHub was unreachable; this is cached data
	Note      string    // replaces the ticker's freshness status, e.g. demo mode
}
