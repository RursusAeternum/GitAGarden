# Compact plants and recent history: design

Date: 2026-09-30
Status: approved in conversation, pending written-spec review
Builds on: `2026-09-27-living-garden-design.md` (the plant model) and
`2026-09-28-change-reactions-design.md` (v0.7 and v0.7.1 as shipped)
Ships as: v0.8.0

## Intent

A repo with a long history grows a plant that fills its whole plot. A
shrub with 3,000 commits covers 714 of its 736 grid cells: a solid
rectangle. Rosettes and cacti keep their outline, but every leaf turns
into a flower once merges run out of room. GAG also downloads up to 3,000
commits and up to 6,000 PRs, issues and releases per repo, so the first
load of a big account takes minutes.

v0.8 grows every plant in two layers:
- **A skeleton** comes from the whole history, so a long-lived project
  still earns a big plant.
- **A canopy** comes from recent activity, so the plant still says what is
  happening now.

The skeleton needs only a repo's totals. So GAG can fetch much less: the
totals, plus recent history in detail. That is the default, and a config
setting brings back full fetching.

This comes before visiting other people's gardens (see Out of scope). Big
accounts must look like gardens and load quickly first.

**Decisions made in conversation**

| Topic | Decision |
|---|---|
| What a huge, old repo looks like | Size and structure from the whole history; lushness and flowers from recent activity |
| Scope | Change how plants grow, and fetch less, with the fetching chosen in the config file |
| Default fetching | `recent` |
| How the setting affects plants | Never. It changes only what is fetched; `full` and `recent` grow identical plants |
| Approach | Skeleton plus canopy, not a compressed replay of events |

## 1. How a plant grows

A plant is grown **as of a moment**, `at`: the time the garden is drawn.
All the windows below count back from `at`.

### The skeleton

It comes from the whole history.

- **Size.** The height cap follows today's log curve of the total push
  count: `log2(1 + pushes) / log2(513)`. A plant reaches full height at
  about 500 pushes.
- **Structure.** Each species' habit takes a number of structural growth
  steps that follows the same curve:
  - the shrub: branching stems
  - the cactus: body columns and arms
  - the rosette: its stalk and the width of its base

  The plan fixes the step count at full size for each species.
- **Only ever growing.** The steps draw from the plant's own random
  stream, seeded by its name. The skeleton at `n` steps is the skeleton at
  `n − 1` steps plus one step. More history only ever adds structure; a
  plant never rearranges itself when new commits arrive.
- **Silhouette.** Each species has a silhouette that scales with the
  skeleton:
  - the shrub: a rounded crown above its trunk
  - the cactus: its columns and arms
  - the rosette: a low, wide base under its stalk

  Leaves, flowers and fruit only go inside the silhouette. A plant never
  fills its plot.

### The canopy

It comes from recent activity.

- **Leaf budget.** Half the cells of the plant's silhouette at its current
  size, rounded down, so there is always air between the leaves.
- **Dormant leaves.** A third of the budget is always in leaf, however
  quiet the repo. An old, quiet repo is a big, sparse plant, not bare
  sticks. Wilting colours already show neglect.
  - They are placed from the plant's own seeded stream, like the skeleton,
    not from old events. So a plant grown from its totals has exactly the
    dormant leaves of one grown from its whole history.
  - Recent leaves take the dormant leaves' places first, then new ones: a
    busy plant is lusher, not a different plant.
- **Recent leaves.** Each push from the last **90 days** adds a leaf
  cluster of up to two cells. They fill the rest of the budget, newest
  first. When a repo pushed more than fits, the newest pushes win.
- **Flowers.** Each merge from the last **30 days** is a flower. From 30 to
  **90 days** it is a seed head, as today. Older merges have dropped theirs.
  Flowers and seed heads fill at most a **quarter** of the leaf budget,
  newest first.
- **Fruit.** One per release from the past year, up to **four**, newest
  first.
- **Staying put.** Each leaf, flower and fruit is placed from its own
  event: a seed made of the repo's name, the event's time and its kind. A
  cell stays where it is while its event stays inside its window. When the
  event ages out, only that cell leaves.

### Unchanged

- buds for open PRs, weeds for open issues, and the snail
- fresh shoots for growth under 14 days old on a rising plant
- the rising check: commits in the last 14 days against the 14 before
- health and wilting colours, from the time of the newest commit
- glass over finished repos
- the pot, labels and status line

## 2. What GAG fetches

### The setting

The config file gains one key:

```
history = recent   # the default: totals, plus recent history in detail
history = full     # everything, as before v0.8
```

- Any other value is ignored with the usual config warning, and the
  default applies.
- `gag config` shows the setting like the others.
- There is no flag: this is a config-file setting.

### In both modes

GAG asks GitHub for four totals per repo, in one query:
- commits on the default branch
- merged PRs
- releases
- open issues

The skeleton and the weeds grow from these. Even `full` stops at 3,000
commits, so the totals are the only honest size for a big repo.

### `recent`

- **Commits:** those from the last 90 days, and always at least the newest
  one. "Last tended", and with it health and wilting, stay right for quiet
  repos.
- **Merged PRs:** those merged in the last 90 days, newest first, stopping
  at the window.
- **Releases:** the ten newest, whatever their age. The card's release line
  stays right for a repo whose last release was two years ago. Only those
  from the past year become fruit.
- **Issues:**
  - open issues, newest first (first page)
  - issues closed in the last 90 days
