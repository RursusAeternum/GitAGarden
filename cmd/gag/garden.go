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

	"github.com/RursusAeternum/GitAGarden/internal/config"
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
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	opts := merge(loadConfig(), set, settings{limit: *limit, repos: list, user: *user, refresh: *refresh, decay: *decay})

	src := source{names: opts.repos, owner: opts.user, limit: opts.limit, ttl: *ttl, demo: *demoFlag, decay: opts.decay, state: &sourceState{}}
	if *once || !term.IsTerminal(int(os.Stdout.Fd())) {
		return printOnce(src, ahead, *simulate, opts.sky)
	}
	if !set["ttl"] {
		src.ttl = opts.refresh / 2 // so each background refresh really fetches
	}
	label := ""
	if ahead > 0 {
		label = fmt.Sprintf("simulating %s ahead", *simulate)
	}
	m := live.New(live.Config{Load: src.snapshot, Refresh: opts.refresh, DecayDays: opts.decay, Ahead: ahead,
		Profile: colorProfile(), Label: label, Sky: opts.sky})
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
	state *sourceState // shared across a live session's loads; nil for one-shot use
}

// sourceState remembers what a live session has already shown.
type sourceState struct {
	real bool // a real GitHub garden has loaded
}

// snapshot loads the repos and grows their plants for the live view. It
// writes nothing to the terminal. Without a GitHub token it falls back to
// the demo garden with a note saying why.
func (s source) snapshot(ctx context.Context, progress func(live.Progress)) (live.Snapshot, error) {
	now := time.Now()
	if s.demo {
		return live.Snapshot{Repos: demoGarden(now), FetchedAt: now, Note: "demo garden", Demo: true}, nil
	}
	ttl := s.ttl
	if live.Forced(ctx) {
		ttl = 0 // the user pressed r: fetch, don't reuse the cache
	}
	var report func(done, total int, current string)
	if progress != nil {
		report = func(done, total int, current string) {
			progress(live.Progress{Done: done, Total: total, Current: current})
		}
	}
	repos, offline, err := loadRepos(ctx, s.names, s.owner, s.limit, ttl, nil, report)
	if errors.Is(err, github.ErrNoToken) && (s.state == nil || !s.state.real) {
		return live.Snapshot{Repos: demoGarden(now), FetchedAt: now, Note: "demo · no GitHub token: run gh auth login", Demo: true}, nil
	}
	if err != nil {
		return live.Snapshot{}, err // mid-session, even a lost token keeps the real garden on screen
	}
	if s.state != nil {
		s.state.real = true
	}
	snap := live.Snapshot{Offline: offline, FetchedAt: now}
	weekAgo := now.Add(-7 * 24 * time.Hour)
	for _, r := range repos {
		snap.Repos = append(snap.Repos, repoFor(r, now))
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
func printOnce(src source, ahead time.Duration, simulate string, sky scene.SkyMode) error {
	now := time.Now()
	at := now.Add(ahead) // the moment the garden is drawn at
	header := ""
	if ahead > 0 {
		header = fmt.Sprintf("simulating %s ahead: %s, nothing tended\n\n", simulate, at.Format("2006-01-02"))
	}
	var plots []scene.Plot
	stars := 0
	if src.demo {
		for _, r := range demoGarden(now) {
			plots = append(plots, live.Plot(r, at, src.decay))
			stars += r.Stars
		}
	} else {
		log, progress := logf, (func(done, total int, current string))(nil)
		if term.IsTerminal(int(os.Stderr.Fd())) {
			log, progress = nil, stderrProgress(os.Stderr) // a bar instead of "fetching …" lines
		}
		repos, offline, err := loadRepos(context.Background(), src.names, src.owner, src.limit, src.ttl, log, progress)
		if err != nil {
			return err
		}
		if offline {
			logf("GitHub unreachable or partly stale; showing cached data")
		}
		for _, r := range repos {
			plots = append(plots, live.Plot(repoFor(r, now), at, src.decay))
			stars += r.Stars
		}
	}
	frame := scene.Draw(scene.View{Cols: termWidth(), Plots: plots, Now: at, Seed: 1, Sky: sky, StarTotal: stars})
	fmt.Print(header + frame.Encode(colorProfile()) + "\n")
	return nil
}

type demoRepo struct {
	name, lang string
	events     int
	idleDays   int
	finished   bool
	ci         live.CI
	prDaysAgo  []int // open PRs, by age in days
	newIssues  int
	rising     bool
	stars      int
}

var demo = []demoRepo{
	{name: "gag-core", lang: "go", events: 160, ci: live.CIPassing, prDaysAgo: []int{2, 10}, rising: true, stars: 31},
	{name: "rustyfs", lang: "rust", events: 90, idleDays: 12, ci: live.CIFailing, stars: 12},
	{name: "notebook-api", lang: "python", events: 70, idleDays: 3, ci: live.CIPending, newIssues: 4, stars: 3},
	{name: "old-blog", lang: "go", events: 120, idleDays: 400, finished: true},
	{name: "dotfiles", lang: "shell", events: 40, idleDays: 35},
	{name: "tiny-cli", lang: "rust", events: 12, idleDays: 1, prDaysAgo: []int{1}},
	{name: "site-v2", lang: "typescript", events: 110, idleDays: 70},
	{name: "lsystem", lang: "c", events: 60, idleDays: 200, finished: true, stars: 2},
}

// demoGarden grows the demo repos, with fake histories whose last event
// lands idleDays before now and a few signals so every one shows.
func demoGarden(now time.Time) []live.Repo {
	var out []live.Repo
	for _, r := range demo {
		events := garden.FakeHistory(r.name, r.events, now)
		shift := now.Sub(events[len(events)-1].At) - time.Duration(r.idleDays)*24*time.Hour
		for i := range events {
			events[i].At = events[i].At.Add(shift)
		}
		lr := live.Repo{Name: r.name, Plant: garden.Grow(r.name, garden.SpeciesFor(r.lang), events),
			Finished: r.finished, Branch: "main", CI: r.ci, NewIssues: r.newIssues, Rising: r.rising, Stars: r.stars}
		for _, d := range r.prDaysAgo {
			lr.PRs = append(lr.PRs, now.Add(-time.Duration(d)*24*time.Hour))
		}
		out = append(out, lr)
	}
	return out
}

// repoFor grows a repo's plant and works out its signals as of now.
func repoFor(r *github.Repo, now time.Time) live.Repo {
	lr := live.Repo{Name: r.Name(), Plant: garden.Grow(r.Name(), r.Species(), r.Events()),
		Finished: r.Finished(), Branch: r.Branch, CI: ciFor(r.CI), Stars: r.Stars}
	for _, pr := range r.OpenPRs {
		if !pr.Draft { // drafts aren't waiting on anyone
			lr.PRs = append(lr.PRs, pr.CreatedAt)
		}
	}
	weekAgo := now.Add(-7 * 24 * time.Hour)
	for _, is := range r.Issues {
		if is.CreatedAt.After(weekAgo) {
			lr.NewIssues++
		}
	}
	var last, before int // commits in the last 14 days, and the 14 before
	for _, c := range r.Commits {
		switch age := now.Sub(c.At); {
		case age < 14*24*time.Hour:
			last++
		case age < 28*24*time.Hour:
			before++
		}
	}
	lr.Rising = last > before
	return lr
}

// ciFor maps GitHub's statusCheckRollup state to a CI state. States GitHub
// may add later count as unknown: clear skies rather than a false alarm.
func ciFor(state string) live.CI {
	switch state {
	case "SUCCESS":
		return live.CIPassing
	case "FAILURE", "ERROR":
		return live.CIFailing
	case "PENDING", "EXPECTED":
		return live.CIPending
	}
	return live.CIUnknown
}

// settings are the garden's options once the config file and flags are
// merged.
type settings struct {
	sky     scene.SkyMode
	limit   int
	repos   []string
	user    string
	refresh time.Duration
	decay   float64
}

// merge takes each option from its flag when the flag was given on the
// command line, and from the config file (which falls back to the defaults)
// otherwise. The sky is set only in the file.
func merge(file config.Config, set map[string]bool, flags settings) settings {
	s := settings{sky: skyMode(file.Sky), limit: file.Limit, repos: file.Repos, user: file.User,
		refresh: file.Refresh, decay: file.Decay}
	if set["limit"] {
		s.limit = flags.limit
	}
	if set["repos"] {
		s.repos = flags.repos
	}
	if set["user"] {
		s.user = flags.user
	}
	if set["refresh"] {
		s.refresh = flags.refresh
	}
	if set["decay"] {
		s.decay = flags.decay
	}
	if set["user"] && !set["repos"] {
		s.repos = nil // -user asks for that garden, not the file's repo list
	}
	return s
}

func skyMode(s string) scene.SkyMode {
	if s == "stars" {
		return scene.SkyStars
	}
	return scene.SkyRandom
}

// loadConfig reads the config file, reporting problems on stderr; it runs
// before any fullscreen view opens.
func loadConfig() config.Config {
	cfg, warns := config.Load(config.Path())
	for _, w := range warns {
		logf(w.String())
	}
	return cfg
}
