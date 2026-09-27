package live

import (
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/garden"
	"github.com/RursusAeternum/GitAGarden/internal/scene"
)

// snailIssues is how many issues opened in a week bring out the snail.
const snailIssues = 3

// Plot is how a repo looks at time at, without motion: health, blooms, buds,
// shoots, weather and snail. Finished repos rest under glass with none of
// these.
func Plot(r Repo, at time.Time, decay float64) scene.Plot {
	pl := scene.Plot{Plant: r.Plant, Finished: r.Finished, Name: r.Name,
		Status: garden.Status(r.Plant, at, r.Finished), Style: garden.Style{Health: 1, Now: at}}
	if r.Finished {
		return pl
	}
	pl.Style = garden.Style{Health: garden.Health(r.Plant, at, decay), Now: at, Buds: r.PRs, Rising: r.Rising}
	pl.Weather = weatherFor(r.CI)
	pl.Snail = r.NewIssues >= snailIssues
	return pl
}

func weatherFor(ci CI) scene.Weather {
	switch ci {
	case CIFailing:
		return scene.Storm
	case CIPending:
		return scene.Cloudy
	}
	return scene.Clear
}
