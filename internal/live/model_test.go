package live

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"

	"github.com/RursusAeternum/GitAGarden/internal/pixel"
	"github.com/RursusAeternum/GitAGarden/internal/scene"
)

var ansi = regexp.MustCompile("\x1b\\[[0-9;]*m")

func visible(s string) string { return ansi.ReplaceAllString(s, "") }

func garden3() Snapshot {
	return Snapshot{
		Repos: []Repo{
			grow("bloom", 60, 4, t0.Add(-time.Hour)),
			grow("quiet", 40, 0, t0.Add(-40*24*time.Hour)),
			grow("seed", 1, 0, t0.Add(-2*time.Hour)),
		},
		Commits7d: 5,
		FetchedAt: t0.Add(-2 * time.Minute),
	}
}

func newModel(snap Snapshot, err error, now time.Time) Model {
	return New(Config{
		Load:      func(context.Context, func(Progress)) (Snapshot, error) { return snap, err },
		DecayDays: 45,
		Profile:   pixel.TrueColor,
		Now:       func() time.Time { return now },
	})
}

func step(m Model, msg tea.Msg) (Model, tea.Cmd) {
	next, cmd := m.Update(msg)
	return next.(Model), cmd
}

// ready sizes the window and runs the first load to completion.
func ready(m Model, cols, rows int) Model {
	m, _ = step(m, tea.WindowSizeMsg{Width: cols, Height: rows})
	return finishLoad(m, m.load(false))
}

// finishLoad runs a load command and feeds its messages back until the
// load is done.
func finishLoad(m Model, cmd tea.Cmd) Model {
	for cmd != nil {
		msg := cmd()
		var next tea.Cmd
		m, next = step(m, msg)
		if _, done := msg.(loadedMsg); done {
			return m
		}
		cmd = next
	}
	return m
}

func key(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

func checkSize(t *testing.T, view string, cols, rows int) {
	t.Helper()
	lines := strings.Split(view, "\n")
	if len(lines) != rows {
		t.Fatalf("%dx%d: view has %d lines", cols, rows, len(lines))
	}
	for i, l := range lines {
		if w := runewidth.StringWidth(visible(l)); w != cols {
			t.Fatalf("%dx%d: line %d is %d cells wide", cols, rows, i, w)
		}
	}
}

func TestViewFitsTheWindow(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {120, 40}, {30, 12}, {240, 65}} {
		m := ready(newModel(garden3(), nil, t0), size[0], size[1])
		checkSize(t, m.View(), size[0], size[1])
	}
}

func TestTickerShowsWiltingAndFreshness(t *testing.T) {
	m := ready(newModel(garden3(), nil, t0), 100, 30)
	lines := strings.Split(m.View(), "\n")
	last := visible(lines[len(lines)-1])
	if !strings.Contains(last, "🥀 quiet: 40d quiet") || !strings.Contains(last, "updated 2m ago") {
		t.Errorf("ticker = %q", last)
	}
}

func TestResizeFollowsTheWindow(t *testing.T) {
	m := ready(newModel(garden3(), nil, t0), 80, 24)
	m, _ = step(m, tea.WindowSizeMsg{Width: 100, Height: 30})
	checkSize(t, m.View(), 100, 30)
	m, _ = step(m, tea.WindowSizeMsg{Width: 20, Height: 10})
	if !strings.Contains(m.View(), "bigger") {
		t.Error("tiny window should ask to be bigger")
	}
	m, _ = step(m, tea.WindowSizeMsg{Width: 90, Height: 26})
	checkSize(t, m.View(), 90, 26)
}

func TestFailedRefreshKeepsGarden(t *testing.T) {
	m := ready(newModel(garden3(), nil, t0), 100, 30)
	m, cmd := step(m, loadedMsg{err: errors.New("rate limited")})
	if cmd == nil {
		t.Error("a failed refresh should schedule the next one")
	}
	view := m.View()
	checkSize(t, view, 100, 30)
	if !strings.Contains(visible(view), "refresh failed · data from 2m ago") {
		t.Error("ticker should say the refresh failed")
	}
}

