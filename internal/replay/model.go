// Package replay is a time-lapse view of one plant growing from its event
// history. A virtual clock sweeps forward; events land as the clock passes
// them, and quiet spells visibly wilt the plant.
package replay

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/RursusAeternum/GitAGarden/internal/garden"
	"github.com/RursusAeternum/GitAGarden/internal/pixel"
	"github.com/RursusAeternum/GitAGarden/internal/scene"
)

const (
	frameRate = 80 * time.Millisecond
	// After the last event the clock keeps running so you can watch neglect.
	tailDays = 60
	logLines = 7
)

// Up to 8 days per tick, so a multi-year real history still plays in about a minute.
var speeds = []time.Duration{time.Hour, 3 * time.Hour, 6 * time.Hour, 12 * time.Hour, 24 * time.Hour, 48 * time.Hour, 96 * time.Hour, 192 * time.Hour}

type Config struct {
	Name      string
	Species   garden.Species
	Events    []garden.Event
	DecayDays float64
	Finished  bool
	// Profile is the color depth for the plant drawing.
	Profile pixel.Profile
	// Now is the wall clock the sky is drawn at (default time.Now). The
	// replay clock moves hours per frame and would strobe day and night.
	Now func() time.Time
	// Sky is what the night sky shows; Stars is the replayed repo's GitHub
	// stars, for the star sky.
	Sky   scene.SkyMode
	Stars int
	// End is where the clock stops. Zero means tailDays after the last event,
	// so a fake history always ends with a spell of neglect.
	End time.Time
	// Regenerate returns a fresh fake history, used by the "new history" key.
	// nil disables the key.
	Regenerate func(name string) []garden.Event
}

type Model struct {
	cfg     Config
	applied int
	clock   time.Time
	start   time.Time
	end     time.Time
	playing bool
	speed   int
	gen     int
}

type tickMsg struct{}

func New(cfg Config) Model {
	m := Model{cfg: cfg, playing: true, speed: 2}
	m.reset()
	return m
}

func (m *Model) reset() {
	m.applied = 0
	if len(m.cfg.Events) == 0 {
		m.start = time.Now()
		m.end = m.start
	} else {
		m.start = m.cfg.Events[0].At.Add(-time.Hour)
		m.end = m.cfg.Events[len(m.cfg.Events)-1].At.Add(tailDays * 24 * time.Hour)
		if !m.cfg.End.IsZero() && m.cfg.End.After(m.start) {
			m.end = m.cfg.End
		}
	}
	m.clock = m.start
}

func tick() tea.Cmd {
	return tea.Tick(frameRate, func(time.Time) tea.Msg { return tickMsg{} })
}

func (m Model) Init() tea.Cmd { return tick() }

// seek moves the clock and recomputes how many events have landed.
func (m *Model) seek(t time.Time) {
	m.clock = t
	m.applied = 0
	for m.applied < len(m.cfg.Events) && !m.cfg.Events[m.applied].At.After(t) {
		m.applied++
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		if m.playing {
			m.seek(m.clock.Add(speeds[m.speed]))
			if !m.clock.Before(m.end) {
				m.clock, m.playing = m.end, false
			}
		}
		return m, tick()

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case " ":
			if !m.clock.Before(m.end) {
				m.reset()
			}
			m.playing = !m.playing
		case "right", "l":
			m.playing = false
			if m.applied < len(m.cfg.Events) {
				m.seek(m.cfg.Events[m.applied].At)
			}
		case "left", "h":
			m.playing = false
			if m.applied > 1 {
				m.seek(m.cfg.Events[m.applied-2].At)
			} else {
				m.seek(m.start)
			}
		case "end", "G":
			m.playing = false
			m.seek(m.end)
		case "+", "=":
			m.speed = min(m.speed+1, len(speeds)-1)
		case "-", "_":
			m.speed = max(m.speed-1, 0)
		case "s":
			m.cfg.Species = m.cfg.Species.Next()
		case "f":
			m.cfg.Finished = !m.cfg.Finished
		case "r":
			m.reset()
			m.playing = true
		case "n":
			if m.cfg.Regenerate != nil {
				m.gen++
				m.cfg.Name = fmt.Sprintf("%s-%d", strings.SplitN(m.cfg.Name, "-", 2)[0], m.gen)
				m.cfg.Events = m.cfg.Regenerate(m.cfg.Name)
				m.reset()
				m.playing = true
			}
		}
	}
	return m, nil
}

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#5fd75f"))
	labelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#808080"))
	panelStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#444444")).Padding(0, 1).Width(44)
)

