package live

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/RursusAeternum/GitAGarden/internal/scene"
)

// pushedTo is r after new commits: its plant grown to pushes, the newest
// commit titled title and made by by (an agent, or "" for a person) at at.
func pushedTo(r Repo, pushes int, at time.Time, title, by string) Repo {
	out := grow(r.Name, pushes, r.Plant.Merges, at)
	out.Branch, out.CI, out.Stars, out.Finished = r.Branch, r.CI, r.Stars, r.Finished
	out.Detail = r.Detail
	out.Detail.LastCommit = Entry{Title: title, At: at, By: by}
	return out
}

func tickerOf(m Model) string {
	lines := strings.Split(m.View(), "\n")
	return visible(lines[len(lines)-1])
}

func TestAPushReactsAndNotes(t *testing.T) {
	now := t0
	m := ready(clocked(garden3(), &now), 100, 30)
	after := garden3()
	after.Repos[0] = pushedTo(after.Repos[0], 65, t0, "Draw the card beside the plant", "Claude")
	m, _ = step(m, loadedMsg{snap: after})
	if len(m.reacts) != 1 {
		t.Fatalf("reactions = %+v, want one", m.reacts)
	}
	r := m.reacts[0]
	if r.repo != "bloom" || r.anim.Kind != scene.ReactPush || !r.anim.Drone || !r.anim.Reveals ||
		!r.anim.Start.Equal(t0.Add(slowFrame)) || r.anim.Before == nil || r.anim.Before.Pushes != 60 {
		t.Errorf("reaction = %+v", r)
	}
	if m.frameInterval() != fastFrame {
		t.Error("a waiting reaction needs the fast frame rate")
	}
	if strings.Contains(tickerOf(m), "💧") {
		t.Error("the note should wait for its reaction to start")
	}
	now = now.Add(slowFrame)
	if want := `💧 bloom: "Draw the card beside the plant" · 5 commits · by Claude`; !strings.Contains(tickerOf(m), want) {
		t.Errorf("ticker = %q, want %q", tickerOf(m), want)
	}
	if v := m.sceneView(m.now()); len(v.Reactions) != 1 || v.Reactions[0].Plot != 1 {
		t.Errorf("the frame's reactions = %+v", v.Reactions)
	}
}

func TestQuietLoadsDontReact(t *testing.T) {
	m := ready(newModel(garden3(), nil, t0), 100, 30)
	offline := garden3()
	offline.Offline = true
	offline.Repos[0] = pushedTo(offline.Repos[0], 65, t0, "cached", "")
	if m, _ = step(m, loadedMsg{snap: offline}); len(m.reacts) != 0 {
		t.Error("an offline load should not react")
	}
	demo := garden3()
	demo.Demo, demo.Note = true, "demo · no GitHub token: run gh auth login"
	m = ready(newModel(demo, nil, t0), 100, 30)
	real := garden3()
	real.Repos[0] = pushedTo(real.Repos[0], 65, t0, "real", "")
	if m, _ = step(m, loadedMsg{snap: real}); len(m.reacts) != 0 {
		t.Error("signing in after the demo should not react")
	}
}

func TestAPlantsReactionsQueue(t *testing.T) {
	now := t0
	m := ready(clocked(garden3(), &now), 100, 30)
	after := garden3()
	r := pushedTo(after.Repos[0], 63, t0, "Speed up", "")
	r.Plant = grow("bloom", 63, 5, t0).Plant
	r.Detail.LastMerge = Entry{Number: 41, Title: "Add a detail card", At: t0.Add(-10 * time.Minute)} // merged before the push
	r.CI, r.Branch = CIFailing, "main"
	after.Repos[0] = r
	m, _ = step(m, loadedMsg{snap: after})
	want := []scene.ReactKind{scene.ReactPush, scene.ReactMerge, scene.ReactStorm}
	if len(m.reacts) != len(want) {
		t.Fatalf("reactions = %+v", m.reacts)
	}
	for i, w := range want {
		a := m.reacts[i].anim
		if a.Kind != w || a.Reveals != (i == 0) {
			t.Errorf("reaction %d: kind %v reveals %v, want %v and %v", i, a.Kind, a.Reveals, w, i == 0)
		}
		if i > 0 {
			prev := m.reacts[i-1].anim
			if gap := a.Start.Sub(prev.Start); gap < reactGap || gap < prev.Duration() {
				t.Errorf("reactions %d and %d start %v apart", i-1, i, gap)
			}
		}
	}
}

