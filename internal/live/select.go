package live

import (
	"math"
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/scene"
)

// idleClear is how long a selection lasts without a key press or click.
const idleClear = time.Minute

// selection is the plant picked with the arrow keys or a click, by repo
// name, and whether its detail card is open.
type selection struct {
	name      string // "" for none
	card      bool
	lastInput time.Time // wall time of the last key press or click
}

// camera controls panning when the garden is wider than the window.
// Automatic panning runs on the session clock minus shift. A selection holds
// the camera, sliding it from → to when the selected plant is off screen.
type camera struct {
	shift     time.Duration
	held      bool
	from, to  float64
	slideFrom time.Time // wall time the hold's slide started
}

// selected is the index of the selected repo, or -1.
func (m Model) selected() int {
	if m.sel.name == "" {
		return -1
	}
	for i, r := range m.repos {
		if r.Name == m.sel.name {
			return i
		}
	}
	return -1
}

// choose selects repo i; an open card follows it.
func (m *Model) choose(i int) {
	m.sel.name = m.repos[i].Name
	m.holdOn(i)
}

// unselect clears the selection and closes the card, handing the camera
// back to automatic panning.
func (m *Model) unselect() {
	m.sel.name, m.sel.card = "", false
	m.release()
}

// stepSelection moves the selection dir (+1 or -1) plants in reading order,
// wrapping. With nothing selected it picks the first or last plant on
// screen.
func (m *Model) stepSelection(dir int) {
	if len(m.repos) == 0 || m.cols == 0 {
		return
	}
	order := scene.ReadingOrder(m.layout(), len(m.repos))
	cur := m.selected()
	if cur < 0 {
		on := scene.OnScreen(m.sceneView(m.now()))
		if len(on) == 0 {
			on = order
		}
		if dir > 0 {
			m.choose(on[0])
		} else {
			m.choose(on[len(on)-1])
		}
		return
	}
	for k, i := range order {
		if i == cur {
			m.choose(order[(k+dir+len(order))%len(order)])
			return
		}
	}
}

// click handles a left click at terminal cell (col, row). A plant selects
// it, the card ignores it, and anything else clears the selection.
func (m *Model) click(col, row int) {
	if len(m.repos) == 0 || m.cols == 0 {
		return
	}
	v := m.sceneView(m.now())
	if x, y, w, h, ok := scene.CardRect(v); ok && col >= x && col < x+w && row >= y && row < y+h {
		return
	}
	if row < m.gardenRows() {
		if i, ok := scene.PlotAt(v, col, row); ok {
			m.choose(i)
			return
		}
	}
	m.unselect()
}

// layout is how the garden fits the window right now.
func (m Model) layout() scene.Layout {
	return scene.LayoutFor(m.cols, m.gardenRows(), len(m.repos))
}

// pan is where the camera is now, and whether it's sliding or about to.
func (m Model) pan(lay scene.Layout) (float64, bool) {
	if !lay.Overflow {
		return 0, false
	}
	if !m.cam.held {
		e := m.elapsed() - m.cam.shift
		return scene.PanAt(e, lay.Columns), scene.Sliding(e) || scene.Sliding(e+slowFrame)
	}
	f := float64(m.cfg.Now().Sub(m.cam.slideFrom)) / float64(scene.SlideFor)
	if f >= 1 {
		return m.cam.to, false
	}
	return scene.SlidePan(m.cam.from, m.cam.to, f, lay.Columns), true
}

// holdOn stops automatic panning and, when plant i is off screen, slides the
// camera to the nearest position that shows it.
func (m *Model) holdOn(i int) {
	lay := m.layout()
	if !lay.Overflow {
		return
	}
	cur, _ := m.pan(lay)
	to := scene.PanShowing(cur, i/lay.Beds, lay.PerRow, lay.Columns)
	start := m.cfg.Now()
	if to == cur {
		start = start.Add(-scene.SlideFor) // already there: no slide
	}
	m.cam = camera{shift: m.cam.shift, held: true, from: cur, to: to, slideFrom: start}
}

// release hands the camera back to automatic panning, resuming from the
// column it rests on.
func (m *Model) release() {
	if !m.cam.held {
		return
	}
	col := 0
	if lay := m.layout(); lay.Overflow {
		p, _ := m.pan(lay)
		col = int(math.Round(p)) % lay.Columns
	}
	m.cam = camera{shift: m.elapsed() - scene.ResumeAt(col)}
}
