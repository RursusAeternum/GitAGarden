package live

import (
	"context"
	"strings"
	"testing"
	"time"
	"unsafe"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/RursusAeternum/GitAGarden/internal/scene"
)

// timeRep mirrors time.Time's fields, to fake a machine that slept.
type timeRep struct {
	wall uint64
	ext  int64
	loc  *time.Location
}

// sleptFor is t after a sleep of wall on the wall clock during which the
// monotonic clock, as on macOS and Linux, advanced only mono.
func sleptFor(t time.Time, wall, mono time.Duration) time.Time {
	u := t.Add(mono)
	p := (*timeRep)(unsafe.Pointer(&u))
	p.wall += uint64((wall-mono)/time.Second) << 30
	return u
}

func TestSleepClearsTheSelectionOnARealClock(t *testing.T) {
	now := time.Now() // carries a monotonic reading, like production
	if w := sleptFor(now, 10*time.Hour, time.Second); w.Sub(now) != time.Second || w.Round(0).Sub(now.Round(0)) != 10*time.Hour {
		t.Skip("this Go's time.Time layout differs; can't fake a sleep")
	}
	m := ready(clocked(garden3(), &now), 100, 30)
	m, _ = step(m, click(24, 15))
	m, _ = step(m, arrow(tea.KeyEnter))
	now = sleptFor(now, 10*time.Hour, time.Second)
	m, _ = step(m, tickMsg{})
	if m.sel.name != "" || m.sel.card {
		t.Error("after 10 hours asleep the card should be closed and the selection gone")
	}
}

func TestZeroWidthTextKeepsTheFrameWhole(t *testing.T) {
	for _, title := range []string{
		"♻️ Refactor the pan", // an emoji with a variation selector
		"🏴\U000E0067\U000E0062\U000E0073\U000E0063\U000E0074\U000E007F flag", // a tag sequence
		"café and 👨‍💻 at work",                                              // a combining accent and a zero-width joiner
	} {
		snap := garden3()
		snap.Repos[0].Detail.LastCommit = Entry{Title: title, At: t0}
		m := ready(newModel(snap, nil, t0), 100, 30)
		m, _ = step(m, click(24, 15))
		m, _ = step(m, arrow(tea.KeyEnter))
		checkSize(t, m.View(), 100, 30)
	}
}

func TestRefreshThatMakesTheGardenPanHoldsTheCamera(t *testing.T) {
	now := t0
	cur := garden3()
	m := New(Config{
		Load:      func(context.Context, func(Progress)) (Snapshot, error) { return cur, nil },
		DecayDays: 45,
		Now:       func() time.Time { return now },
	})
	m = ready(m, 100, 30)
	m, _ = step(m, click(24, 15)) // bloom
	m, _ = step(m, arrow(tea.KeyEnter))
	nine := garden3()
	for _, r := range eight().Repos[:6] {
		nine.Repos = append(nine.Repos, r)
	}
	m, _ = step(m, loadedMsg{snap: nine}) // the garden now pans
	now = now.Add(45 * time.Second)
	if on := scene.OnScreen(m.sceneView(m.now())); len(on) == 0 || !contains(on, 0) {
		t.Errorf("45 s after the refresh the plants on screen are %v; bloom is selected and should stay", on)
	}
}

func TestSelectionStaysOnScreenWhenAnEarlierRepoLeaves(t *testing.T) {
	now := t0
	m := ready(clocked(eight(), &now), 80, 24)
	// ← picks p2; →→ slides the camera to show p2-p4; ←← comes back to p2.
	for _, k := range []tea.KeyType{tea.KeyLeft, tea.KeyRight, tea.KeyRight, tea.KeyLeft, tea.KeyLeft} {
		m, _ = step(m, arrow(k))
		now = now.Add(scene.SlideFor)
	}
	if on := scene.OnScreen(m.sceneView(m.now())); m.sel.name != "p2" || !contains(on, 2) {
		t.Fatalf("setup: selected %q, plants on screen %v", m.sel.name, on)
	}
	fewer := eight()
	fewer.Repos = fewer.Repos[1:] // p0 leaves: p2 moves from 2 to 1
	m, _ = step(m, loadedMsg{snap: fewer})
	now = now.Add(scene.SlideFor)
	i := m.selected()
	if on := scene.OnScreen(m.sceneView(m.now())); m.sel.name != "p2" || !contains(on, i) {
		t.Errorf("selected %q at %d; plants on screen %v", m.sel.name, i, on)
	}
}

func TestTickerToggleKeepsTheSelectionOnScreen(t *testing.T) {
	now := t0
	m := ready(clocked(eight(), &now), 80, 48)
	m, _ = step(m, key("t"))           // the ticker goes: two beds fit
	m, _ = step(m, arrow(tea.KeyLeft)) // the last plant on screen, in the second bed
	name := m.sel.name
	m, _ = step(m, key("?")) // the help line comes back: one bed
	now = now.Add(scene.SlideFor)
	if on := scene.OnScreen(m.sceneView(m.now())); m.sel.name != name || !contains(on, m.selected()) {
		t.Errorf("after ? the selection %q (%d) is off screen: %v", m.sel.name, m.selected(), on)
	}
}

func TestDraftsShowOnTheCard(t *testing.T) {
	snap := garden3()
	snap.Repos[0] = fullRepo()
	m := ready(newModel(snap, nil, t0), 100, 30)
	m, _ = step(m, click(24, 15))
	m, _ = step(m, arrow(tea.KeyEnter))
	if v := visible(m.View()); !strings.Contains(v, "+1 draft") {
		t.Errorf("the card should show the draft PR:\n%s", v)
	}
}

func contains(xs []int, x int) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