func TestCameraVisitsOffScreenPlants(t *testing.T) {
	now := t0
	m := ready(clocked(eight(), &now), 80, 24)
	after := eight()
	after.Repos[6] = pushedTo(after.Repos[6], 25, t0, "Far away", "")
	m, _ = step(m, loadedMsg{snap: after})
	if len(m.visits) != 1 || m.visits[0].repo != "p6" || len(m.reacts) != 1 {
		t.Fatalf("visits %+v, reactions %+v", m.visits, m.reacts)
	}
	vis := m.visits[0]
	if !m.reacts[0].anim.Start.Equal(vis.from.Add(scene.SlideFor)) {
		t.Errorf("the reaction should start as the camera arrives")
	}
	now = vis.from
	m, _ = step(m, tickMsg{})
	now = now.Add(scene.SlideFor)
	if on := scene.OnScreen(m.sceneView(m.now())); !contains(on, 6) {
		t.Errorf("during the visit the plants on screen are %v; want p6 among them", on)
	}
	held, _ := m.pan(m.layout())
	now = vis.until
	m, _ = step(m, tickMsg{})
	if len(m.visits) != 0 || m.cam.held {
		t.Errorf("after the visit: visits %+v, held %v", m.visits, m.cam.held)
	}
	if p, _ := m.pan(m.layout()); p != held {
		t.Errorf("automatic panning should resume from %v, not %v", held, p)
	}
}

func TestSelectionKeepsTheCameraStill(t *testing.T) {
	now := t0
	m := ready(clocked(eight(), &now), 80, 24)
	m, _ = step(m, arrow(tea.KeyRight)) // p0
	after := eight()
	after.Repos[6] = pushedTo(after.Repos[6], 25, t0, "Far away", "")
	m, _ = step(m, loadedMsg{snap: after})
	if len(m.visits) != 0 || len(m.reacts) != 1 || !m.reacts[0].anim.Start.Equal(t0.Add(slowFrame)) {
		t.Fatalf("visits %+v, reactions %+v", m.visits, m.reacts)
	}
	now = now.Add(slowFrame)
	if !strings.Contains(tickerOf(m), `💧 p6: "Far away"`) {
		t.Errorf("the note should still appear: %q", tickerOf(m))
	}
}

func TestSelectingDuringAVisitHoldsTheCamera(t *testing.T) {
	now := t0
	m := ready(clocked(eight(), &now), 80, 24)
	after := eight()
	after.Repos[6] = pushedTo(after.Repos[6], 25, t0, "Far away", "")
	m, _ = step(m, loadedMsg{snap: after})
	now = m.visits[0].from
	m, _ = step(m, tickMsg{})
	m, _ = step(m, arrow(tea.KeyLeft))
	name := m.sel.name
	m, _ = step(m, tickMsg{})
	if len(m.visits) != 0 || !m.cam.held || m.sel.name != name || name == "" {
		t.Errorf("visits %+v, held %v, selected %q", m.visits, m.cam.held, m.sel.name)
	}
}

func TestManyChangesSettle(t *testing.T) {
	now := t0
	m := ready(clocked(eight(), &now), 80, 24)
	after := eight()
	for i := range after.Repos {
		after.Repos[i] = pushedTo(after.Repos[i], 25, t0, fmt.Sprintf("change %d", i), "")
	}
	m, _ = step(m, loadedMsg{snap: after})
	if len(m.reacts) != 8 {
		t.Fatalf("%d reactions, want 8", len(m.reacts))
	}
	for s := 0; s < 60; s++ {
		now = now.Add(time.Second)
		m, _ = step(m, tickMsg{})
	}
	if len(m.visits) != 0 || m.cam.held || len(m.reacts) != 0 || m.reacting() {
		t.Errorf("after a minute: visits %+v, held %v, %d reactions", m.visits, m.cam.held, len(m.reacts))
	}
}

