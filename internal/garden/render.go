package garden

import (
	"fmt"
	"hash/fnv"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// CardWidth is the rendered width of one plant including its glass casing.
const CardWidth = Width + 4

const defaultDecayDays = 45

type RenderOpts struct {
	Now       time.Time
	DecayDays float64 // days of neglect until fully wilted; 0 means default
	Finished  bool    // finished projects live under glass, preserved at their best
}

// Health is 1 for a recently tended plant, falling to 0 after DecayDays of
// neglect (with a two-day grace period).
func Health(p *Plant, now time.Time, decayDays float64) float64 {
	if p.LastTended.IsZero() {
		return 1
	}
	if decayDays <= 0 {
		decayDays = defaultDecayDays
	}
	idle := now.Sub(p.LastTended).Hours()/24 - 2
	if idle <= 0 {
		return 1
	}
	return math.Max(0, 1-idle/decayDays)
}

type rgb struct{ r, g, b float64 }

type stop struct {
	h float64
	c rgb
}

var (
	leafStops = []stop{{0, rgb{122, 82, 48}}, {0.3, rgb{200, 155, 60}}, {0.6, rgb{168, 192, 48}}, {1, rgb{95, 215, 95}}}
	bodyStops = []stop{{0, rgb{150, 125, 80}}, {0.4, rgb{140, 160, 70}}, {1, rgb{60, 170, 90}}}

	stemColor    = lipgloss.Color("#a0785a")
	deadStem     = lipgloss.Color("#6e5a4a")
	flowerColor  = lipgloss.Color("#ff87d7")
	wiltedFlower = lipgloss.Color("#8a6a6a")
	rotColor     = lipgloss.Color("#7a5230")
	fruitColor   = lipgloss.Color("#ff5f5f")
	starColor    = lipgloss.Color("#ffd75f")
	weedColor    = lipgloss.Color("#9a9a3a")
	potColor     = lipgloss.Color("#d7875f")
	glassColor   = lipgloss.Color("#6fa8b8")
	dimColor     = lipgloss.Color("#808080")

	weedGlyphs = []rune("ψw")
)

func shade(stops []stop, h float64) lipgloss.Color {
	c := stops[len(stops)-1].c
	for i := 1; i < len(stops); i++ {
		if h <= stops[i].h {
			a, b := stops[i-1], stops[i]
			t := (h - a.h) / (b.h - a.h)
			mix := func(x, y float64) float64 { return x + (y-x)*t }
			c = rgb{mix(a.c.r, b.c.r), mix(a.c.g, b.c.g), mix(a.c.b, b.c.b)}
			break
		}
	}
	return lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", int(c.r), int(c.g), int(c.b)))
}

// hash01 gives a stable pseudo-random value per cell, so the same leaves fall
// first every time rather than flickering between frames.
func hash01(name string, x, y int) float64 {
	h := fnv.New32a()
	fmt.Fprintf(h, "%s:%d:%d", name, x, y)
	return float64(h.Sum32()) / math.MaxUint32
}

func paint(c lipgloss.Color, s string) string {
	return lipgloss.NewStyle().Foreground(c).Render(s)
}

func (p *Plant) cellString(x, y int, h float64) string {
	c := p.Grid[y][x]
	switch c.Kind {
	case Leaf:
		if h < 0.45 && hash01(p.Name, x, y) < (0.45-h)/0.45*0.75 {
			return " " // dropped
		}
		st := lipgloss.NewStyle().Foreground(shade(leafStops, h)).Bold(c.Level > 0)
		return st.Render(string(c.Glyph))
	case Body:
		return paint(shade(bodyStops, h), string(c.Glyph))
	case Stem:
		if h < 0.3 {
			return paint(deadStem, string(c.Glyph))
		}
		return paint(stemColor, string(c.Glyph))
	case Flower:
		if h < 0.5 {
			return paint(wiltedFlower, ",")
		}
		return paint(flowerColor, string(c.Glyph))
	case Fruit:
		if h < 0.3 {
			return paint(rotColor, ".")
		}
		if c.Glyph == '✦' {
			return paint(starColor, string(c.Glyph))
		}
		return paint(fruitColor, string(c.Glyph))
	}
	return " "
}

// Render draws the plant, its pot, weeds for open issues and, for finished
// projects, a glass cloche. Every card has the same size so they tile.
func Render(p *Plant, o RenderOpts) string {
	h := Health(p, o.Now, o.DecayDays)
	if o.Finished {
		h = 1
	}

	weeds := map[int]bool{}
	for i := 0; i < p.OpenIssues && i < len(p.weedSlots); i++ {
		weeds[p.weedSlots[i]] = true
	}

	var rows []string
	for y := 0; y < Height; y++ {
		var sb strings.Builder
		for x := 0; x < Width; x++ {
			if y == ground && weeds[x] && p.Grid[y][x].Kind == Empty {
				sb.WriteString(paint(weedColor, string(weedGlyphs[x%len(weedGlyphs)])))
				continue
			}
			sb.WriteString(p.cellString(x, y, h))
		}
		rows = append(rows, sb.String())
	}
	rim := "  [" + strings.Repeat("=", Width-6) + "]  "
	pot := "   \\" + strings.Repeat("_", Width-8) + "/   "
	rows = append(rows, paint(potColor, rim), paint(potColor, pot))

	blank := strings.Repeat(" ", CardWidth)
	var out []string
	if o.Finished {
		g := func(s string) string { return paint(glassColor, s) }
		out = append(out,
			g("  ╭"+strings.Repeat("─", Width-2)+"╮  "),
			g(" ╱"+strings.Repeat(" ", Width)+"╲ "),
		)
		for _, r := range rows {
			out = append(out, g(" │")+r+g("│ "))
		}
		out = append(out, g(" ╘"+strings.Repeat("═", Width)+"╛ "))
	} else {
		out = append(out, blank, blank)
		for _, r := range rows {
			out = append(out, "  "+r+"  ")
		}
		out = append(out, blank)
	}
	return strings.Join(out, "\n")
}

// Card is a rendered plant with its name and status underneath.
func Card(p *Plant, o RenderOpts) string {
	name := []rune(p.Name)
	if len(name) > CardWidth {
		name = append(name[:CardWidth-1], '…')
	}
	var status string
	switch {
	case o.Finished:
		status = paint(glassColor, "✓ finished")
	case p.LastTended.IsZero():
		status = paint(dimColor, "untended")
	default:
		h := Health(p, o.Now, o.DecayDays)
		status = paint(shade(leafStops, h), ago(o.Now.Sub(p.LastTended))) +
			paint(dimColor, " · ") + paint(weedColor, issues(p.OpenIssues))
	}
	center := func(s string) string { return lipgloss.PlaceHorizontal(CardWidth, lipgloss.Center, s) }
	return lipgloss.JoinVertical(lipgloss.Left,
		Render(p, o),
		center(lipgloss.NewStyle().Bold(true).Render(string(name))),
		center(status),
	)
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
