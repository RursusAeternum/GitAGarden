package live

import (
	"context"
	"hash/fnv"
	"math"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/RursusAeternum/GitAGarden/internal/garden"
	"github.com/RursusAeternum/GitAGarden/internal/pixel"
	"github.com/RursusAeternum/GitAGarden/internal/scene"
)

const (
	fastFrame        = 125 * time.Millisecond // ~8 fps while critters fly or the camera slides
	slowFrame        = 500 * time.Millisecond // ~2 fps when only clouds and sway move
	defaultRefresh   = 5 * time.Minute
	swayPeriod       = 5 * time.Second
	minCols, minRows = 24, 12
	helpLine         = "q quit · r refresh · t ticker · ? help"
)

var tickerStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#d0d0c8")).
	Background(lipgloss.Color("#1c1a17"))

// Config is what the live view needs from the outside world.
type Config struct {
	Load      func(ctx context.Context) (Snapshot, error)
	Refresh   time.Duration // how often to reload; default 5m
	DecayDays float64
	Ahead     time.Duration // added to the wall clock, for -simulate
	Profile   pixel.Profile
	Now       func() time.Time // wall clock; default time.Now
}

type Model struct {
	cfg        Config
	cols, rows int
	start      time.Time
	repos      []Repo // stable order: first seen first
	snap       Snapshot
	loading    bool
	loadErr    error // the last load's error, if it failed
	gen        int   // bumps on every finished load; older refresh timers are ignored
	ticker     bool
	help       bool
}

type (
	tickMsg   struct{}
	loadedMsg struct {
		snap Snapshot
		err  error
	}
	refreshMsg struct{ gen int }
)

func New(cfg Config) Model {
	if cfg.Refresh <= 0 {
		cfg.Refresh = defaultRefresh
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return Model{cfg: cfg, start: cfg.Now(), loading: true, ticker: true}
}

func (m Model) Init() tea.Cmd { return tea.Batch(m.load(false), m.tick()) }

type forceKey struct{}

// Forced reports whether a load was asked for by the user (r), so it should
// bypass caches.
func Forced(ctx context.Context) bool {
	forced, _ := ctx.Value(forceKey{}).(bool)
	return forced
}

func (m Model) load(force bool) tea.Cmd {
	load := m.cfg.Load
	return func() tea.Msg {
		ctx := context.Background()
		if force {
			ctx = context.WithValue(ctx, forceKey{}, true)
		}
		s, err := load(ctx)
		return loadedMsg{s, err}
	}
}

func (m Model) tick() tea.Cmd {
	return tea.Tick(m.frameInterval(), func(time.Time) tea.Msg { return tickMsg{} })
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.cols, m.rows = msg.Width, msg.Height
	case tickMsg:
		return m, m.tick()
	case loadedMsg:
		m.loading = false
		m.gen++
		m.loadErr = msg.err
		if msg.err == nil {
			m.snap = msg.snap
			if msg.snap.Offline && len(m.repos) > 0 {
				m.repos = updateKnown(m.repos, msg.snap.Repos)
			} else {
				m.repos = merge(m.repos, msg.snap.Repos)
			}
		}
		gen := m.gen
		return m, tea.Tick(m.cfg.Refresh, func(time.Time) tea.Msg { return refreshMsg{gen} })
	case refreshMsg:
		if msg.gen != m.gen || m.loading {
			return m, nil
		}
		m.loading = true
		return m, m.load(false)
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "r":
			if !m.loading {
				m.loading = true
				return m, m.load(true)
			}
		case "t":
			m.ticker = !m.ticker
		case "?":
			m.help = !m.help
		}
	}
	return m, nil
}

// merge keeps the order plants first appeared in: known repos are updated in
// place, new ones are appended, and ones no longer loaded are dropped.
func merge(old, fresh []Repo) []Repo {
	byName := make(map[string]Repo, len(fresh))
	for _, r := range fresh {
		byName[r.Name] = r
	}
	var out []Repo
	seen := map[string]bool{}
	for _, r := range old {
		if f, ok := byName[r.Name]; ok {
			out = append(out, f)
			seen[r.Name] = true
		}
	}
	for _, r := range fresh {
		if !seen[r.Name] {
			out = append(out, r)
		}
	}
	return out
}