func TestReactionsFollowTheirPlant(t *testing.T) {
	now := t0
	m := ready(clocked(garden3(), &now), 100, 30)
	after := garden3()
	after.Repos[2] = pushedTo(after.Repos[2], 5, t0, "seedling", "")
	m, _ = step(m, loadedMsg{snap: after})
	fewer := after
	fewer.Repos = after.Repos[1:] // bloom leaves: seed moves from 3rd to 2nd
	m, _ = step(m, loadedMsg{snap: fewer})
	if v := m.sceneView(m.now()); len(v.Reactions) != 1 || v.Reactions[0].Plot != 2 {
		t.Errorf("the reaction should follow seed to plot 2: %+v", v.Reactions)
	}
}

func TestNoteTextKeepsTheTickerWhole(t *testing.T) {
	now := t0
	m := ready(clocked(garden3(), &now), 100, 30)
	after := garden3()
	title := "♻️ Refactor\tthe pan \U0001F3F4\U000E0067\U000E0062\U000E0073\U000E0063\U000E0074\U000E007F" + strings.Repeat(" very long", 20)
	after.Repos[0] = pushedTo(after.Repos[0], 61, t0, title, "")
	m, _ = step(m, loadedMsg{snap: after})
	now = now.Add(slowFrame)
	checkSize(t, m.View(), 100, 30)
}

func BenchmarkLiveFrameWithReactions(b *testing.B) {
	snap := garden3()
	for i := 0; len(snap.Repos) < 12; i++ {
		r := snap.Repos[i%3]
		r.Name += strings.Repeat("x", i+1)
		snap.Repos = append(snap.Repos, r)
	}
	m := ready(newModel(snap, nil, t0), 240, 65)
	for i, r := range m.repos {
		m.reacts = append(m.reacts, reaction{repo: r.Name, anim: scene.Reaction{Kind: scene.ReactKind(i % 7),
			Start: t0.Add(-time.Second), Before: grow(r.Name, 5, 0, t0).Plant, Reveals: true, Drone: i%2 == 0, Weeds: 1, Seed: int64(i)}})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.View()
	}
}

func TestCIComparesWithItsLastSettledState(t *testing.T) {
	now := t0
	withCI := func(ci ...CI) Snapshot {
		s := garden3()
		for i, c := range ci {
			s.Repos[i].CI, s.Repos[i].Branch = c, "main"
		}
		return s
	}
	m := ready(clocked(withCI(CIFailing, CIFailing, CIPassing), &now), 100, 30)
	// bloom: a fix runs, then passes. quiet: a failed check fetch in between.
	// seed: a run that fails.
	for _, snap := range []Snapshot{withCI(CIPending, CIUnknown, CIPending), withCI(CIPassing, CIFailing, CIFailing)} {
		m, _ = step(m, loadedMsg{snap: snap})
	}
	var got []string
	for _, r := range m.reacts {
		got = append(got, fmt.Sprintf("%s %v", r.repo, r.anim.Kind))
	}
	if want := fmt.Sprint([]string{fmt.Sprintf("bloom %v", scene.ReactClear), fmt.Sprintf("seed %v", scene.ReactStorm)}); fmt.Sprint(got) != want {
		t.Errorf("reactions = %v, want %v", got, want)
	}
}

func TestNoteTextKeepsCombiningMarks(t *testing.T) {
	for _, title := range []string{"नमस्ते दुनिया", "สวัสดีชาวโลก", "café au lait"} {
		now := t0
		m := ready(clocked(garden3(), &now), 100, 30)
		after := garden3()
		after.Repos[0] = pushedTo(after.Repos[0], 61, t0, title, "")
		m, _ = step(m, loadedMsg{snap: after})
		now = now.Add(slowFrame)
		if !strings.Contains(tickerOf(m), `"`+title+`"`) {
			t.Errorf("ticker = %q, want %q in it", tickerOf(m), title)
		}
		checkSize(t, m.View(), 100, 30)
	}
}

