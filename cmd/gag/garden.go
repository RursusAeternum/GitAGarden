package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"

	"github.com/RursusAeternum/GitAGarden/internal/garden"
	"github.com/RursusAeternum/GitAGarden/internal/github"
	"github.com/RursusAeternum/GitAGarden/internal/live"
	"github.com/RursusAeternum/GitAGarden/internal/scene"
)

func runGarden(args []string) error {
	fs := flag.NewFlagSet("garden", flag.ExitOnError)
	demoFlag := fs.Bool("demo", false, "show fake demo repos instead of GitHub")
	limit := fs.Int("limit", 8, "how many recently pushed repos to show")
	names := fs.String("repos", "", "comma-separated owner/name list to show instead (any public repo works)")
	user := fs.String("user", "", "show another GitHub user's or organization's public garden")
	ttl := fs.Duration("ttl", 15*time.Minute, "reuse cached GitHub data younger than this (live view: half of -refresh)")
	decay := fs.Float64("decay", 45, "days of neglect until fully wilted")
	simulate := fs.String("simulate", "", "fast-forward: show the garden as it would look after this long untouched, e.g. 30d, 2w, 36h")
	once := fs.Bool("once", false, "print one static frame and exit (the default when output isn't a terminal)")
	refresh := fs.Duration("refresh", 5*time.Minute, "how often the live view reloads GitHub data")
	fs.Parse(args)

	var ahead time.Duration
	if *simulate != "" {
		var err error
		if ahead, err = parseSpan(*simulate); err != nil || ahead < 0 {
			return fmt.Errorf("-simulate %q: want a positive span like 30d, 2w or 36h", *simulate)
		}
	}
	var list []string
	for _, s := range strings.Split(*names, ",") {
		if s = strings.TrimSpace(s); s != "" {
			list = append(list, s)
		}
	}
	src := source{names: list, owner: *user, limit: *limit, ttl: *ttl, demo: *demoFlag, decay: *decay}

	if *once || !term.IsTerminal(int(os.Stdout.Fd())) {
		return printOnce(src, ahead, *simulate)
	}
	ttlSet := false
	fs.Visit(func(f *flag.Flag) { ttlSet = ttlSet || f.Name == "ttl" })
	if !ttlSet {
		src.ttl = *refresh / 2 // so each background refresh really fetches
	}
	m := live.New(live.Config{Load: src.snapshot, Refresh: *refresh, DecayDays: *decay, Ahead: ahead, Profile: colorProfile()})
	_, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

// source says which repos the garden shows and how to load them.
type source struct {
	names []string
	owner string
	limit int
	ttl   time.Duration
	demo  bool
	decay float64
}

// snapshot loads the repos and grows their plants for the live view. It
// writes nothing to the terminal. Without a GitHub token it falls back to
// the demo garden with a note saying why.
func (s source) snapshot(ctx context.Context) (live.Snapshot, error) {
	now := time.Now()
	if s.demo {
		return live.Snapshot{Repos: demoGarden(now), FetchedAt: now, Note: "demo garden"}, nil
	}
	repos, offline, err := loadRepos(ctx, s.names, s.owner, s.limit, s.ttl, nil)
	if errors.Is(err, github.ErrNoToken) {
		return live.Snapshot{Repos: demoGarden(now), FetchedAt: now, Note: "demo · no GitHub token: run gh auth login"}, nil
	}
	if err != nil {
		return live.Snapshot{}, err
	}
	snap := live.Snapshot{Offline: offline, FetchedAt: now}
	weekAgo := now.Add(-7 * 24 * time.Hour)
	for _, r := range repos {
		snap.Repos = append(snap.Repos, live.Repo{Name: r.Name(), Plant: garden.Grow(r.Name(), r.Species(), r.Events()), Finished: r.Finished()})
		if !r.FetchedAt.IsZero() && r.FetchedAt.Before(snap.FetchedAt) {
			snap.FetchedAt = r.FetchedAt
		}
		for _, c := range r.Commits {
			if c.At.After(weekAgo) {
				snap.Commits7d++
			}
		}
	}
	return snap, nil
}

// printOnce prints one static frame: for pipes, scripts and -once.
func printOnce(src source, ahead time.Duration, simulate string) error {
	now := time.Now()
	at := now.Add(ahead) // the moment the garden is drawn at
	header := ""
	if ahead > 0 {
		header = fmt.Sprintf("simulating %s ahead: %s, nothing tended\n\n", simulate, at.Format("2006-01-02"))
	}
	var plots []scene.Plot
	if src.demo {
		for _, r := range demoGarden(now) {
			plots = append(plots, plotFor(r.Plant, r.Name, r.Finished, at, src.decay))
		}
	} else {
		repos, offline, err := loadRepos(context.Background(), src.names, src.owner, src.limit, src.ttl, logf)
		if err != nil {
			return err
		}
		if offline {
			logf("GitHub unreachable or partly stale; showing cached data")
		}
		for _, r := range repos {
			plots = append(plots, plotFor(garden.Grow(r.Name(), r.Species(), r.Events()), r.Name(), r.Finished(), at, src.decay))
		}
	}
	fmt.Print(header + scene.Compose(termWidth(), plots, at, 1).Encode(colorProfile()) + "\n")
	return nil
}

// plotFor bundles a grown plant with its health and status at time at.
func plotFor(p *garden.Plant, name string, finished bool, at time.Time, decay float64) scene.Plot {
	h := garden.Health(p, at, decay)
	if finished {
		h = 1
	}
	return scene.Plot{Plant: p, Style: garden.Style{Health: h}, Finished: finished,
		Name: name, Status: garden.Status(p, at, finished)}
}

type demoRepo struct {
	name, lang string
	events     int
	idleDays   int
	finished   bool
}

var demo = []demoRepo{
	{"gag-core", "go", 160, 0, false},
	{"rustyfs", "rust", 90, 12, false},
	{"notebook-api", "python", 70, 3, false},
	{"old-blog", "go", 120, 400, true},
	{"dotfiles", "shell", 40, 35, false},
	{"tiny-cli", "rust", 12, 1, false},
	{"site-v2", "typescript", 110, 70, false},
	{"lsystem", "c", 60, 200, true},
}

// demoGarden grows the demo repos, with fake histories whose last event
// lands idleDays before now.
func demoGarden(now time.Time) []live.Repo {
	var out []live.Repo
	for _, r := range demo {
		events := garden.FakeHistory(r.name, r.events, now)
		shift := now.Sub(events[len(events)-1].At) - time.Duration(r.idleDays)*24*time.Hour
		for i := range events {
			events[i].At = events[i].At.Add(shift)
		}
		out = append(out, live.Repo{Name: r.name, Plant: garden.Grow(r.name, garden.SpeciesFor(r.lang), events), Finished: r.finished})
	}
	return out
}
