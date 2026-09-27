package live

import (
	"context"
	"fmt"
	"hash/fnv"
	"math"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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
	wakeGap          = time.Minute      // a longer gap between ticks means the machine slept
	wakeDelay        = 10 * time.Second // then give the network a moment before refreshing
	maxShots         = 3                // shooting stars per refresh, however many stars arrive
	shotGap          = 2 * time.Second  // between queued shooting stars
	noteFor          = time.Minute      // how long the ticker names a new star
)

var tickerStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#d0d0c8")).
	Background(lipgloss.Color("#1c1a17"))

// Config is what the live view needs from the outside world.
type Config struct {
	Load      func(ctx context.Context, progress func(Progress)) (Snapshot, error)
	Refresh   time.Duration // how often to reload; default 5m
	DecayDays float64
	Ahead     time.Duration // added to the wall clock, for -simulate
	Profile   pixel.Profile
	Now       func() time.Time // wall clock; default time.Now
	Label     string           // shown before the ticker's status, e.g. "simulating 30d ahead"
	Sky       scene.SkyMode    // what the night sky shows
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
	progress   Progress         // how far the current load has come
	lastTick   time.Time        // wall-clock time of the last frame, to notice sleep
	wokeAt     time.Time        // when a wake was noticed; the refresh waits wakeDelay
	progressAt time.Time        // when the last progress update arrived
	stars      map[string]int   // star counts from the last online load; nil before the first
	shots      []scene.Shooting // queued and flying shooting stars
	notes      []starNote       // ticker notes about new stars
}