// updateKnown refreshes the plants already shown from offline data without
// adding, dropping or reordering any: a cache is no evidence that the garden
// itself changed.
func updateKnown(shown, cached []Repo) []Repo {
	byName := make(map[string]Repo, len(cached))
	for _, r := range cached {
		byName[r.Name] = r
	}
	out := make([]Repo, len(shown))
	for i, r := range shown {
		if c, ok := byName[r.Name]; ok {
			r = c
		}
		out[i] = r
	}
	return out
}

func (m Model) now() time.Time         { return m.cfg.Now().Add(m.cfg.Ahead) }
func (m Model) elapsed() time.Duration { return m.cfg.Now().Sub(m.start) }
func (m Model) tickerShown() bool      { return m.ticker || m.help }

func (m Model) gardenRows() int {
	if m.tickerShown() {
		return m.rows - 1
	}
	return m.rows
}

func (m Model) plots(at time.Time) []scene.Plot {
	out := make([]scene.Plot, len(m.repos))
	for i, r := range m.repos {
		st := garden.Style{Health: 1} // under glass: full health, still air
		if !r.Finished {
			h := garden.Health(r.Plant, at, m.cfg.DecayDays)
			st = garden.Style{Health: h, Sway: sway(r.Name, h, at)}
		}
		out[i] = scene.Plot{Plant: r.Plant, Style: st, Finished: r.Finished,
			Name: r.Name, Status: garden.Status(r.Plant, at, r.Finished)}
	}
	return out
}

// sway is a plant's gentle lean at time at, in pixels at its top. Each plant
// has its own phase; wilted plants barely move.
func sway(name string, health float64, at time.Time) float64 {
	h := fnv.New32a()
	h.Write([]byte(name))
	phase := float64(h.Sum32()%1000) / 1000 * 2 * math.Pi
	period := swayPeriod.Milliseconds()
	return 1.5 * health * math.Sin(2*math.Pi*float64(at.UnixMilli()%period)/float64(period)+phase)
}

// frameInterval is fast while critters fly or the camera slides (or is about
// to), slow otherwise, so an idle garden costs little.
func (m Model) frameInterval() time.Duration {
	if m.busy() {
		return fastFrame
	}
	return slowFrame
}

func (m Model) busy() bool {
	if len(m.repos) == 0 || m.cols == 0 {
		return false
	}
	lay := scene.LayoutFor(m.cols, m.gardenRows(), len(m.repos))
	if lay.Overflow && (scene.Sliding(m.elapsed()) || scene.Sliding(m.elapsed()+slowFrame)) {
		return true
	}
	at := m.now()
	if scene.Darkness(at) > 0.5 {
		return false
	}
	for _, pl := range m.plots(at) {
		if scene.Flowering(pl) {
			return true
		}
	}
	return false
}

func (m Model) View() string {
	switch {
	case m.cols == 0:
		return "" // waiting for the first window size
	case m.cols < minCols || m.rows < minRows:
		return center(m.cols, m.rows, "make the window a bit bigger 🌱")
	case len(m.repos) == 0 && m.loadErr != nil:
		return center(m.cols, m.rows, "couldn't load your garden: "+m.loadErr.Error()+"\npress r to retry")
	case len(m.repos) == 0:
		return center(m.cols, m.rows, "🌱 growing your garden…")
	}
	at := m.now()
	rows := m.gardenRows()
	plots := m.plots(at)
	v := scene.View{Cols: m.cols, Rows: rows, Plots: plots, Now: at, Seed: 1, Motion: true}
	if lay := scene.LayoutFor(m.cols, rows, len(plots)); lay.Overflow {
		v.Pan = scene.PanAt(m.elapsed(), lay.Columns)
	}
	out := scene.Draw(v).Encode(m.cfg.Profile)
	if m.tickerShown() {
		out += "\n" + tickerStyle.Render(m.tickerLine(at))
	}
	return out
}

func (m Model) tickerLine(at time.Time) string {
	status := Status(m.snap, m.cfg.Now(), m.loading, m.loadErr != nil)
	if m.help {
		return fit(" "+helpLine, status+" ", m.cols)
	}
	return Line(Items(m.repos, at, m.cfg.DecayDays, m.snap.Commits7d), m.elapsed(), status, m.cols)
}

// center wraps text to the window's width and centers it in the window.
func center(cols, rows int, text string) string {
	wrapped := lipgloss.NewStyle().Width(cols).Align(lipgloss.Center).Render(text)
	return lipgloss.Place(cols, rows, lipgloss.Center, lipgloss.Center, wrapped)
}