func TestFirstLoadFailureOffersRetry(t *testing.T) {
	m := ready(newModel(Snapshot{}, errors.New("no network"), t0), 80, 24)
	if v := m.View(); !strings.Contains(v, "couldn't load your garden") || !strings.Contains(v, "press r") {
		t.Errorf("view = %q", visible(v))
	}
}

func TestKeys(t *testing.T) {
	m := ready(newModel(garden3(), nil, t0), 80, 24)
	m, _ = step(m, key("t"))
	if v := visible(m.View()); strings.Contains(v, "updated") {
		t.Error("t should hide the ticker")
	}
	checkSize(t, m.View(), 80, 24)
	m, _ = step(m, key("?"))
	if v := visible(m.View()); !strings.Contains(v, "q quit") {
		t.Error("? should show the keys")
	}
	_, cmd := step(m, key("q"))
	if cmd == nil {
		t.Fatal("q should quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("q should send tea.QuitMsg")
	}
}

func TestStaleRefreshTimersAreIgnored(t *testing.T) {
	m := ready(newModel(garden3(), nil, t0), 80, 24) // one load done: gen 1
	if _, cmd := step(m, refreshMsg{gen: 0}); cmd != nil {
		t.Error("a timer from an older load should be ignored")
	}
	m, cmd := step(m, refreshMsg{gen: 1})
	if cmd == nil || !m.loading {
		t.Error("the current timer should start a load")
	}
}

func TestMergeKeepsFirstSeenOrder(t *testing.T) {
	a, b, c := Repo{Name: "a"}, Repo{Name: "b"}, Repo{Name: "c"}
	got := merge([]Repo{a, b}, []Repo{c, b, a})
	if len(got) != 3 || got[0].Name != "a" || got[1].Name != "b" || got[2].Name != "c" {
		t.Errorf("merge = %v", got)
	}
	if got := merge([]Repo{a, b}, []Repo{b}); len(got) != 1 || got[0].Name != "b" {
		t.Errorf("vanished repos should be dropped: %v", got)
	}
}

func TestFrameRate(t *testing.T) {
	if m := ready(newModel(garden3(), nil, t0), 80, 24); m.frameInterval() != fastFrame {
		t.Error("a blooming garden at noon has critters: want the fast frame rate")
	}
	night := t0.Add(11 * time.Hour) // 23:00
	if m := ready(newModel(garden3(), nil, night), 80, 24); m.frameInterval() != slowFrame {
		t.Error("at night nothing flies: want the slow frame rate")
	}
	if m := New(Config{Now: func() time.Time { return t0 }}); m.frameInterval() != slowFrame {
		t.Error("an empty garden should idle")
	}
}

func TestLongSleepKeepsDrawing(t *testing.T) {
	now := t0
	m := New(Config{
		Load:      func(context.Context, func(Progress)) (Snapshot, error) { return garden3(), nil },
		DecayDays: 45,
		Now:       func() time.Time { return now },
	})
	m = ready(m, 80, 24)
	now = t0.Add(10*24*time.Hour + 37*time.Minute) // the laptop slept for ten days
	checkSize(t, m.View(), 80, 24)
}

func BenchmarkLiveFrame(b *testing.B) {
	snap := garden3()
	for i := 0; len(snap.Repos) < 12; i++ {
		r := snap.Repos[i%3]
		r.Name += strings.Repeat("x", i+1)
		snap.Repos = append(snap.Repos, r)
	}
	m := ready(newModel(snap, nil, t0), 240, 65)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.View()
	}
}

func TestOfflineRefreshKeepsThePlantSet(t *testing.T) {
	m := ready(newModel(garden3(), nil, t0), 100, 30) // bloom, quiet, seed
	stranger, bloom := grow("stranger", 10, 0, t0), grow("bloom", 61, 4, t0)
	m, _ = step(m, loadedMsg{snap: Snapshot{Repos: []Repo{stranger, bloom}, Offline: true, FetchedAt: t0.Add(-time.Hour)}})
	var got []string
	for _, r := range m.repos {
		got = append(got, r.Name)
	}
	if strings.Join(got, ",") != "bloom,quiet,seed" {
		t.Errorf("plants = %v; offline data must not add, drop or reorder plants", got)
	}
	if m.repos[0].Plant.Pushes != 61 {
		t.Error("known plants should still be updated from offline data")
	}
}