func TestVisitsGoInGardenOrder(t *testing.T) {
	now := t0
	m := ready(clocked(eight(), &now), 80, 24)
	on := scene.OnScreen(m.sceneView(m.now()))
	var off []int
	for i := range m.repos {
		if !contains(on, i) {
			off = append(off, i)
		}
	}
	if len(off) < 2 {
		t.Fatalf("need two plants off screen; on screen: %v", on)
	}
	a, b := off[0], off[len(off)-1]
	after := eight()
	after.Repos[a] = pushedTo(after.Repos[a], 25, t0, "earlier in the garden", "")
	after.Repos[b] = pushedTo(after.Repos[b], 25, t0, "pushed last", "")
	after.Repos[a], after.Repos[b] = after.Repos[b], after.Repos[a] // GitHub lists the newest push first
	m, _ = step(m, loadedMsg{snap: after})
	var got []string
	for _, v := range m.visits {
		got = append(got, v.repo)
	}
	if want := []string{m.repos[a].Name, m.repos[b].Name}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("visits = %v, want %v", got, want)
	}
}

func TestSelectedRepoLeavingStillVisits(t *testing.T) {
	now := t0
	m := ready(clocked(eight(), &now), 80, 24)
	m, _ = step(m, arrow(tea.KeyRight))
	gone := m.sel.name
	after := eight()
	var kept []Repo
	for i, r := range after.Repos {
		if r.Name != gone {
			kept = append(kept, pushedTo(r, 25, t0, fmt.Sprintf("change %d", i), ""))
		}
	}
	after.Repos = kept
	m, _ = step(m, loadedMsg{snap: after})
	if m.sel.name != "" || len(m.visits) == 0 {
		t.Errorf("selected %q, visits %+v: with the selection gone the camera should visit", m.sel.name, m.visits)
	}
}

