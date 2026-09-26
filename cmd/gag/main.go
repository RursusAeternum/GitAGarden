// Command gag renders your git projects as a garden in the terminal.
//
//	gag garden   your GitHub repos as a garden (default; -demo for fake ones)
//	gag replay   time-lapse of one plant growing from its history
//	gag version
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"

	"github.com/RursusAeternum/GitAGarden/internal/garden"
	"github.com/RursusAeternum/GitAGarden/internal/github"
	"github.com/RursusAeternum/GitAGarden/internal/replay"
)

// version is stamped at release time by GoReleaser.
var version = "dev"

func main() {
	cmd, args := "garden", os.Args[1:]
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cmd, args = args[0], args[1:]
	}
	var err error
	switch cmd {
	case "replay":
		err = runReplay(args)
	case "garden":
		err = runGarden(args)
	case "version", "--version", "-v":
		fmt.Println("gag", version)
	default:
		err = fmt.Errorf("unknown command %q (want garden, replay or version)", cmd)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "gag:", err)
		os.Exit(1)
	}
}

func logf(s string) { fmt.Fprintln(os.Stderr, "gag:", s) }

// loadRepos returns synced repos: the named ones, or the limit most recently
// pushed. If GitHub is unreachable it falls back to the local cache.
func loadRepos(ctx context.Context, names []string, limit int, ttl time.Duration) ([]*github.Repo, error) {
	store, err := github.OpenStore()
	if err != nil {
		return nil, err
	}
	c, err := github.NewClient()
	if err != nil {
		return nil, err
	}

	var metas []*github.Repo
	if len(names) > 0 {
		for _, n := range names {
			m, err := c.LookupRepo(ctx, n)
			if err != nil {
				return cachedRepos(store, names, limit, err)
			}
			metas = append(metas, m)
		}
	} else if metas, err = c.ListRepos(ctx, limit); err != nil {
		return cachedRepos(store, nil, limit, err)
	}

	repos, err := github.Sync(ctx, c, store, metas, ttl, logf)
	if err != nil {
		if len(repos) == 0 {
			return nil, err
		}
		logf("some repos could not be refreshed; showing cached data")
	}
	return repos, nil
}

func cachedRepos(store *github.Store, names []string, limit int, cause error) ([]*github.Repo, error) {
	var repos []*github.Repo
	if len(names) > 0 {
		for _, n := range names {
			if r := store.Repos[n]; r != nil {
				repos = append(repos, r)
			}
		}
	} else {
		for _, r := range store.Repos {
			repos = append(repos, r)
		}
		sort.Slice(repos, func(i, j int) bool { return repos[i].PushedAt.After(repos[j].PushedAt) })
		repos = repos[:min(limit, len(repos))]
	}
	if len(repos) == 0 {
		return nil, cause
	}
	logf(fmt.Sprintf("offline (%v); showing cached data", cause))
	return repos, nil
}

func historyStart(n int) time.Time {
	// Roughly: enough runway that the fake history ends near today.
	return time.Now().Add(-time.Duration(n) * 18 * time.Hour)
}

func parseSpecies(s string) (garden.Species, error) {
	for _, sp := range garden.AllSpecies {
		if string(sp) == s {
			return sp, nil
		}
	}
	return "", fmt.Errorf("unknown species %q", s)
}