func TestFirstLoadMayBeOffline(t *testing.T) {
	snap := garden3()
	snap.Offline = true
	if m := ready(newModel(snap, nil, t0), 80, 24); len(m.repos) != 3 {
		t.Errorf("an offline first load should still show the cached garden, got %d plants", len(m.repos))
	}
}

func TestManualRefreshForcesAFetch(t *testing.T) {
	var forced []bool
	m := New(Config{
		Load: func(ctx context.Context, _ func(Progress)) (Snapshot, error) {
			forced = append(forced, Forced(ctx))
			return garden3(), nil
		},
		DecayDays: 45,
		Now:       func() time.Time { return t0 },
	})
	m = ready(m, 80, 24)
	m, cmd := step(m, key("r"))
	cmd()
	m, cmd = step(m, refreshMsg{gen: m.gen})
	if cmd != nil {
		cmd()
	}
	if len(forced) < 2 || forced[0] || !forced[1] {
		t.Errorf("forced = %v, want the initial load unforced and r forced", forced)
	}
}

func TestLoadStreamsProgressThenGarden(t *testing.T) {
	m := New(Config{
		Load: func(_ context.Context, progress func(Progress)) (Snapshot, error) {
			progress(Progress{Done: 1, Total: 3, Current: "me/quiet"})
			return garden3(), nil
		},
		DecayDays: 45,
		Now:       func() time.Time { return t0 },
	})
	m, _ = step(m, tea.WindowSizeMsg{Width: 80, Height: 24})
	msg := m.load(false)()
	pm, ok := msg.(progressMsg)
	if !ok {
		t.Fatalf("first message = %T, want a progress update", msg)
	}
	m, cmd := step(m, pm)
	if v := visible(m.View()); !strings.Contains(v, "1/3") || !strings.Contains(v, "fetching me/quiet") {
		t.Errorf("loading screen = %q", v)
	}
	m = finishLoad(m, cmd)
	if len(m.repos) != 3 {
		t.Errorf("garden not loaded after progress: %d plants", len(m.repos))
	}
}

func TestLoadingScreenBeforeTheListIsKnown(t *testing.T) {
	m := newModel(garden3(), nil, t0)
	m, _ = step(m, tea.WindowSizeMsg{Width: 80, Height: 24})
	if v := visible(m.View()); !strings.Contains(v, "finding your repos") {
		t.Errorf("view = %q", v)
	}
}

func TestRefreshShowsProgressInTheTicker(t *testing.T) {
	m := ready(newModel(garden3(), nil, t0), 100, 30)
	m, _ = step(m, key("r"))
	m, _ = step(m, progressMsg{p: Progress{Done: 2, Total: 8}, ch: make(chan tea.Msg)})
	lines := strings.Split(m.View(), "\n")
	if last := visible(lines[len(lines)-1]); !strings.Contains(last, "refreshing") || !strings.Contains(last, "2/8") {
		t.Errorf("ticker = %q", last)
	}
}

func TestStormsRainAtFullFrameRate(t *testing.T) {
	snap := garden3()
	snap.Repos[2].CI = CIFailing
	night := t0.Add(11 * time.Hour) // 23:00: no critters
	if m := ready(newModel(snap, nil, night), 80, 24); m.frameInterval() != fastFrame {
		t.Error("rain falls at night too: want the fast frame rate")
	}
}

func TestWakingUpRefreshes(t *testing.T) {
	now := t0
	m := New(Config{
		Load:      func(context.Context, func(Progress)) (Snapshot, error) { return garden3(), nil },
		DecayDays: 45,
		Now:       func() time.Time { return now },
	})
	m = ready(m, 80, 24)
	m, _ = step(m, tickMsg{}) // the first tick notes the time
	now = now.Add(125 * time.Millisecond)
	if m, _ = step(m, tickMsg{}); m.loading {
		t.Fatal("an ordinary tick should not reload")
	}
	now = now.Add(9 * time.Hour) // the lid was closed overnight
	if m, _ = step(m, tickMsg{}); m.loading {
		t.Error("the refresh should wait a moment for the network to come back")
	}
	now = now.Add(11 * time.Second)
	if m, _ = step(m, tickMsg{}); !m.loading {
		t.Error("waking up should start a refresh once the network had a moment")
	}
}

