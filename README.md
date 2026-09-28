# GAG — Git a Garden

A terminal garden of your git projects, drawn in true-color pixel art.
Each repo is a plant in a terracotta pot that grows a little with every
push and merge, gets weeds for open issues, wilts when neglected, and goes
under a glass cloche when it's finished, all under a sky that follows your
clock.

Try the demo garden in your browser: <https://kaine.at/demo/gag/>

## Install

**Homebrew** (macOS, Linux):

```sh
brew install RursusAeternum/tap/gag
```

**Install script** (macOS, Linux, Raspberry Pi):

```sh
curl -fsSL https://raw.githubusercontent.com/RursusAeternum/GitAGarden/main/install.sh | sh
```

The script picks the right build for your OS and CPU (including armv6/armv7/arm64
Pis), checks its checksum, and installs to `/usr/local/bin` or `~/.local/bin`.

**From source:** `go install github.com/RursusAeternum/GitAGarden/cmd/gag@latest`

Then run `gag`. It shows your garden, using your `gh` login or `GITHUB_TOKEN`.

### Releasing

Push a tag and GitHub Actions builds with GoReleaser and publishes the
release. [homebrew-tap](https://github.com/RursusAeternum/homebrew-tap)
checks for new releases every hour and updates its cask on its own. To
update it right away, run its workflow by hand:

```sh
git tag v0.1.0 && git push origin v0.1.0
gh workflow run update-gag.yml -R RursusAeternum/homebrew-tap   # optional
```

## Run

```sh
gag                                  # your GitHub repos as a live, animated garden
gag garden --once                    # print one static frame instead (also when piped)
gag replay                           # watch one (fake) plant grow from its history
gag replay -species cactus -name rustyfs -events 300
```

GAG draws with half-block characters and true color. If your terminal
supports true color but GAG shows banded colors, force it with
`GAG_COLOR=truecolor gag`. Use `GAG_COLOR=256` to force 256 colors.

The live view runs until you quit it. Plants sway, clouds drift, the sky
follows your clock, and bees visit healthy flowering plants. If the garden is
wider than the window it pans slowly, and a ticker at the bottom calls out
wilting projects and says how fresh the data is. It reloads from GitHub every
5 minutes (`-refresh`).

Live keys: `←/→` select a plant · `enter` open its detail card · `esc` close ·
`r` refresh now · `t` hide/show the ticker · `?` help · `q` quit. A click
selects a plant too. While GAG runs it takes the mouse. To select text, hold
⌥ Option in iTerm2, toggle View ▸ Allow Mouse Reporting (⌘R) in Terminal.app,
or hold Shift in most Linux terminals.

Replay keys: `space` play/pause · `←/→` step event · `+/-` speed · `s` cycle
species · `f` toggle glass · `n` new history · `r` restart · `G` jump to end · `q` quit.

## Your GitHub repos

`gag garden` shows your most recently pushed repos. It authenticates with
`GITHUB_TOKEN` / `GH_TOKEN`, or borrows the token of the logged-in `gh` CLI.

```sh
gag garden                                   # 8 most recently pushed repos you own
gag garden -limit 12
gag garden -repos owner/a,owner/b            # a fixed set; any public repo works
gag garden -user charmbracelet               # someone else's public garden (user or org)
gag garden -refresh 2m                       # live view that reloads every 2 minutes
gag garden -simulate 30d                     # preview: your garden after 30 days untouched (d, w, h)
gag replay -repo owner/name                  # time-lapse of a real repo
gag garden -demo                             # fake repos, no network
```

- **Pushes** are commits on the default branch. GitHub only keeps real push events for 90 days.
- **Finished** means the repo is archived, or has the topic `finished` or `gag-finished`.
- **Species** comes from the primary language.
- **Cache:** histories are stored in the OS cache dir (`~/.cache/gag` on Linux,
  `~/Library/Caches/gag` on macOS). Commits are fetched incrementally. `--once`
  reuses data younger than `-ttl` (15m); the live view refetches on every
  refresh and when you press `r`. If GitHub is unreachable, the garden falls
  back to your cached repos.
- **No token?** The live view shows the demo garden and tells you how to sign in.

## Config file

GAG reads `~/.config/gag/config` (or `$XDG_CONFIG_HOME/gag/config`): one
`key = value` per line, `#` for comments. Flags on the command line win over
the file. `gag config` shows where the file is and the settings in effect.

```
# ~/.config/gag/config
sky = stars          # the night sky shows your GitHub stars (default: random)
limit = 10
refresh = 2m
# repos = owner/a, owner/b
# user = someone
# decay = 30
```

With `sky = stars` the night sky has one star per GitHub star across the
repos in your garden. Whatever the sky, a shooting star crosses it when one
of your repos gets a new star while GAG is running, and the ticker says which.

## How plants grow

`plant = Grow(repo name, species, events)`. The name seeds the RNG, and the
events are replayed in order, so the same repo always grows the same plant.
Nothing is stored between runs.

| Signal | In the garden |
|---|---|
| push | a leaf, a stem segment, or a spine; once full, leaves get lusher |
| merged PR | a flower, which goes to seed 30 days later |
| release | fruit |
| open PR | a pink bud (up to 5); after a week of waiting it droops and fades |
| CI on the default branch | running: a small grey cloud · failing: a storm cloud with rain |
| more commits in the last 14 days than the 14 before | bright new shoots |
| open issues | weeds by the pot; 3+ new issues in a week bring a snail |
| time since last tended | leaves yellow → brown → fall, flowers droop |

The ticker at the bottom of the live view names what needs you, most urgent
first: failing CI, PRs waiting over a week, wilting plants, other open PRs,
then bursts of new issues.

Species (`garden.SpeciesFor(language)`): **shrub** (branching, cbonsai-ish),
**cactus** (column, arms, spines), **rosette** (low succulent with a flower stalk).

## Layout

```
cmd/gag/            CLI: the live garden, --once prints, replay
internal/github/    GraphQL client, repo histories and signals, on-disk cache
internal/garden/    events, growth per species, the plant painter
internal/scene/     sky, clouds, weather, critters, pots, beds and layout
internal/pixel/     RGB canvas encoded as half-block terminal text
internal/live/      the live Bubble Tea view and its ticker
internal/replay/    Bubble Tea time-lapse of one plant
web/                the browser demo: build script, page, Bubble Tea js stubs
```

## In a browser

```sh
web/build.sh            # -> web/dist/: index.html, gag.wasm, vendor/
```

Builds gag for WebAssembly (`cmd/gag/main_js.go`) onto a page where
[xterm.js](https://xtermjs.org) is the terminal: the demo garden and replay,
no server and no GitHub. Needs `go` and `npm`. Bubble Tea v1 can't build for
js, so the script builds against a temporary copy of it with the stubs in
`web/_bubbletea/`; the repo's `go.mod` is left alone.

## Raspberry Pi

```sh
GOOS=linux GOARCH=arm64 go build -o gag ./cmd/gag   # Pi 3/4/5 on 64-bit OS
GOOS=linux GOARCH=arm GOARM=7 go build -o gag ./cmd/gag   # 32-bit Raspberry Pi OS
```

## License

MIT, see [LICENSE](LICENSE).
