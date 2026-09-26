# GAG — Git a Garden

A terminal garden of your git projects. Each repo is a plant that grows a
little with every push and merge, gets weeds for open issues, wilts when
neglected, and goes under a glass cloche when it's finished.

## Install

```sh
go install github.com/RursusAeternum/GitAGarden/cmd/gag@latest
```

## Run

```sh
go run ./cmd/gag                     # replay: watch one plant grow from its history
go run ./cmd/gag replay -species cactus -name rustyfs -events 300
go run ./cmd/gag garden              # static garden of demo repos
```

Replay keys: `space` play/pause · `←/→` step event · `+/-` speed · `s` cycle
species · `f` toggle glass · `n` new history · `r` restart · `G` jump to end · `q` quit.

The history is fake for now (`garden.FakeHistory`). The GitHub hookup comes next.

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
