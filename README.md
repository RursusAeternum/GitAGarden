# GAG — Git a Garden

A terminal garden of your git projects. Each repo is a plant that grows a
little with every push and merge, gets weeds for open issues, wilts when
neglected, and goes under a glass cloche when it's finished.

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

Push a tag and GitHub Actions does the rest: it builds with GoReleaser,
publishes the release and updates the Homebrew tap.

```sh
git tag v0.1.0 && git push origin v0.1.0
```

This needs the repo secret `HOMEBREW_TAP_TOKEN`: a fine-grained token with
Contents read/write on `RursusAeternum/homebrew-tap`.

## Run

```sh
gag                                  # your GitHub repos as a garden
gag replay                           # watch one (fake) plant grow from its history
gag replay -species cactus -name rustyfs -events 300
```

Replay keys: `space` play/pause · `←/→` step event · `+/-` speed · `s` cycle
species · `f` toggle glass · `n` new history · `r` restart · `G` jump to end · `q` quit.

## Your GitHub repos

`gag garden` shows your most recently pushed repos. It authenticates with
`GITHUB_TOKEN` / `GH_TOKEN`, or borrows the token of the logged-in `gh` CLI.

```sh
gag garden                                   # 8 most recently pushed repos you own
gag garden -limit 12
gag garden -repos owner/a,owner/b            # a fixed set
gag garden -watch 5m                         # always-on display, e.g. on a Pi
gag replay -repo owner/name                  # time-lapse of a real repo
gag garden -demo                             # fake repos, no network
```

- **Pushes** are commits on the default branch. GitHub only keeps real push events for 90 days.
- **Finished** means the repo is archived, or has the topic `finished` or `gag-finished`.
- **Species** comes from the primary language.
- **Cache:** histories are stored in the OS cache dir (`~/.cache/gag` on Linux,
  `~/Library/Caches/gag` on macOS). They refresh at most every `-ttl` (15m), and
  commits are fetched incrementally. If GitHub is unreachable, the garden falls
  back to the cached data.

## How plants grow

`plant = Grow(repo name, species, events)`. The name seeds the RNG, and the
events are replayed in order, so the same repo always grows the same plant.
Nothing is stored between runs.

| Event | Adds |
|---|---|
| push | a leaf, a stem segment, or a spine; once full, leaves get lusher |
| merged PR | a flower |
| release | fruit |
| issue opened / closed | adds / removes a weed by the pot |
| time since last tended | leaves yellow → brown → fall, flowers droop (applied at render) |

Species (`garden.SpeciesFor(language)`): **shrub** (branching, cbonsai-ish),
**cactus** (column, arms, spines), **rosette** (low succulent with a flower stalk).

## Layout

```
cmd/gag/            CLI: replay and garden subcommands
internal/garden/    events, fake history, growers per species, renderer
internal/replay/    Bubble Tea time-lapse UI
```

## Raspberry Pi

```sh
GOOS=linux GOARCH=arm64 go build -o gag ./cmd/gag   # Pi 3/4/5 on 64-bit OS
GOOS=linux GOARCH=arm GOARM=7 go build -o gag ./cmd/gag   # 32-bit Raspberry Pi OS
```

## License

MIT, see [LICENSE](LICENSE).