func runReplay(args []string) error {
	fs := flag.NewFlagSet("replay", flag.ExitOnError)
	repo := fs.String("repo", "", "owner/name of a GitHub repo to replay (default: a fake history)")
	name := fs.String("name", "gag-core", "fake repo name; seeds the plant's shape")
	species := fs.String("species", "shrub", "shrub, cactus or rosette (default for -repo: from its language)")
	n := fs.Int("events", 150, "number of fake events to generate")
	decay := fs.Float64("decay", 45, "days of neglect until fully wilted")
	finished := fs.Bool("finished", false, "show the project under glass")
	ttl := fs.Duration("ttl", 15*time.Minute, "reuse cached GitHub data younger than this")
	fs.Parse(args)

	speciesSet := false
	fs.Visit(func(f *flag.Flag) { speciesSet = speciesSet || f.Name == "species" })
	sp, err := parseSpecies(*species)
	if err != nil {
		return err
	}
	cfg := replay.Config{Name: *name, Species: sp, DecayDays: *decay, Finished: *finished}

	if *repo != "" {
		repos, err := loadRepos(context.Background(), []string{*repo}, 1, *ttl)
		if err != nil {
			return err
		}
		r := repos[0]
		cfg.Name, cfg.Events, cfg.End = r.Name(), r.Events(), time.Now()
		cfg.Finished = cfg.Finished || r.Finished()
		if !speciesSet {
			cfg.Species = r.Species()
		}
		if len(cfg.Events) == 0 {
			return fmt.Errorf("%s has no history to replay", *repo)
		}
	} else {
		cfg.Regenerate = func(name string) []garden.Event {
			return garden.FakeHistory(name, *n, historyStart(*n))
		}
		cfg.Events = cfg.Regenerate(*name)
	}
	_, err = tea.NewProgram(replay.New(cfg), tea.WithAltScreen()).Run()
	return err
}

func runGarden(args []string) error {
	fs := flag.NewFlagSet("garden", flag.ExitOnError)
	demo := fs.Bool("demo", false, "show fake demo repos instead of GitHub")
	limit := fs.Int("limit", 8, "how many recently pushed repos to show")
	names := fs.String("repos", "", "comma-separated owner/name list to show instead")
	ttl := fs.Duration("ttl", 15*time.Minute, "reuse cached GitHub data younger than this")
	watch := fs.Duration("watch", 0, "redraw at this interval, e.g. 5m (for an always-on display)")
	decay := fs.Float64("decay", 45, "days of neglect until fully wilted")
	simulate := fs.String("simulate", "", "fast-forward: show the garden as it would look after this long untouched, e.g. 30d, 2w, 36h")
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

	render := func() (string, error) {
		now := time.Now()
		at := now.Add(ahead) // the moment the garden is drawn at
		header := ""
		if ahead > 0 {
			header = fmt.Sprintf("simulating %s ahead: %s, nothing tended\n\n", *simulate, at.Format("2006-01-02"))
		}
		if *demo {
			return header + layout(demoCards(now, at, *decay)), nil
		}
		repos, err := loadRepos(context.Background(), list, *limit, *ttl)
		if err != nil {
			return "", err
		}
		var cards []string
		for _, r := range repos {
			p := garden.Grow(r.Name(), r.Species(), r.Events())
			cards = append(cards, garden.Card(p, garden.RenderOpts{Now: at, DecayDays: *decay, Finished: r.Finished()}))
		}
		return header + layout(cards), nil
	}

	if *watch <= 0 {
		out, err := render()
		fmt.Print(out)
		return err
	}
	for {
		out, err := render()
		if err != nil {
			logf(err.Error())
		} else {
			fmt.Print("\033[H\033[2J" + out)
		}
		time.Sleep(*watch)
	}
}

// layout tiles cards into rows that fit the terminal.
func layout(cards []string) string {
	width := 100
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		width = w
	}
	perRow := max(1, (width+1)/(garden.CardWidth+1))
	var b strings.Builder
	for i := 0; i < len(cards); i += perRow {
		row := cards[i:min(i+perRow, len(cards))]
		spaced := make([]string, 0, 2*len(row))
		for j, c := range row {
			if j > 0 {
				spaced = append(spaced, " ")
			}
			spaced = append(spaced, c)
		}
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, spaced...) + "\n\n")
	}
	return b.String()
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

// demoCards builds histories relative to now and draws them at at, which is
// later than now when simulating.
func demoCards(now, at time.Time, decay float64) []string {
	var cards []string
	for _, r := range demo {
		// Shift the fake history so its last event lands idleDays ago.
		events := garden.FakeHistory(r.name, r.events, now)
		shift := now.Sub(events[len(events)-1].At) - time.Duration(r.idleDays)*24*time.Hour
		for i := range events {
			events[i].At = events[i].At.Add(shift)
		}
		p := garden.Grow(r.name, garden.SpeciesFor(r.lang), events)
		cards = append(cards, garden.Card(p, garden.RenderOpts{Now: at, DecayDays: decay, Finished: r.finished}))
	}
	return cards
}