func speedLabel(d time.Duration) string {
	if d >= 24*time.Hour {
		return fmt.Sprintf("%dd/tick", int(d.Hours()/24))
	}
	return fmt.Sprintf("%dh/tick", int(d.Hours()))
}

func bar(frac float64, width int) string {
	n := int(frac*float64(width) + 0.5)
	n = max(0, min(n, width))
	return strings.Repeat("█", n) + labelStyle.Render(strings.Repeat("░", width-n))
}

func (m Model) View() string {
	events := m.cfg.Events[:m.applied]
	p := garden.Grow(m.cfg.Name, m.cfg.Species, events)
	health := garden.Health(p, m.clock, m.cfg.DecayDays)
	if m.cfg.Finished {
		health = 1
	}
	plot := scene.Plot{Plant: p, Style: garden.Style{Health: health}, Finished: m.cfg.Finished,
		Name: m.cfg.Name, Status: garden.Status(p, m.clock, m.cfg.Finished)}
	now := time.Now
	if m.cfg.Now != nil {
		now = m.cfg.Now
	}
	card := scene.Draw(scene.View{Cols: scene.BedCols, Plots: []scene.Plot{plot}, Now: now(), Seed: 1,
		Sky: m.cfg.Sky, StarTotal: m.cfg.Stars}).Encode(m.cfg.Profile)

	state := "▶ playing"
	if !m.playing {
		state = "⏸ paused"
	}
	progress := 0.0
	if span := m.end.Sub(m.start); span > 0 {
		progress = float64(m.clock.Sub(m.start)) / float64(span)
	}

	var b strings.Builder
	fmt.Fprintln(&b, titleStyle.Render("GAG · replay"))
	fmt.Fprintf(&b, "%s %s  %s %s\n", labelStyle.Render("repo"), m.cfg.Name, labelStyle.Render("species"), m.cfg.Species)
	fmt.Fprintf(&b, "%s  %s  %s\n", m.clock.Format("2006-01-02 15:04"), labelStyle.Render(state), labelStyle.Render(speedLabel(speeds[m.speed])))
	fmt.Fprintf(&b, "%s %s %d/%d\n", labelStyle.Render("time  "), bar(progress, 20), m.applied, len(m.cfg.Events))
	fmt.Fprintf(&b, "%s %s %.0f%%\n\n", labelStyle.Render("health"), bar(health, 20), health*100)
	fmt.Fprintf(&b, "%d pushes · %d merges · %d releases\n%d open issues\n\n", p.Pushes, p.Merges, p.Releases, p.OpenIssues)

	for i := 0; i < logLines; i++ {
		j := len(events) - 1 - i
		if j < 0 {
			fmt.Fprintln(&b)
			continue
		}
		e := events[j]
		line := fmt.Sprintf("%s %s %s", e.Kind.Icon(), e.At.Format("Jan 02"), e.Note)
		if r := []rune(line); len(r) > 42 {
			line = string(r[:41]) + "…"
		}
		if i > 0 {
			line = labelStyle.Render(line)
		}
		fmt.Fprintln(&b, line)
	}
	keys := "space play/pause  ←/→ step  + - speed\ns species  f glass"
	if m.cfg.Regenerate != nil {
		keys += "  n new history"
	}
	fmt.Fprint(&b, "\n"+labelStyle.Render(keys+"\nr restart  G jump to end  q quit"))

	return lipgloss.JoinHorizontal(lipgloss.Top, card, "  ", panelStyle.Render(b.String())) + "\n"
}