type (
	tickMsg   struct{}
	loadedMsg struct {
		snap Snapshot
		err  error
	}
	refreshMsg  struct{ gen int }
	progressMsg struct {
		p  Progress
		ch chan tea.Msg // where the rest of this load's messages arrive
	}
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

// load runs Load in the background. Its progress updates and final result
// arrive as messages on one channel: each progressMsg carries the channel so
// Update can wait for the next message, and loadedMsg ends the stream.
func (m Model) load(force bool) tea.Cmd {
	load := m.cfg.Load
	return func() tea.Msg {
		ctx := context.Background()
		if force {
			ctx = context.WithValue(ctx, forceKey{}, true)
		}
		ch := make(chan tea.Msg, 8)
		go func() {
			s, err := load(ctx, func(p Progress) {
				msg := progressMsg{p: p, ch: ch}
				for { // keep the newest update: when the queue is full, drop its oldest
					select {
					case ch <- msg:
						return
					default:
						select {
						case <-ch:
						default:
						}
					}
				}
			})
			ch <- loadedMsg{s, err}
		}()
		return <-ch
	}
}

// next waits for a load's next message.
func next(ch chan tea.Msg) tea.Cmd {
	return func() tea.Msg { return <-ch }
}

func (m Model) tick() tea.Cmd {
	return tea.Tick(m.frameInterval(), func(time.Time) tea.Msg { return tickMsg{} })
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.cols, m.rows = msg.Width, msg.Height
	case tickMsg:
		m.prune()
		now := m.cfg.Now().Round(0) // wall clock: the monotonic one stops while the machine sleeps
		if !m.lastTick.IsZero() && now.Sub(m.lastTick) > wakeGap {
			m.wokeAt = now // Wi-Fi often needs a few seconds after waking
		}
		m.lastTick = now
		if !m.wokeAt.IsZero() && now.Sub(m.wokeAt) >= wakeDelay {
			m.wokeAt = time.Time{}
			if !m.loading && len(m.repos) > 0 {
				m.loading = true
				return m, tea.Batch(m.tick(), m.load(false))
			}
		}
		return m, m.tick()
	case progressMsg:
		m.progress, m.progressAt = msg.p, m.cfg.Now().Round(0)
		return m, next(msg.ch)
	case loadedMsg:
		m.loading = false
		m.progress = Progress{}
		m.gen++
		m.loadErr = msg.err
		if msg.err == nil {
			m.snap = msg.snap
			if msg.snap.Offline && len(m.repos) > 0 {
				m.repos = updateKnown(m.repos, msg.snap.Repos)
			} else {
				m.repos = merge(m.repos, msg.snap.Repos)
			}
			if !msg.snap.Offline { // cached counts are no news
				m.noticeNewStars(msg.snap.Repos)
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

// starNote tells the ticker that repo got n new stars, until until.
type starNote struct {
	repo  string
	n     int
	until time.Time
}

// noticeNewStars compares star counts with the last online load: each repo
// that gained stars gets a ticker note, and up to maxShots shooting stars
// cross the sky. The first load only sets the baseline; repos that just
// joined the garden don't count, and neither do lost stars.
func (m *Model) noticeNewStars(repos []Repo) {
	counts := make(map[string]int, len(repos))
	for _, r := range repos {
		counts[r.Name] = r.Stars
	}
	if m.stars != nil {
		wall, at, shots := m.cfg.Now(), m.now(), 0
		for _, r := range repos {
			before, known := m.stars[r.Name]
			gained := r.Stars - before
			if !known || gained <= 0 {
				continue
			}
			m.notes = append(m.notes, starNote{repo: r.Name, n: gained, until: wall.Add(noteFor)})
			for i := 0; i < gained && shots < maxShots; i++ {
				m.shots = append(m.shots, scene.Shooting{Start: at.Add(time.Duration(shots) * shotGap), Seed: at.UnixMilli() + int64(len(m.shots))})
				shots++
			}
		}
	}
	m.stars = counts
}

// prune forgets shooting stars that have landed and notes that have expired.
func (m *Model) prune() {
	at, wall := m.now(), m.cfg.Now()
	var shots []scene.Shooting
	for _, s := range m.shots {
		if at.Sub(s.Start) < scene.ShootingFor {
			shots = append(shots, s)
		}
	}
	var notes []starNote
	for _, n := range m.notes {
		if wall.Before(n.until) {
			notes = append(notes, n)
		}
	}
	m.shots, m.notes = shots, notes
}

func (m Model) starTotal() int {
	total := 0
	for _, r := range m.repos {
		total += r.Stars
	}
	return total
}

// starItems are the ticker's notes about new stars.
func (m Model) starItems() []Item {
	var items []Item
	for _, n := range m.notes {
		if !m.cfg.Now().Before(n.until) {
			continue
		}
		text := "New star on " + n.repo
		if n.n > 1 {
			text = fmt.Sprintf("%d new stars on %s", n.n, n.repo)
		}
		items = append(items, Item{"⭐", text})
	}
	return items
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
		pl := Plot(r, at, m.cfg.DecayDays)
		if !r.Finished { // still air under glass
			pl.Style.Sway = sway(r.Name, pl.Style.Health, at)
		}
		out[i] = pl
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

// frameInterval is fast while critters fly, rain falls or the camera slides
// (or is about to), slow otherwise, so an idle garden costs little.
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
	for _, s := range m.shots {
		if at.Sub(s.Start) < scene.ShootingFor {
			return true // a shooting star is flying, or queued to
		}
	}
	day := scene.Darkness(at) <= 0.5
	for _, pl := range m.plots(at) {
		if pl.Weather == scene.Storm || (day && scene.Flowering(pl)) {
			return true // rain falls, or critters fly
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
		return m.loadingView()
	}
	at := m.now()
	rows := m.gardenRows()
	plots := m.plots(at)
	v := scene.View{Cols: m.cols, Rows: rows, Plots: plots, Now: at, Seed: 1, Motion: true,
		Sky: m.cfg.Sky, StarTotal: m.starTotal(), Shooting: m.shots}
	if lay := scene.LayoutFor(m.cols, rows, len(plots)); lay.Overflow {
		v.Pan = scene.PanAt(m.elapsed(), lay.Columns)
	}
	out := scene.Draw(v).Encode(m.cfg.Profile)
	if m.tickerShown() {
		out += "\n" + tickerStyle.Render(m.tickerLine(at))
	}
	return out
}

// loadingView is the first-load screen: a progress bar and the repo being
// fetched, so a slow connection doesn't look like a hang.
func (m Model) loadingView() string {
	p := m.progress
	text := "🌱 growing your garden\n\nfinding your repos…"
	if p.Total > 0 {
		width := max(4, min(30, m.cols-12))
		text = fmt.Sprintf("🌱 growing your garden\n\n%s %d/%d", Bar(p.Done, p.Total, width), p.Done, p.Total)
		if p.Current != "" {
			text += "\nfetching " + p.Current
			if secs := int(m.cfg.Now().Round(0).Sub(m.progressAt).Seconds()); secs >= 2 {
				text += fmt.Sprintf(" · %ds", secs) // a big repo's first sync can take a minute
			}
		}
	}
	return center(m.cols, m.rows, text)
}

func (m Model) tickerLine(at time.Time) string {
	status := Status(m.snap, m.cfg.Now().Round(0), m.loading, m.loadErr != nil)
	if m.loading && m.progress.Total > 0 {
		status = fmt.Sprintf("refreshing %s %d/%d", Bar(m.progress.Done, m.progress.Total, 8), m.progress.Done, m.progress.Total)
	}
	if m.cfg.Label != "" {
		status = strings.TrimSuffix(m.cfg.Label+" · "+status, " · ")
	}
	if m.help {
		return fit(" "+helpLine, status+" ", m.cols)
	}
	items := Items(m.repos, at, m.cfg.DecayDays, m.snap.Commits7d)
	if m.cfg.Sky == scene.SkyStars && len(items) == 1 && items[0].Icon == calmIcon {
		items[0].Text += fmt.Sprintf(" · ⭐ %d", m.starTotal())
	}
	return Line(append(m.starItems(), items...), m.elapsed(), status, m.cols)
}

// center wraps text to the window's width and centers it in the window.
func center(cols, rows int, text string) string {
	wrapped := lipgloss.NewStyle().Width(cols).Align(lipgloss.Center).Render(text)
	return lipgloss.Place(cols, rows, lipgloss.Center, lipgloss.Center, wrapped)
}
