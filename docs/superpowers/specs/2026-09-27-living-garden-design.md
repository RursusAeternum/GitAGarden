# Living Garden: design

Date: 2026-09-27
Status: approved in conversation, pending written-spec review

## Intent

GAG should do two things at once: be **entertaining and interesting to look
at**, and **show the current status of your git projects**. v0.1 does both
only at a bare minimum. It prints a static, sparse picture with thin status.

**Primary use:** an always-on, ambient screen. A terminal window on the
user's Mac stays open next to their work and gets glanced at now and then.
It must feel alive, and it must show at a glance which project needs
attention.

**Success criteria**

- Left running, it is pleasant to look at: constant subtle motion, a scene
  that changes over the day, reactions to real activity.
- From across the room, a failing build or long-waiting PR is noticeable
  within a glance. The ticker states the same information in plain words.
- It stays light enough to leave open all day. The targets are in
  Performance.

**Decisions made in conversation**

| Topic | Decision |
|---|---|
| Main use | Always-on ambient screen |
| Signals | Neglect/momentum, open PRs + CI status, issues, live activity events (all four) |
| Display | A terminal window on the Mac, any size; the layout must adapt |
| Art style | True-color pixel art using half-block characters (2 pixels per cell) |
| Approach | "Living diorama": the status is part of the scene, plus a ticker and a detail card |
| Old ASCII renderer | Removed, replaced by the pixel painter |

## Architecture

Plain `gag` starts the **live view**, fullscreen and animated. `gag garden
--once` prints one static frame in the new style. `gag replay` uses the new
painter. `gag version` is unchanged.

Five units, each with one job:

### 1. `internal/pixel`: canvas

- `Canvas{W, H int; px []RGB}` in pixels. A terminal of `C×R` cells maps to
  `C × 2R` pixels.
- Primitives: `Set`, `Rect`, `Line`, `Blit(sprite)`, `Blend(color, alpha)`.
- `Encode() string`: each cell becomes `▀` with foreground = top pixel and
  background = bottom pixel, using true-color SGR codes. A code is emitted
  only when the fg/bg differs from the previous cell (run-length). Rows are
  joined with `\n`.
- `Text` overlay: a cell can instead hold a character with a fg color and
  the cell's background (used for plant names and the ticker).
- No knowledge of gardens.

### 2. `internal/garden`: plants

- The growth model is kept: `Grow(name, species, events)` is deterministic
  and replays events in order.
- Growers output **geometry in pixel space**, not characters: a
  `Structure` of stems (polylines with thickness), leaves (position,
  angle, size), buds, flowers, seed heads, fruit and weeds, on a plot
  canvas of about 22×40 pixels.
- **Size scales with log(total commits).** A 1-commit repo is a proper
  seedling (stem plus two leaves); around 500 commits is a full plant.
- **Flowers come only from merges in the last 30 days.** Older merges
  become seed heads, which fixes PR-heavy repos being buried in flowers.
- **Buds:** one per open PR, up to 5. Buds for PRs older than 7 days droop
  and turn pale.
- Painter: `Paint(canvas, structure, style)`, where style holds health
  (0–1), sway phase, species palette and a per-seed palette jitter so no
  two plants look identical.
- Species (shrub, cactus, rosette) and `SpeciesFor(language)` stay, with
  new pixel palettes and shapes.

### 3. `internal/scene`: the world

Pure functions of `(time, seed, state)`. No I/O.

- **Sky:** a gradient from local clock time (dawn, day, dusk, night). Sun
  and moon arcs, and stars at night.
- **Clouds:** drift slowly, seeded.
- **Ground:** a soil strip and pots (terracotta), with name labels on the
  ground row.
- **Weather per plant:** a small grey cloud while CI is running. When CI
  fails, a dark cloud with rain over that plant.
- **Critters:** occasional bees and butterflies wandering across; more of
  them around healthy, flowering plants.
- **Glass cloche** for finished repos, with a periodic glint sweep.

### 4. `internal/github`: extended data

Existing: commits, merged PRs, issues, releases, and the incremental cache.
Added:

- **Open PRs:** `pullRequests(states: OPEN)` → number, title, createdAt,
  isDraft.
- **CI status:** `defaultBranchRef.target.statusCheckRollup.state` →
  SUCCESS / FAILURE / ERROR / PENDING / EXPECTED / none.
- **Momentum:** computed from commit dates: commits in the last 14 days
  versus the 14 days before.
- The cache gains these fields. Older cache files still load; missing
  fields mean "unknown" until the next refresh.

### 5. `internal/live`: the app

A Bubble Tea program:

- Animation clock at ~8 fps. It drops to ~2 fps when no animation is
  active, since only clouds and sway are moving then.
- A background poller refreshes every 5 minutes (`-refresh` flag).
- Layout, ticker, selection and detail card.
- Diffs each new snapshot against the previous one and queues animations.

## Signals → scene

