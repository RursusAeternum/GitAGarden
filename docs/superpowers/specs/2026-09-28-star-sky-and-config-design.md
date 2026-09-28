# Star sky, shooting stars and a config file: design

Date: 2026-09-28
Status: approved in conversation, pending written-spec review
Builds on: `2026-09-27-living-garden-design.md` (v0.4.0 as shipped)

## Intent

The night sky should be able to show **the GitHub stars your repos have
earned**. When someone stars one of your repos while GAG is running, a
**shooting star** marks the moment. Because this changes the look, the star
sky is a **setting**. GAG gets its first **config file** to hold it.

**Decisions made in conversation**

| Topic | Decision |
|---|---|
| What the stars mean | One shared sky: the total stars across the repos in the garden, not a patch per repo |
| Shooting stars | On for everyone, whatever the sky setting |
| Naming the repo | The ticker says which repo got the star, for a minute |
| How to enable the star sky | A config file with `sky = stars`; the random sky stays the default |
| Other config keys | The file also accepts `limit`, `repos`, `user`, `refresh`, `decay`; flags win over the file |

## 1. Data

- Add `stargazerCount` to the repo metadata query (`metaFields`), used by
  `ListRepos`, `ListOwnerRepos` and `LookupRepo`. It costs no extra API calls.
- Store it as `github.Repo.Stars` (JSON `stars`) in the cache. A repo served
  from cache within the TTL still takes the fresh count from the listing;
  only offline loads use cached counts. Caches from older versions load
  with 0 until the next listing.
- The garden's star total is the sum of `Stars` over the repos being shown.

## 2. The star sky (`sky = stars`)

- **Night only**, like the random stars today: their brightness follows
  `Darkness`, and the day sky is unchanged.
- **One dot per GitHub star.** The sky has room for
  `capacity = W × bandHeight / 40` dots, the density the random sky uses
  today.
  - If the total fits, exactly `total` dots are drawn.
  - Otherwise `capacity` dots are drawn, and the whole sky is brighter by
    `min(1, log10(total / capacity) / 2)` of the way to full brightness.
    A popular garden looks rich, not cluttered.
- **Stable positions.** Star *i* gets its position and base brightness from
  a hash of *i* alone, in the upper two thirds of the sky band, so a new star
  adds one dot and never reshuffles the others. Positions depend on the
  canvas size, so resizing the window may move them; that's acceptable.
- A garden with 0 stars has an empty night sky, apart from the moon.
- `--once`, the live view and replay all use the configured sky. Replay
  uses the replayed repo's star count with `-repo`, and 0 (an empty night
  sky) for a fake history.

## 3. Shooting stars (always on, live view only)

- **Trigger.** After a successful load, compare each repo's `Stars` with the
  previous successful **online** snapshot. A repo whose count went up earned
  new stars.
  - The first load never triggers.
  - Offline (cached) snapshots never trigger.
  - A count that went down is ignored.
  - At most **3 shooting stars per refresh**, whatever the total gained.
- **Animation.** A bright streak with a fading tail, about 1.5 s long,
  crosses the sky band diagonally from a random upper-left point. Several
  shooting stars queue about 2 s apart.
  - They are visible **day and night**: bright white-gold on the night sky,
    and white with a warm edge by day, so they stand out from the pale sky.
  - While a shooting star is in flight the frame rate is fast (125 ms).
- **Ticker note.** For 60 s after a shooting star, the ticker's first item
  is `⭐ New star on <repo>`, or `⭐ 3 new stars on <repo>`. If several repos
  gained stars, each gets its own item. It sits ahead of the attention
  items: it's rare and fleeting.
- **Always on.** The trigger and animation work with the random sky too. In
  `--once` and replay there is no earlier snapshot to compare with, so there
  are no shooting stars.

## 4. Config file

- **Path.** `$XDG_CONFIG_HOME/gag/config` if `XDG_CONFIG_HOME` is set,
  otherwise `~/.config/gag/config`. This is the same path on macOS, Linux and
  the Pi. (Go's `os.UserConfigDir` isn't used on macOS on purpose.)
- **Format.** One `key = value` per line. `#` starts a comment, and blank
  lines are ignored. Whitespace around keys and values is trimmed, and values
  may not contain `#`. There are no new dependencies.
- **Keys:**

| Key | Values | Default | Same as |
|---|---|---|---|
| `sky` | `random` or `stars` | `random` | — |
| `limit` | integer ≥ 1 | 8 | `-limit` |
| `repos` | comma-separated `owner/name` list | (your recent repos) | `-repos` |
| `user` | GitHub login | (you) | `-user` |
| `refresh` | Go duration ≥ 30s, e.g. `5m` | 5m | `-refresh` |
| `decay` | days, number > 0 | 45 | `-decay` |

- **Precedence.** Flags given on the command line win over the file, the
  file wins over the defaults, and flags not given take the file's value. It
  applies to `gag` and `gag garden`. `gag replay` reads only `sky`.
- **Problems never stop GAG.** A missing file means defaults, silently. An
  unreadable file, an unknown key or an invalid value prints one line to
  stderr **before** the live view opens, for example:
  `gag: config line 3: sky "sparkly" isn't random or stars; using random`.
  The rest of the file still applies.
- **`gag config`** prints the file's path (and whether it exists) and each
  setting with its value and where it came from (`default`, `file`):

  ```
  config: /Users/me/.config/gag/config
  sky      stars   (file)
  limit    8       (default)
  repos    -       (default)
  user     -       (default)
  refresh  5m      (default)
  decay    45      (default)
  ```

## 5. Units

- `internal/config`: `Load(path) (Config, []Warning)`, `Path()`, the
  `Config` struct with a source per field, and the parser. Pure, no terminal
  output.
- `internal/github`: `Repo.Stars` and `stargazerCount` in `metaFields`.
- `internal/scene`: the star sky (count, stable positions, capacity and
  brightness), shooting-star drawing, and a `View` field for the sky mode,
  star total and shooting stars in flight.
- `internal/live`: detecting new stars between online snapshots, queueing
  shooting stars, the ticker note, and the fast frame rate while one flies.
- `cmd/gag`: loading the config, merging it with flags, the `gag config`
  command, and passing the sky mode and star total down.

## 6. Testing

- **config:** comments, blank lines, spacing, each key's valid and invalid
  values, unknown keys, warnings carrying line numbers, flag > file >
  default precedence, `XDG_CONFIG_HOME` handling.
- **github:** `stargazerCount` parsed from the listing (fake GraphQL server),
  kept for cached repos, and 0 for old caches.
- **scene:**
  - exactly `total` star dots when the total fits, and `capacity` when it doesn't
  - existing dots unchanged when the total grows by one
  - no stars by day, and random mode unchanged, so the golden frame is unchanged
  - a shooting star draws pixels during its 1.5 s and none after
  - it stands out from both the day and the night sky (colour distance)
- **live:**
  - the first load, offline snapshots and decreases don't trigger
  - an increase triggers one shooting star per new star, at most 3
  - the ticker note comes first for 60 s and names the repo
  - the fast frame rate while one is in flight
- **cmd:** `gag config` output, and the config reaching the live view (sky mode, limit).

## Out of scope

- Per-repo patches of stars, and twinkling. Twinkling can come with a
  later polish pass.
- Writing the config file from GAG (`gag config set …`). Users edit the
  file.
- Other v0.5 Reactions. This ships as its own small release before them.
