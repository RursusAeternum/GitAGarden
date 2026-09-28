package live

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/RursusAeternum/GitAGarden/internal/scene"
)

func arrow(k tea.KeyType) tea.KeyMsg { return tea.KeyMsg{Type: k} }

func click(col, row int) tea.MouseMsg {
	return tea.MouseMsg{X: col, Y: row, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress}
}

// eight is a garden that pans at 80×24: three plants fit side by side.
func eight() Snapshot {
	var s Snapshot
	for i := 0; i < 8; i++ {
		s.Repos = append(s.Repos, grow(fmt.Sprintf("p%d", i), 20, 0, t0.Add(-time.Hour)))
	}
	s.FetchedAt = t0
	return s
}

func clocked(snap Snapshot, now *time.Time) Model {
	return New(Config{
		Load:      func(context.Context, func(Progress)) (Snapshot, error) { return snap, nil },
		DecayDays: 45,
		Now:       func() time.Time { return *now },
	})
}

// In garden3 at 100×30 the slots are columns 11-36 (bloom), 37-62 (quiet)
// and 63-88 (seed); the bed fills rows 5-28 and the ticker is row 29.

func TestArrowsStepThroughThePlants(t *testing.T) {
	m := ready(newModel(garden3(), nil, t0), 100, 30)
	var got []string
	for _, k := range []tea.KeyType{tea.KeyRight, tea.KeyRight, tea.KeyRight, tea.KeyRight, tea.KeyLeft, tea.KeyLeft} {
		m, _ = step(m, arrow(k))
		got = append(got, m.sel.name)
	}
	if want := "bloom quiet seed bloom seed quiet"; strings.Join(got, " ") != want {
		t.Errorf("→→→→←← selected %q, want %q", strings.Join(got, " "), want)
	}
	m = ready(newModel(garden3(), nil, t0), 100, 30)
	if m, _ = step(m, arrow(tea.KeyLeft)); m.sel.name != "seed" {
		t.Errorf("← with nothing selected picked %q, want the last plant on screen", m.sel.name)
	}
}

func TestClicksSelectAndClear(t *testing.T) {
	m := ready(newModel(garden3(), nil, t0), 100, 30)
	for _, c := range []struct {
		col, row int
		want     string
	}{
		{24, 15, "bloom"}, {50, 2, "quiet"}, {80, 28, "seed"}, // a plant, the sky above the bed, a label
		{5, 15, ""},                     // open ground beside the plants
		{50, 15, "quiet"}, {50, 29, ""}, // the ticker
	} {
		m, _ = step(m, click(c.col, c.row))
		if m.sel.name != c.want {
			t.Errorf("click at %d,%d selected %q, want %q", c.col, c.row, m.sel.name, c.want)
		}
	}
	m, _ = step(m, tea.MouseMsg{X: 24, Y: 15, Button: tea.MouseButtonRight, Action: tea.MouseActionPress})
	if m.sel.name != "" {
		t.Error("a right click should do nothing")
	}
}

func TestEnterOpensTheCardAndEscBacksOut(t *testing.T) {
	m := ready(newModel(garden3(), nil, t0), 100, 30)
	if m, _ = step(m, arrow(tea.KeyEnter)); m.sel.card {
		t.Error("enter with nothing selected opened a card")
	}
	m, _ = step(m, click(24, 15))
	m, _ = step(m, arrow(tea.KeyEnter))
	if !strings.Contains(visible(m.View()), "┌ bloom") {
		t.Fatalf("enter should open bloom's card:\n%s", visible(m.View()))
	}
	checkSize(t, m.View(), 100, 30)
	m, _ = step(m, arrow(tea.KeyRight))
	if !strings.Contains(visible(m.View()), "┌ quiet") {
		t.Error("→ should move the open card to quiet")
	}
	m, cmd := step(m, arrow(tea.KeyEscape))
	if cmd != nil || m.sel.card || m.sel.name != "quiet" {
		t.Errorf("the first esc should only close the card: card %v, selected %q", m.sel.card, m.sel.name)
	}
	m, cmd = step(m, arrow(tea.KeyEscape))
	if cmd != nil || m.sel.name != "" {
		t.Errorf("the second esc should only clear the selection, selected %q", m.sel.name)
	}
	_, cmd = step(m, arrow(tea.KeyEscape))
	if cmd == nil {
		t.Fatal("esc with nothing selected should quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("esc should send tea.QuitMsg")
	}
}

func TestClickOnTheCardDoesNothing(t *testing.T) {
	m := ready(newModel(garden3(), nil, t0), 100, 30)
	m, _ = step(m, click(24, 15))
	m, _ = step(m, arrow(tea.KeyEnter))
	x, y, _, _, ok := scene.CardRect(m.sceneView(m.now()))
	if !ok {
		t.Fatal("no card on screen")
	}
	m, _ = step(m, click(x+2, y+1))
	if m.sel.name != "bloom" || !m.sel.card {
		t.Errorf("a click on the card changed the selection to %q, card %v", m.sel.name, m.sel.card)
	}
}

func TestSelectionClearsAfterAMinute(t *testing.T) {
	now := t0
	m := ready(clocked(garden3(), &now), 100, 30)
	m, _ = step(m, click(24, 15))
	m, _ = step(m, arrow(tea.KeyEnter))
	now = now.Add(59 * time.Second)
	m, _ = step(m, tickMsg{})
	if m.sel.name != "bloom" || !m.sel.card {
		t.Fatal("the selection cleared before a minute was up")
	}
	now = now.Add(2 * time.Second)
	m, _ = step(m, tickMsg{})
	if m.sel.name != "" || m.sel.card {
		t.Error("a minute without input should clear the selection and close the card")
	}
}

func TestSelectionFollowsItsRepo(t *testing.T) {
	m := ready(newModel(garden3(), nil, t0), 100, 30)
	m, _ = step(m, click(50, 15)) // quiet
	more := garden3()
	more.Repos = append([]Repo{grow("newcomer", 5, 0, t0)}, more.Repos...)
	m, _ = step(m, loadedMsg{snap: more})
	if i := m.selected(); i < 0 || m.repos[i].Name != "quiet" {
		t.Errorf("after a refresh the selection is %d, want quiet", i)
	}
	gone := garden3()
	gone.Repos = append(gone.Repos[:1], gone.Repos[2:]...)
	m, _ = step(m, loadedMsg{snap: gone})
	if m.sel.name != "" {
		t.Error("the selection should clear when its repo leaves the garden")
	}
}

func TestCameraHoldsOnTheSelection(t *testing.T) {
	now := t0
	m := ready(clocked(eight(), &now), 80, 24)
	panNow := func() float64 { p, _ := m.pan(m.layout()); return p }
	m, _ = step(m, arrow(tea.KeyRight)) // p0, on screen
	m, _ = step(m, arrow(tea.KeyLeft))  // wraps to p7, off screen
	if m.sel.name != "p7" {
		t.Fatalf("selected %q, want p7", m.sel.name)
	}
	if m.frameInterval() != fastFrame {
		t.Error("the camera's slide needs the fast frame rate")
	}
	now = now.Add(scene.SlideFor)
	if p := panNow(); p != 7 {
		t.Errorf("after the slide the camera is at %v, want 7 (p7 on the left)", p)
	}
	now = now.Add(50 * time.Second) // longer than an automatic pan's hold
	if p := panNow(); p != 7 {
		t.Errorf("a selection should hold the camera; it moved to %v", p)
	}
	m, _ = step(m, arrow(tea.KeyEscape))
	if p := panNow(); p != 7 {
		t.Errorf("automatic panning should resume from 7, not jump to %v", p)
	}
	now = now.Add(20 * time.Second)
	if p := panNow(); p != 7 {
		t.Errorf("20 s after resuming the camera is at %v; want it still resting on 7", p)
	}
	now = now.Add(15 * time.Second)
	if p := panNow(); p == 7 {
		t.Error("automatic panning should carry on after the hold")
	}
}

func TestFrameFitsWithACardOpen(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {24, 12}, {240, 65}} {
		m := ready(newModel(eight(), nil, t0), size[0], size[1])
		m, _ = step(m, arrow(tea.KeyRight))
		m, _ = step(m, arrow(tea.KeyEnter))
		if !m.sel.card {
			t.Fatalf("%dx%d: no card open", size[0], size[1])
		}
		checkSize(t, m.View(), size[0], size[1])
	}
}