func TestProgressShowsTheLatestRepo(t *testing.T) {
	release := make(chan struct{})
	m := New(Config{
		Load: func(_ context.Context, progress func(Progress)) (Snapshot, error) {
			for i := 0; i < 20; i++ { // a burst from cached repos, then one slow fetch
				progress(Progress{Done: i, Total: 21, Current: fmt.Sprintf("me/r%d", i)})
			}
			<-release
			return garden3(), nil
		},
		DecayDays: 45,
		Now:       func() time.Time { return t0 },
	})
	m, _ = step(m, tea.WindowSizeMsg{Width: 80, Height: 24})
	cmd := m.load(false)
	for {
		got := make(chan tea.Msg, 1)
		go func() { got <- cmd() }()
		select {
		case msg := <-got:
			pm, ok := msg.(progressMsg)
			if !ok {
				t.Fatalf("unexpected %T before the load was released", msg)
			}
			m, cmd = step(m, pm)
			if pm.p.Current == "me/r19" {
				close(release)
				finishLoad(m, cmd)
				return
			}
		case <-time.After(time.Second):
			close(release)
			t.Fatalf("the screen is stuck on %q; the latest repo never arrived", m.progress.Current)
		}
	}
}

func TestLoadingScreenCountsSeconds(t *testing.T) {
	now := t0
	m := New(Config{
		Load:      func(context.Context, func(Progress)) (Snapshot, error) { return garden3(), nil },
		DecayDays: 45,
		Now:       func() time.Time { return now },
	})
	m, _ = step(m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = step(m, progressMsg{p: Progress{Done: 1, Total: 3, Current: "me/big"}, ch: make(chan tea.Msg)})
	now = now.Add(12 * time.Second) // a big repo's first sync
	if v := visible(m.View()); !strings.Contains(v, "12s") {
		t.Errorf("a long fetch should show it is still going: %q", v)
	}
}

func TestLabelShowsInTheTicker(t *testing.T) {
	m := New(Config{
		Load:      func(context.Context, func(Progress)) (Snapshot, error) { return garden3(), nil },
		DecayDays: 45,
		Now:       func() time.Time { return t0 },
		Label:     "simulating 30d ahead",
	})
	m = ready(m, 100, 30)
	lines := strings.Split(m.View(), "\n")
	if last := visible(lines[len(lines)-1]); !strings.Contains(last, "simulating 30d ahead") {
		t.Errorf("ticker = %q", last)
	}
}

// starred copies snap with the given star counts on its repos, in order.
func starred(snap Snapshot, stars ...int) Snapshot {
	out := snap
	out.Repos = append([]Repo(nil), snap.Repos...)
	for i, s := range stars {
		out.Repos[i].Stars = s
	}
	return out
}

func TestNewStarsShootAndGetNoted(t *testing.T) {
	m := ready(newModel(starred(garden3(), 5, 0, 1), nil, t0), 100, 30)
	if len(m.shots) != 0 {
		t.Fatal("the first load only sets the baseline")
	}
	m, _ = step(m, loadedMsg{snap: starred(garden3(), 7, 1, 1)})
	if len(m.shots) != 3 {
		t.Errorf("shots = %d, want 3 (2 + 1)", len(m.shots))
	}
	lines := strings.Split(m.View(), "\n")
	if last := visible(lines[len(lines)-1]); !strings.Contains(last, "⭐ 2 new stars on bloom") {
		t.Errorf("ticker = %q", last)
	}
	m, _ = step(m, loadedMsg{snap: starred(garden3(), 57, 1, 1)})
	if len(m.shots) != 6 {
		t.Errorf("shots = %d; a refresh adds at most 3", len(m.shots))
	}
}

func TestStarsThatDontCountDontShoot(t *testing.T) {
	m := ready(newModel(starred(garden3(), 5, 5, 5), nil, t0), 100, 30)
	offline := starred(garden3(), 9, 9, 9)
	offline.Offline = true
	m, _ = step(m, loadedMsg{snap: offline})
	m, _ = step(m, loadedMsg{snap: starred(garden3(), 4, 5, 5)}) // an unstar
	newcomer := starred(garden3(), 4, 5, 5)
	extra := grow("newcomer", 10, 0, t0)
	extra.Stars = 30
	newcomer.Repos = append(newcomer.Repos, extra)
	m, _ = step(m, loadedMsg{snap: newcomer}) // a repo joining brings its stars along
	if len(m.shots) != 0 || len(m.notes) != 0 {
		t.Errorf("shots %d, notes %d; want none", len(m.shots), len(m.notes))
	}
}

func TestShotsAndNotesExpire(t *testing.T) {
	now := t0
	m := New(Config{
		Load:      func(context.Context, func(Progress)) (Snapshot, error) { return starred(garden3(), 1, 1, 1), nil },
		DecayDays: 45,
		Now:       func() time.Time { return now },
	})
	m = ready(m, 100, 30)
	m, _ = step(m, loadedMsg{snap: starred(garden3(), 2, 1, 1)})
	if len(m.shots) != 1 || len(m.notes) != 1 {
		t.Fatalf("shots %d, notes %d; want 1 each", len(m.shots), len(m.notes))
	}
	now = now.Add(61 * time.Second)
	m, _ = step(m, tickMsg{})
	if len(m.shots) != 0 || len(m.notes) != 0 {
		t.Errorf("shots %d, notes %d; want none after a minute", len(m.shots), len(m.notes))
	}
}

func TestShootingStarsKeepFramesFast(t *testing.T) {
	night := t0.Add(11 * time.Hour)
	m := ready(newModel(starred(garden3(), 1, 1, 1), nil, night), 80, 24)
	if m.frameInterval() != slowFrame {
		t.Fatal("a quiet night should idle")
	}
	m, _ = step(m, loadedMsg{snap: starred(garden3(), 2, 1, 1)})
	if m.frameInterval() != fastFrame {
		t.Error("a shooting star in flight needs the fast frame rate")
	}
}

func TestCalmLineShowsTheStarTotalInStarMode(t *testing.T) {
	snap := Snapshot{Repos: []Repo{grow("bloom", 60, 4, t0.Add(-time.Hour))}, Commits7d: 5, FetchedAt: t0}
	snap.Repos[0].Stars = 48
	m := New(Config{
		Load:      func(context.Context, func(Progress)) (Snapshot, error) { return snap, nil },
		DecayDays: 45,
		Now:       func() time.Time { return t0 },
		Sky:       scene.SkyStars,
	})
	m = ready(m, 100, 30)
	lines := strings.Split(m.View(), "\n")
	if last := visible(lines[len(lines)-1]); !strings.Contains(last, "Garden thriving: 5 commits this week · ⭐ 48") {
		t.Errorf("ticker = %q", last)
	}
}

func TestStarNoteShowsWhileItsShotFlies(t *testing.T) {
	now := t0
	var snap Snapshot
	for i := 0; i < 8; i++ { // 24 attention items: longer than a note's minute to cycle through
		r := grow(strings.Repeat("r", i+1), 40, 0, t0.Add(-40*24*time.Hour))
		r.CI = CIFailing
		r.PRs = []time.Time{t0.Add(-time.Hour)}
		snap.Repos = append(snap.Repos, r)
	}
	snap.FetchedAt = t0
	m := New(Config{
		Load:      func(context.Context, func(Progress)) (Snapshot, error) { return snap, nil },
		DecayDays: 45,
		Now:       func() time.Time { return now },
	})
	m = ready(m, 100, 30)
	now = now.Add(5 * time.Second) // the ticker has moved past its first item
	m, _ = step(m, loadedMsg{snap: starred(snap, 1)})
	lines := strings.Split(m.View(), "\n")
	if last := visible(lines[len(lines)-1]); !strings.Contains(last, "⭐ New star on r") {
		t.Errorf("while the shooting star flies the ticker says %q", last)
	}
}

func TestSameShortNamesDontShoot(t *testing.T) {
	a, b := grow("gag", 40, 0, t0), grow("gag", 30, 0, t0) // me/gag and upstream/gag
	a.Stars, b.Stars = 100, 2
	snap := Snapshot{Repos: []Repo{a, b}, FetchedAt: t0}
	m := ready(newModel(snap, nil, t0), 100, 30)
	for i := 0; i < 3; i++ {
		m, _ = step(m, loadedMsg{snap: snap}) // nothing changed
	}
	if len(m.shots) != 0 || len(m.notes) != 0 {
		t.Errorf("unchanged refreshes gave %d shots and notes %+v", len(m.shots), m.notes)
	}
}

func TestDemoGardenIsNoStarBaseline(t *testing.T) {
	demo := Snapshot{Repos: []Repo{grow("dotfiles", 40, 0, t0)}, FetchedAt: t0, Demo: true,
		Note: "demo · no GitHub token: run gh auth login"}
	m := ready(newModel(demo, nil, t0), 100, 30)
	real := Snapshot{Repos: []Repo{grow("dotfiles", 12, 0, t0)}, FetchedAt: t0}
	real.Repos[0].Stars = 4
	m, _ = step(m, loadedMsg{snap: real}) // the user signed in
	if len(m.shots) != 0 || len(m.notes) != 0 {
		t.Errorf("signing in gave %d shots and notes %+v", len(m.shots), m.notes)
	}
}

func TestShootingStarsQueueUp(t *testing.T) {
	now := t0
	cur := starred(garden3(), 1, 1, 1)
	m := New(Config{
		Load:      func(context.Context, func(Progress)) (Snapshot, error) { return cur, nil },
		DecayDays: 45,
		Now:       func() time.Time { return now },
	})
	m = ready(m, 100, 30)
	m, _ = step(m, loadedMsg{snap: starred(garden3(), 3, 1, 1)})
	if first := m.shots[0].Start; first.Before(t0.Add(slowFrame)) {
		t.Errorf("the first shooting star starts %v after its refresh; want it to wait one idle frame", first.Sub(t0))
	}
	now = now.Add(time.Second)
	m, _ = step(m, loadedMsg{snap: starred(garden3(), 5, 1, 1)})
	if len(m.shots) != 4 {
		t.Fatalf("shots = %d, want 4", len(m.shots))
	}
	for i := 1; i < len(m.shots); i++ {
		if gap := m.shots[i].Start.Sub(m.shots[i-1].Start); gap < shotGap {
			t.Errorf("shots %d and %d start %v apart; want at least %v", i-1, i, gap, shotGap)
		}
	}
}

func TestStarsForTheSameRepoShareANote(t *testing.T) {
	now := t0
	cur := starred(garden3(), 1, 1, 1)
	m := New(Config{
		Load:      func(context.Context, func(Progress)) (Snapshot, error) { return cur, nil },
		DecayDays: 45,
		Now:       func() time.Time { return now },
	})
	m = ready(m, 100, 30)
	m, _ = step(m, loadedMsg{snap: starred(garden3(), 2, 1, 1)})
	now = now.Add(5 * time.Second)
	m, _ = step(m, loadedMsg{snap: starred(garden3(), 4, 1, 1)})
	if len(m.notes) != 1 || m.notes[0].n != 3 {
		t.Fatalf("notes = %+v; want one note of 3 new stars on bloom", m.notes)
	}
	if m.notes[0].until != now.Add(noteFor) {
		t.Error("new stars should restart the note's minute")
	}
	lines := strings.Split(m.View(), "\n")
	if last := visible(lines[len(lines)-1]); !strings.Contains(last, "⭐ 3 new stars on bloom") {
		t.Errorf("ticker = %q", last)
	}
}

func TestSkySettingReachesTheLiveView(t *testing.T) {
	m := New(Config{
		Load:      func(context.Context, func(Progress)) (Snapshot, error) { return starred(garden3(), 5, 0, 1), nil },
		DecayDays: 45,
		Now:       func() time.Time { return t0 },
		Sky:       scene.SkyStars,
	})
	m = ready(m, 100, 30)
	if v := m.sceneView(m.now()); v.Sky != scene.SkyStars || v.StarTotal != 6 {
		t.Errorf("the live frame has sky %v with %d stars; want the star sky with 6", v.Sky, v.StarTotal)
	}
}