- **Open PRs:** as today.

Today, PRs, issues and releases are downloaded whole on every refresh,
oldest first, and capped at 2,000. That is slow, and on very large repos
it hides new merges. `recent` fetches them newest first and stops at its
window.

### `full`

Everything that was fetched before v0.8, plus the totals.

### Refreshes and the cache

- **Incremental commits.** Refreshes fetch only commits newer than the
  newest cached one, in both modes, as today.
- **Completeness.** The cache keeps whatever was fetched and records
  whether a repo's history is complete. A complete cache also serves
  `recent`. Switching to `full` fetches the missing history once.
- **Old caches.** Caches written before v0.8 have no totals. GAG counts
  them from the cached history until the next online fetch fills them in.
  Nothing is downloaded again just because GAG was upgraded.
- **Totals and events.** When the totals are lower than what the cached
  events count, as with an old cache or a repo whose history was
  rewritten, the larger count applies.

### `gag replay`

Replay wants a repo's whole life. In `recent` mode it fetches the full
history for the one repo it replays, and caches it as complete.

## 3. Time and the rest of GAG

### When plants are grown

- **The live view.** Plants are grown when a load arrives, at that load's
  time: every refresh, 5 minutes by default. Between loads, only the
  per-frame ageing changes, as today: flowers going to seed, buds drooping.
- **`-simulate`.** Plants are grown at the simulated time. As leaves age
  out of the window, the canopy thins, down to its dormant third.
- **`--once`.** Grown at the time of the frame.
- **`gag replay`.** At each frame's moment, the plant grows from the totals
  counted up to that moment and the events up to it. The skeleton builds
  up while the canopy comes and goes, and flowers bloom and drop.

### The rest of GAG

- **Reactions (v0.7).** They keep working on the old and new shape of a
  plant. A push can now add a leaf and drop the oldest; cells gone from
  the old shape vanish, as the v0.7 spec already says. A merge's new
  flower is still found as a flower the old shape lacked.
- **The card.** It is unchanged. Everything it shows falls inside what is
  fetched, and the open-issue count comes from the totals.
- **The demo.** It runs through the same growth, with totals counted from
  its fake histories. Its plants look different, so the golden frame is
  updated once, on purpose.
- **The ticker.** It is unchanged. Its 7-day commit count and 30-day
  flower window fall inside the fetched window.

## 4. Units

- **`internal/garden`**
  - `Totals{Pushes, Merges, Releases, OpenIssues int}`.
  - `Grow(name, species, totals, events, at)`:
    - grows the skeleton from the totals, and the canopy from the events
      inside its windows as of `at`
    - accepts full or recent events alike
    - uses the larger of the totals and the counted events
    - sets the plant's counters (pushes, merges, releases, open issues)
      from the totals, so change reactions keep comparing like with like
  - A helper that counts totals from a complete event list, for the demo,
    replay and tests.
  - The species silhouettes, the leaf budget, and per-event placement.
- **`internal/github`**
  - the totals query
  - the `recent` fetches
  - `Repo.Totals` and a completeness mark
  - loading old caches without totals
- **`internal/config`**
  - the `history` key, with its warning and default
- **`cmd/gag`**
  - `repoFor` grows plants from totals and events at the load's time
  - loading passes the fetch mode
  - replay forces full history for its repo
  - `gag config` shows `history`
- **`internal/replay`**
  - grows each frame from the totals and events up to that frame's moment

## 5. Testing

- **garden**
  - No plant fills more than its leaf budget of its silhouette, whatever
    its history: each species at 10, 500, 3,000 and 50,000 pushes. At
    3,000 pushes the shrub is a crown on a trunk, not a rectangle.
  - More pushes only ever add skeleton cells: the skeleton at `n` pushes
    is a subset of the skeleton at `n + 100`.
  - A leaf stays put while its push is inside the window, and only it
    leaves when the push ages out.
  - A repo quiet for a year keeps a third of its leaf budget in leaf,
    in the same cells as the day its last push aged out.
  - Flowers come only from merges in the last 90 days, at most a quarter
    of the budget. Fruit comes only from the past year, at most four.
  - Grown from a complete history, and from its totals plus what `recent`
    fetches, a plant is identical.
  - Growing a 50,000-push plant takes under 2 ms.
- **github** (against a fake GraphQL server)
  - `recent` asks for:
    - the totals
    - commits since 90 days ago, and the newest commit when none fall in
      the window
    - merged PRs newest first, stopping at the window
    - the ten newest releases
    - the first page of open issues, and issues closed in the window
  - `full` asks for everything, as before, plus the totals.
  - A cache from before v0.8 loads, with totals counted from its history.
  - A complete cache serves `recent` without fetching history again.
- **config**
  - `history = full` and `history = recent` are read.
  - Any other value warns and falls back to `recent`.
  - `gag config` prints the setting.
- **cmd and replay**
  - Replay in `recent` mode fetches the full history for its repo.
  - The demo garden grows through the new path; the golden frame is
    updated once.
  - `-simulate` thins the canopy.

## Out of scope

- **Visiting other people's gardens** from inside the running garden. It
  comes next, as its own design. Choices so far:
  - it happens inside the running garden
  - you type a name, with suggestions from the people you follow and
    from your recent visits
  - what shows while a new garden loads is still open
- Changing the windows (90 days, 30 days, a year) in the config file.
- New species, or changing a species' look beyond its silhouette.
- Fetching for the browser demo, which has no GitHub access.
