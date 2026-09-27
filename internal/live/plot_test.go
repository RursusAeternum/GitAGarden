package live

import (
	"testing"
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/scene"
)

func TestPlotCarriesSignals(t *testing.T) {
	r := grow("x", 30, 2, t0.Add(-time.Hour))
	r.CI, r.PRs, r.NewIssues, r.Rising = CIFailing, []time.Time{t0}, 3, true
	pl := Plot(r, t0, 45)
	if pl.Weather != scene.Storm || !pl.Snail || len(pl.Style.Buds) != 1 || !pl.Style.Rising || !pl.Style.Now.Equal(t0) {
		t.Errorf("signals lost: %+v", pl)
	}
	r.CI = CIPending
	if Plot(r, t0, 45).Weather != scene.Cloudy {
		t.Error("running CI should be a grey cloud")
	}
	r.CI = CIUnknown
	if Plot(r, t0, 45).Weather != scene.Clear {
		t.Error("no CI should be clear skies")
	}
	r.CI, r.Finished = CIFailing, true
	pl = Plot(r, t0, 45)
	if pl.Weather != scene.Clear || pl.Snail || len(pl.Style.Buds) != 0 || pl.Style.Health != 1 {
		t.Errorf("finished repos rest under glass without signals: %+v", pl)
	}
}
