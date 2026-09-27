package scene

import (
	"time"

	"github.com/mattn/go-runewidth"

	"github.com/RursusAeternum/GitAGarden/internal/garden"
	"github.com/RursusAeternum/GitAGarden/internal/pixel"
)

// Bed geometry. A bed is one row of plots: sky headroom, the plants, their
// pots, and a strip of ground carrying name and status labels. Everything
// derives from the plant grid size, so it follows garden.Width/Height.
const (
	headroom   = 5                            // px of sky above the tallest plant
	plantBaseY = headroom + garden.Height - 1 // px row of a plant's ground row
	potTop     = plantBaseY + 1
	groundTop  = potTop + 5
	bedPx      = (groundTop + 7) / 2 * 2 // even, so a bed is whole terminal rows
	BedRows    = bedPx / 2               // terminal rows per bed
	BedCols    = garden.Width + 3        // terminal columns per plot
)

// Plot is one plant with what's needed to draw it.
type Plot struct {
	Plant        *garden.Plant
	Style        garden.Style
	Finished     bool
	Name, Status string
}

var (
	nameColor  = rgb(240, 232, 214)
	glassLabel = rgb(150, 215, 230)
	sadLabel   = rgb(200, 120, 90)
	happyLabel = rgb(150, 220, 130)
)

// PerRow is how many plots fit side by side in cols terminal columns.
func PerRow(cols int) int { return max(1, cols/BedCols) }

// Compose draws a static garden as tall as its beds need: every plot has a
// place and nothing moves. Used for one-shot prints and replay.
func Compose(cols int, plots []Plot, t time.Time, seed int64) *pixel.Canvas {
	return Draw(View{Cols: cols, Plots: plots, Now: t, Seed: seed})
}

// drawPlot draws one plot centered on column cx of the bed whose top is
// pixel row oy.
func drawPlot(c *pixel.Canvas, v View, pl Plot, cx, oy int) {
	DrawPot(c, cx, oy+potTop)
	garden.Paint(c, pl.Plant, cx, oy+plantBaseY, pl.Style)
	if pl.Finished {
		DrawCloche(c, cx, oy+2, oy+potTop+PotH-1, BedCols-3)
		if v.Motion {
			DrawGlint(c, v.Now, cx, oy+2, oy+potTop+PotH-1, BedCols-3, nameSeed(pl.Name))
		}
	}
	labelRow := oy/2 + BedRows - 2
	label(c, cx, labelRow, pl.Name, nameColor)
	label(c, cx, labelRow+1, pl.Status, statusColor(pl))
}

// nameSeed turns a plot name into a stable seed, so each cloche glints on
// its own schedule.
func nameSeed(name string) int64 {
	var h int64
	for _, r := range name {
		h = h*31 + int64(r)
	}
	return h
}

func statusColor(pl Plot) pixel.RGB {
	if pl.Finished {
		return glassLabel
	}
	return pixel.Lerp(sadLabel, happyLabel, pl.Style.Health)
}

// label centers s on column cx, truncated to fit a bed. Runes that don't
// take exactly one terminal cell become '?', so they can't shift the row.
func label(c *pixel.Canvas, cx, row int, s string, fg pixel.RGB) {
	var rs []rune
	for _, r := range s {
		if runewidth.RuneWidth(r) != 1 {
			r = '?'
		}
		rs = append(rs, r)
	}
	if limit := BedCols - 2; len(rs) > limit {
		rs = append(rs[:limit-1], '…')
	}
	c.Text(cx-len(rs)/2, row, string(rs), fg)
}
