// Command gag renders your git projects as a garden in the terminal.
//
//	gag          the live, animated garden of your GitHub repos (= gag garden)
//	gag garden   the same; -once prints one static frame, -demo uses fake repos
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
// pushed by owner (your own repos when owner is empty). If GitHub is
// unreachable it falls back to the local cache and reports offline. log gets
// progress lines ("fetching owner/repo"); nil discards them. progress, if not
// nil, hears (0, 0, "") while repos are listed, then Sync's per-repo progress.
func loadRepos(ctx context.Context, names []string, owner string, limit int, ttl time.Duration, log func(string), progress func(done, total int, current string)) ([]*github.Repo, bool, error) {
	store, err := github.OpenStore()
	if err != nil {
		return nil, false, err
	}
	c, err := github.NewClient()
	if err != nil {
		return nil, false, err
	}
	if progress != nil {
		progress(0, 0, "") // listing repos; the total isn't known yet
	}

	var metas []*github.Repo
	if len(names) > 0 {
		for _, n := range names {
			m, err := c.LookupRepo(ctx, n)
			if err != nil {
				return cachedRepos(store, names, "", limit, err)
			}
			metas = append(metas, m)
		}
	} else {
		if owner != "" {
			metas, err = c.ListOwnerRepos(ctx, owner, limit)
		} else {
			metas, err = c.ListRepos(ctx, limit)
		}
		if err != nil {
			return cachedRepos(store, nil, owner, limit, err)
		}
		if owner == "" && len(metas) > 0 {
			if login, _, _ := strings.Cut(metas[0].NameWithOwner, "/"); login != store.Viewer {
				store.Viewer = login
				if err := store.Save(); err != nil && log != nil {
					log("could not save the cache: " + err.Error())
				}
			}
		}
	}

	repos, err := github.Sync(ctx, c, store, metas, ttl, log, progress)
	if err != nil {
		if len(repos) == 0 {
			return nil, false, err
		}
		return repos, true, nil // some repos are stale cached copies
	}
	return repos, false, nil
}

// cachedRepos is loadRepos' offline fallback: the cached copies of the
// requested repos, or cause if there are none.
func cachedRepos(store *github.Store, names []string, owner string, limit int, cause error) ([]*github.Repo, bool, error) {
	var repos []*github.Repo
	if len(names) > 0 {
		for _, n := range names {
			if r := store.Repos[n]; r != nil {
				repos = append(repos, r)
			}
		}
	} else {
		if owner == "" {
			owner = store.Viewer // your own garden, not repos cached by -user or -repos
		}
		if owner == "" {
			return nil, false, cause // never listed online, so whose repos are yours is unknown
		}
		for _, r := range store.Repos {
			if strings.EqualFold(strings.SplitN(r.NameWithOwner, "/", 2)[0], owner) {
				repos = append(repos, r)
			}
		}
		sort.Slice(repos, func(i, j int) bool { return repos[i].PushedAt.After(repos[j].PushedAt) })
		repos = repos[:min(limit, len(repos))]
	}
	if len(repos) == 0 {
		return nil, false, cause
	}
	return repos, true, nil
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
	cfg := replay.Config{Name: *name, Species: sp, DecayDays: *decay, Finished: *finished, Profile: colorProfile()}

	if *repo != "" {
		repos, _, err := loadRepos(context.Background(), []string{*repo}, "", 1, *ttl, logf, nil)
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