func TestResizeKeepsTheSelection(t *testing.T) {
	now := t0
	m := ready(clocked(eight(), &now), 80, 24)
	m, _ = step(m, arrow(tea.KeyLeft)) // the last plant on screen: p2
	m, _ = step(m, arrow(tea.KeyEnter))
	for _, size := range [][2]int{{240, 65}, {24, 12}, {80, 24}} {
		m, _ = step(m, tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		now = now.Add(scene.SlideFor)
		checkSize(t, m.View(), size[0], size[1])
		if m.sel.name != "p2" || !m.sel.card {
			t.Fatalf("%dx%d: selection %q, card %v", size[0], size[1], m.sel.name, m.sel.card)
		}
	}
	if on := scene.OnScreen(m.sceneView(m.now())); !slices.Contains(on, 2) {
		t.Errorf("back at 80×24 the plants on screen are %v; want p2 among them", on)
	}
}

func TestHelpShowsEveryKey(t *testing.T) {
	m := ready(newModel(garden3(), nil, t0), 80, 24)
	m, _ = step(m, key("?"))
	lines := strings.Split(m.View(), "\n")
	last := visible(lines[len(lines)-1])
	for _, k := range []string{"←/→ select", "enter details", "r refresh", "t ticker", "? help", "q quit"} {
		if !strings.Contains(last, k) {
			t.Errorf("the help line %q lacks %q", last, k)
		}
	}
}

func TestInputBeforeTheGardenLoads(t *testing.T) {
	m := New(Config{Now: func() time.Time { return t0 }})
	m, _ = step(m, tea.WindowSizeMsg{Width: 80, Height: 24})
	for _, msg := range []tea.Msg{arrow(tea.KeyRight), arrow(tea.KeyLeft), arrow(tea.KeyEnter), click(10, 10)} {
		m, _ = step(m, msg)
	}
	if m.sel.name != "" || m.sel.card {
		t.Errorf("input on the loading screen selected %q", m.sel.name)
	}
	checkSize(t, m.View(), 80, 24)
}

func TestLongSleepClosesTheCard(t *testing.T) {
	now := t0
	m := ready(clocked(garden3(), &now), 100, 30)
	m, _ = step(m, click(24, 15))
	m, _ = step(m, arrow(tea.KeyEnter))
	now = now.Add(10 * time.Hour) // the laptop slept with the card open
	m, _ = step(m, tickMsg{})
	if m.sel.name != "" || m.sel.card {
		t.Error("after a long sleep the card should be closed and the selection gone")
	}
	checkSize(t, m.View(), 100, 30)
}

func BenchmarkLiveFrameWithCard(b *testing.B) {
	snap := garden3()
	for i := 0; len(snap.Repos) < 12; i++ {
		r := snap.Repos[i%3]
		r.Name += strings.Repeat("x", i+1)
		snap.Repos = append(snap.Repos, r)
	}
	m := ready(newModel(snap, nil, t0), 240, 65)
	m, _ = step(m, arrow(tea.KeyRight))
	m, _ = step(m, arrow(tea.KeyEnter))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.View()
	}
}
