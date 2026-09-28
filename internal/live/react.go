package live

import (
	"slices"
	"time"

	"github.com/RursusAeternum/GitAGarden/internal/scene"
)

const (
	reactGap    = 3 * time.Second // a plant's reactions start at least this far apart
	visitLinger = time.Second     // the camera stays this long after a visit's reactions
)

// reaction is an animation scheduled on a repo's plant; its Plot is filled
// in when a frame is drawn, since plants can move.
type reaction struct {
	repo string
	anim scene.Reaction
}

// visit is the camera going to an off-screen plant while its reactions
// play, from from until until, in scene time.
type visit struct {
	repo        string
	from, until time.Time
	started     bool
}

// noticeChanges compares an online load with the one before and schedules
// a reaction and a ticker note for every change. Switching between the demo
// garden and a real one starts a fresh baseline.
func (m *Model) noticeChanges(snap Snapshot) {
	if snap.Demo != m.seenDemo {
		m.seen = nil
	}
	m.seenDemo = snap.Demo
	changes := ChangesBetween(m.seen, snap.Repos)
	m.seen = settled(m.seen, snap.Repos)
	if m.seen == nil {
		m.seen = []Repo{} // loaded, even if empty: the next load compares with it
	}
	m.schedule(changes)
}

// schedule queues each changed plant's reactions. Plants on screen start at
// once. When the garden pans and nothing is selected, the camera then visits
// the others in turn, in garden order, and their reactions start as it
// arrives; the first visit waits for every reaction already queued. While
// the camera still has visits to make, every changed plant joins the end of
// them, even one on screen now: by its turn the camera will have moved. A
// plant the visits already end on keeps that visit, for longer.
func (m *Model) schedule(changes []Change) {
	if len(changes) == 0 {
		return
	}
	at := m.now()
	base := at.Add(slowFrame) // by then the fast frames have started
	on := map[int]bool{}
	for _, i := range scene.OnScreen(m.sceneView(at)) {
		on[i] = true
	}
	visiting := m.layout().Overflow && m.sel.name == ""
	busy := visiting && len(m.visits) > 0
	var later [][]Change
	nextVisit := laterOf(base, m.lastEnd())
	for len(changes) > 0 {
		n := 1
		for n < len(changes) && changes[n].Repo == changes[0].Repo {
			n++
		}
		group := changes[:n]
		changes = changes[n:]
		i := m.indexOf(group[0].Repo)
		if i < 0 {
			continue
		}
		if visiting && (busy || !on[i]) {
			later = append(later, group)
			continue
		}
		if end := m.queue(group, laterOf(base, m.nextStart(group[0].Repo))); end.After(nextVisit) {
			nextVisit = end
		}
	}
	for _, v := range m.visits {
		nextVisit = laterOf(nextVisit, v.until)
	}
	slices.SortStableFunc(later, func(a, b []Change) int { // in garden order
		return m.indexOf(a[0].Repo) - m.indexOf(b[0].Repo)
	})
	for _, group := range later {
		repo := group[0].Repo
		if last := len(m.visits) - 1; last >= 0 && m.visits[last].repo == repo {
			end := m.queue(group, laterOf(base, m.nextStart(repo)))
			m.visits[last].until = laterOf(m.visits[last].until, end.Add(visitLinger))
			nextVisit = m.visits[last].until
			continue
		}
		from := nextVisit
		end := m.queue(group, laterOf(from.Add(scene.SlideFor), m.nextStart(repo)))
		nextVisit = end.Add(visitLinger)
		m.visits = append(m.visits, visit{repo: repo, from: from, until: nextVisit})
	}
}

// queue schedules one plant's changes one at a time from start, at least
// reactGap apart. Each change's note appears as its reaction starts, and
// the first reaction brings the plant's new shape. It returns when the last
// one ends.
func (m *Model) queue(group []Change, start time.Time) time.Time {
	t, end := start, start
	for k, ch := range group {
		r := scene.Reaction{Kind: ch.Kind, Start: t, Before: ch.Before, Reveals: k == 0, Drone: ch.Agent != "",
			Seed: t.UnixMilli() + int64(k)}
		m.reacts = append(m.reacts, reaction{repo: ch.Repo, anim: r})
		m.addChangeNote(ch.Icon, ch.Text, t.Add(-m.cfg.Ahead))
		end = t.Add(r.Duration())
		t = t.Add(max(r.Duration(), reactGap))
	}
	return end
}

// nextStart is the soonest repo's next reaction can start: once its last
// one has ended, and at least reactGap after that one started. Zero when
// none is queued.
func (m Model) nextStart(repo string) time.Time {
	var t time.Time
	for _, r := range m.reacts {
		if r.repo == repo {
			t = laterOf(t, laterOf(r.anim.Start.Add(r.anim.Duration()), r.anim.Start.Add(reactGap)))
		}
	}
	return t
}

// lastEnd is when the last reaction queued on any plant ends; zero when none.
func (m Model) lastEnd() time.Time {
	var end time.Time
	for _, r := range m.reacts {
		end = laterOf(end, r.anim.Start.Add(r.anim.Duration()))
	}
	return end
}

// indexOf is the index of the repo named name, or -1.
func (m Model) indexOf(name string) int {
	for i, r := range m.repos {
		if r.Name == name {
			return i
		}
	}
	return -1
}

// runVisits moves the camera for the visits due now: to each changed
// off-screen plant as its visit starts, and back to automatic panning after
// the last one ends. A selection holds the camera, so it cancels them.
func (m *Model) runVisits() {
	if len(m.visits) == 0 {
		return
	}
	if m.sel.name != "" {
		m.visits = nil
		return
	}
	at := m.now()
	var keep []visit
	ended, busy := false, false
	for _, v := range m.visits {
		if !at.Before(v.until) {
			ended = true
			continue
		}
		if !v.started && !at.Before(v.from) {
			if i := m.indexOf(v.repo); i >= 0 {
				m.holdOn(i)
			}
			v.started = true
		}
		busy = busy || v.started
		keep = append(keep, v)
	}
	m.visits = keep
	if ended && !busy {
		m.release()
	}
}

// reacting reports whether a reaction is playing or waiting, or the camera
// has visits to make.
func (m Model) reacting() bool {
	at := m.now()
	for _, r := range m.reacts {
		if at.Before(r.anim.Start.Add(r.anim.Duration())) {
			return true
		}
	}
	return len(m.visits) > 0
}

// laterOf is the later of two times.
func laterOf(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}