func TestResizeDuringAVisitKeepsThePlantInView(t *testing.T) {
	for _, size := range [][2]int{{60, 24}, {100, 24}, {60, 50}, {35, 50}, {60, 75}} { // a bed is 24 rows
		now := t0
		m := ready(clocked(eight(), &now), 80, 24)
		after := eight()
		after.Repos[6] = pushedTo(after.Repos[6], 25, t0, "Far away", "")
		m, _ = step(m, loadedMsg{snap: after})
		if len(m.visits) != 1 {
			t.Fatalf("visits %+v", m.visits)
		}
		vis := m.visits[0]
		now = vis.from
		m, _ = step(m, tickMsg{})
		now = now.Add(scene.SlideFor)
		m, _ = step(m, tickMsg{})
		m, _ = step(m, tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		now = now.Add(scene.SlideFor)
		m, _ = step(m, tickMsg{})
		if on := scene.OnScreen(m.sceneView(m.now())); now.Before(vis.until) && !contains(on, 6) {
			t.Errorf("%dx%d mid-visit: on screen %v, want p6 among them", size[0], size[1], on)
		}
	}
}

// visitNames lists the plants the camera will visit, in order.
func visitNames(m Model) []string {
	var out []string
	for _, v := range m.visits {
		out = append(out, v.repo)
	}
	return out
}

func TestChangesWaitForTheVisitChain(t *testing.T) {
	now := t0
	m := ready(clocked(eight(), &now), 80, 24)
	near := scene.OnScreen(m.sceneView(m.now()))[0]
	first := eight()
	first.Repos[6] = pushedTo(first.Repos[6], 25, t0, "far away", "")
	m, _ = step(m, loadedMsg{snap: first})
	second := first
	second.Repos = append([]Repo(nil), first.Repos...)
	second.Repos[near] = pushedTo(second.Repos[near], 25, t0, "on screen now", "")
	m, _ = step(m, loadedMsg{snap: second})
	name := m.repos[near].Name
	if want := []string{"p6", name}; fmt.Sprint(visitNames(m)) != fmt.Sprint(want) {
		t.Fatalf("visits = %v, want %v", visitNames(m), want)
	}
	for _, r := range m.reacts {
		if r.repo == name && r.anim.Start.Before(m.visits[0].until) {
			t.Errorf("%s reacts at %v, before the camera is done with p6 at %v", name, r.anim.Start, m.visits[0].until)
		}
	}
}

func TestVisitsWaitForReactionsOnScreen(t *testing.T) {
	now := t0
	m := ready(clocked(eight(), &now), 80, 24)
	near := scene.OnScreen(m.sceneView(m.now()))[0]
	first := eight()
	r := pushedTo(first.Repos[near], 25, t0, "on screen", "")
	r.CI, r.Branch = CIFailing, "main" // a push, then a storm
	first.Repos[near] = r
	m, _ = step(m, loadedMsg{snap: first})
	busy := m.lastEnd() // its push and storm
	now = now.Add(time.Second)
	second := first
	second.Repos = append([]Repo(nil), first.Repos...)
	second.Repos[6] = pushedTo(second.Repos[6], 25, t0, "far away", "")
	m, _ = step(m, loadedMsg{snap: second})
	if len(m.visits) != 1 || m.visits[0].from.Before(busy) {
		t.Errorf("visits %+v: the camera should wait until %s is done at %v", m.visits, r.Name, busy)
	}
}

func TestAPlantKeepsOneVisit(t *testing.T) {
	now := t0
	m := ready(clocked(eight(), &now), 80, 24)
	first := eight()
	first.Repos[6] = pushedTo(first.Repos[6], 25, t0, "far away", "")
	m, _ = step(m, loadedMsg{snap: first})
	now = now.Add(time.Second)
	second := first
	second.Repos = append([]Repo(nil), first.Repos...)
	second.Repos[6] = pushedTo(first.Repos[6], 27, now, "further", "")
	m, _ = step(m, loadedMsg{snap: second})
	if len(m.visits) != 1 || len(m.reacts) != 2 {
		t.Fatalf("visits %v, %d reactions: want one visit for both", visitNames(m), len(m.reacts))
	}
	a, b := m.reacts[0].anim, m.reacts[1].anim
	if b.Start.Sub(a.Start) < reactGap || b.Start.Before(a.Start.Add(a.Duration())) {
		t.Errorf("the second push starts %v after the first", b.Start.Sub(a.Start))
	}
	if end := b.Start.Add(b.Duration()); m.visits[0].until.Before(end.Add(visitLinger)) {
		t.Errorf("the visit ends at %v, before the second push is done at %v", m.visits[0].until, end)
	}
}

func TestTheLastVisitsPlantKeepsItsVisit(t *testing.T) {
	now := t0
	m := ready(clocked(eight(), &now), 80, 24)
	first := eight()
	first.Repos[6] = pushedTo(first.Repos[6], 25, t0, "far away", "")
	m, _ = step(m, loadedMsg{snap: first})
	now = now.Add(time.Second)
	second := first
	second.Repos = append([]Repo(nil), first.Repos...)
	second.Repos[0] = pushedTo(second.Repos[0], 25, now, "earlier in the garden", "")
	second.Repos[6] = pushedTo(first.Repos[6], 27, now, "further", "")
	m, _ = step(m, loadedMsg{snap: second})
	if want := []string{"p6", "p0"}; fmt.Sprint(visitNames(m)) != fmt.Sprint(want) {
		t.Fatalf("visits = %v, want %v: p6's visit should stay longer, not come round again", visitNames(m), want)
	}
	for _, r := range m.reacts {
		if r.repo == "p6" && m.visits[0].until.Before(r.anim.Start.Add(r.anim.Duration()).Add(visitLinger)) {
			t.Errorf("p6's visit ends at %v, before its push at %v is done", m.visits[0].until, r.anim.Start)
		}
	}
}