| Signal | Steady state | Animation on change |
|---|---|---|
| Neglect (days since last tended) | Leaves fade green → yellow → brown, sway weakens, leaves droop, then drop | — |
| Momentum (14d vs. prior 14d) | Rising: bright new shoots at the tips. Falling: no new growth | Push: watering-can splash, one leaf unfurls |
| Size (total history) | Log-scaled plant size, with a minimum seedling | — |
| Open PRs | Closed buds (max 5); older than 7 days: drooping and pale | — |
| Merges (last 30d) | Flowers; older merges are seed heads | Merge: a bud bursts open, a butterfly flies off |
| Releases | Fruit; the latest release has a gold glint | Release: a sparkle burst, bees arrive |
| CI on default branch | Pass: clear sky. Pending: small grey cloud. Fail: dark cloud with rain | Goes red: storm rolls in. Recovers: cloud clears, brief rainbow |
| Issues | Weeds at the pot base, log-scaled; a snail if 3 or more issues were opened in the last 7 days | New issue: a weed pops up. Closed: a weed gets pulled |
| Finished (archived or `finished` topic) | Glass cloche, full health, periodic glint | — |

**Attention order**, used by both the scene emphasis and the ticker:

1. CI failing
2. PRs waiting more than 7 days
3. Wilting (health < 0.5)
4. Other open PRs
5. Growing weeds

## Layout and interaction

```
 sky (time of day, sun/moon/stars, clouds, per-plant weather)
 plants in pots on the ground
 name labels on the ground strip
 ticker: urgent items cycling ...................... updated 3m ago
```

- **Plots** are about 22 columns wide. As many as fit sit side by side,
  with a second row when the height allows at least 2 plot heights.
- **Overflow:** if there are more repos than fit, the camera pans slowly
  along the garden, one plot every 30 s.
- **Stable order:** most recently pushed first, fixed at launch. Plants
  never reorder during a session.
- **Ticker:** one line. It cycles through urgent items in attention order,
  about 4 s each. Examples:
  - `⛈ DDNM: CI failing on main`
  - `🌷 KaineWebPage: 2 PRs waiting (9d)`
  - `🥀 larptown: 18d quiet`

  With nothing urgent it shows calm stats such as
  `Garden thriving: 41 commits this week`. The right side shows
  `updated 3m ago` or `offline · cached 2h ago`.
- **Selection:** `←/→` or a mouse click selects a plant, and the ground
  under it brightens. Selection clears after 60 s without input.
- **Detail card** (`enter` opens it, `esc` closes it): repo, language and
  species, last commit message and age, 14-day commit sparkline, open PRs
  with ages, CI status, issue count and newest issue, latest release.
- **Keys:** `←/→` select · `enter` details · `r` refresh now · `t` toggle
  ticker · `?` help · `q` quit.

## Data flow

```
poller (every 5m) → Snapshot{repos, openPRs, ci, fetchedAt}
  → diff(prev, next) → []Change{push, merge, release, issueOpened,
                                issueClosed, ciFailed, ciRecovered}
  → rebuild Structures for changed repos only
  → animation queue per plant (one at a time, ≥3 s apart)
clock tick → scene.Draw + garden.Paint(sway, active animations)
          → pixel.Encode → View()
```

Structures are rebuilt only when data changes. Frames only repaint.

## Error handling

| Situation | Behavior |
|---|---|
| No GitHub token | Demo garden; ticker says `no GitHub token: run gh auth login` |
| Network failure | Keep animating on cached data; ticker says `offline · cached <age>` |
| Rate limited | Back off to the next refresh interval; ticker notice |
| Window below 24×12 cells | Centered "make the window a bit bigger" message |
| No true-color support | Colors degrade to 256 (termenv) automatically |

## Performance

- The encoder emits SGR codes only on color change. Motion is subtle, so
  unchanged rows are skipped by Bubble Tea's line diff.
- Targets: under 10 ms to draw a 240×65-cell frame (benchmark test), and
  about 5% CPU or less on the user's Mac while idle-animating.

## Testing

- `pixel`: encoding of known framebuffers, run-length SGR output.
- `garden`: determinism, size scaling (1 commit → seedling), flower window
  (merges older than 30 days → seed heads), bud count and droop.
- `scene`: fixed-time checks (stars at 23:00, none at 12:00; storm present
  only when CI fails).
- `live`: diff → change events; Update-driven tests for keys, selection
  timeout, panning and refresh messages.
- **Golden frames:** a fixed snapshot at a fixed time, compared against
  `testdata/*.golden` (`-update` flag to regenerate).
- Benchmark: frame render time.

## Phasing

Each phase is a release:

1. **v0.2 New look:** `pixel`, pixel plants, static scene (sky at the
   current time, ground, pots, labels). `gag garden --once` and replay
   use it. The ASCII renderer is removed.
2. **v0.3 Alive:** the live view as the default command, with animation
   clock, sway, day/night, clouds, critters, layout with panning, the
   ticker (neglect items only) and the poller.
3. **v0.4 Signals:** open PRs as buds, CI weather, momentum shoots, the
   30-day flower window, weeds and snail, and ticker attention ordering.
4. **v0.5 Reactions:** diff-driven live animations, the detail card and
   mouse selection.

## Out of scope

- Sound, notifications or alerts outside the terminal.
- Non-GitHub hosts (GitLab etc.) and local-only repos.
- A Pi-specific kiosk setup. The live view should run there unchanged,
  but no systemd or autostart work is included.
- Configurable themes. Species palettes are fixed for now.
