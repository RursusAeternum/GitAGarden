# Plant selection and a detail card: design

Date: 2026-09-28
Status: approved in conversation, pending written-spec review
Builds on: `2026-09-27-living-garden-design.md` (its "Reactions" phase) and
`2026-09-28-star-sky-and-config-design.md` (v0.5.0 as shipped)

## Intent

The garden shows *that* a plant is wilting, stormy or covered in buds. v0.6
lets you ask it *why*. Pick a plant with the arrow keys or a click, and a
**detail card** opens next to it with the facts behind its look. It's only
there when you ask. With nothing selected the garden is exactly the ambient
screen it is today.

This is the first half of the living-garden spec's "Reactions" phase. The
change animations (a splash on push, a bud bursting on merge, and so on)
follow as v0.7.

**Decisions made in conversation**

| Topic | Decision |
|---|---|
| When the card shows | Only when asked: no idle tour |
| Where the card sits | A floating card beside the selected plant, over the garden |
| Mouse | On. A click only selects; `enter` opens the card, as with the keyboard |
| How it's drawn | Into the garden's own pixel canvas (pixels plus text cells), not spliced in as styled text |
| Small v0.5 fixes | Folded in as a separate polish task |

## 1. Selection

- **Keys.**
  - `←` and `→` step through the plants in reading order: along a row of
    plants left to right, then on to the next row, wrapping at both ends.
    When the garden pans, reading order runs across all its columns, not
    only the visible ones.
  - With nothing selected, `→` selects the first plant on screen and `←`
    the last.
- **Help line.** It becomes
  `←/→ select · enter details · r refresh · t ticker · ? help · q quit`.
- **Mouse.** The live view turns on mouse reporting for clicks (Bubble Tea's
  cell-motion mode).
  - A left click anywhere in a plant's slot selects that plant. The slot is
    the plant's column of the bed, from its sky down to its label.
  - A left click on open sky, bare ground or the ticker line clears the
    selection.
  - A click inside an open card does nothing. Other buttons, the wheel and
    drags are ignored.
  - The README notes that ⌥ Option-drag still selects text while GAG runs.
- **Highlight.** The ground strip under the selected plant brightens and its
  name label turns bright white. Nothing else in the scene changes.
- **Camera.** This applies when the garden is wider than the window and pans.
  - While a plant is selected, the automatic panning stops.
  - If the selected plant is off screen, the camera slides, with the
    existing slide timing, to the nearest position that shows it.
  - When the selection clears, automatic panning resumes from the current
    position, with no jump.
- **Timeout.** After 60 s without a key press or click, the selection clears
  and any open card closes. Window resizes and refreshes don't count as
  input.
- **Refreshes.**
  - The selection belongs to a repo, by name. Plants keep their order across
    refreshes, so it stays on the same plant.
  - If that repo leaves the garden, the selection clears and its card closes.
- **`esc`.**
  - With a card open, it closes the card; the plant stays selected.
  - With only a selection, it clears the selection.
  - With nothing selected, it quits, as it does today.
- **`q` and `ctrl+c`** always quit.
- **Scope.** Selection exists only in the live view. `--once` and replay are
  unchanged.

## 2. The detail card

```
┌ gag-core ────────────── Go · shrub ┐
│ RursusAeternum/gag-core       ★ 31 │
│ state    thriving · tended 2h ago  │
│ last     Make storms readable, ke… │
│ 14 days  ▁▃▅█▂▁▁▃▆▇▂▁▄█ 23 commits │
│ CI       ✓ passing on main         │
│ PRs      2 open · oldest 10d       │
│          #12 Add sparkline… (2d)   │
│          #9 Bump deps (10d)        │
│ issues   4 open · newest 2d ago    │
│          #31 Crash on empty repo   │
│ release  v0.5.0 · 1h ago           │
└────────────────────────────────────┘
```

- **Opening and closing.**
  - `enter` with a plant selected opens its card, and `enter` again closes it.
  - `enter` with nothing selected does nothing.
  - While the card is open, `←`/`→` move the selection, and the card follows
    to the neighbouring plant.
- **Title row.** The repo's short name on the left. The language and species
  (`Go · shrub`) on the right, or only the species when the language is
  unknown.
- **Lines.** In this order:

  | Label | Content | When there's nothing |
  |---|---|---|
  | — | `owner/name`, and `★ N` on the right | `★` part omitted at 0 stars |
  | `state` | `thriving`, `wilting · 38d quiet`, or `under glass` for finished repos, then `tended <age> ago` | `no activity yet` |
  | `last` | the last commit's headline | `no commits yet` |
  | `14 days` | a sparkline of commits per local day, oldest first, then `N commits` | all-zero sparkline, `0 commits` |
  | `CI` | `✓ passing on <branch>`, `✗ failing on <branch>`, `● running on <branch>` | `– no checks` |
  | `PRs` | `N open · oldest Nd`, then up to two lines `#N title (age)`, oldest first, then `+N more` | `none open` |
  | `issues` | `N open · newest <age> ago`, then one line `#N title` for the newest | `none open` |
  | `release` | `tag · <age> ago` | `none yet` |

  - PRs are the non-draft open PRs, the same ones the plant shows as buds.
    When drafts exist, the PR summary ends with `· +N draft`.
  - The sparkline uses `▁▂▃▄▅▆▇█` scaled to the busiest of the 14 days, and
    `·` for a day with no commits.
  - Ages use the ticker's existing short forms (`2h`, `10d`).
- **Size.**
  - The card is 38 columns wide, or the window width minus 2 when narrower.
  - It is as tall as its lines, and at most as tall as the garden area. The
    garden area is the window minus the ticker line when the ticker shows.
- **Placement.**
  - The card's top is level with the top of the selected plant's row of
    beds, clamped into the garden area.
  - It opens on whichever side of the plant's slot has more room. If neither
    side fits the card, it is centred horizontally over the garden.
  - It never extends past the window.
- **Short windows.** Lines drop in this order until the card fits:
  1. the PR title lines
  2. the issue title line
  3. `release`
  4. `14 days`
  5. the `owner/name` line
  6. `last`
  7. `issues`
  8. `PRs`

  The title row, `state` and `CI` always stay.
- **Narrow windows.** Values are trimmed with `…` to the card's width.
- **Messy text.** Before they reach the card, commit headlines and PR and
  issue titles lose control characters, and every rune wider than one column
  (emoji, CJK) becomes `?`. The canvas's text cells are one column wide, so
  anything wider would shift the row. Every glyph the card itself uses
  (`┌─┐│└┘★✓✗●–·…` and the sparkline blocks) is one column wide under
  go-runewidth.
- **Live.** The card is rebuilt every frame from the current data, so ages
  count up and a refresh updates it in place.
- **Colours.**
  - the card's background is the ticker's dark background
  - the border and labels are muted
  - values are light
  - `✗ failing` is red and `✓ passing` green
  - the release tag is gold, matching the latest release's glint in the
    garden
- **Fallback.** The 256-colour fallback applies to the card like everything
  else.

## 3. Data

No new GitHub queries: everything is already fetched and cached.

- **`live.Repo.Detail`** holds:
  - full name (`owner/name`) and language
  - the last commit's headline and time
  - commits per local day for the last 14 days, oldest first
  - non-draft open PRs, each with number, title and when it was opened
  - the number of draft PRs
  - the open issue count, and the newest open issue's number, title and
    time
  - the latest release's tag and time
- **Filling it.** `cmd/gag`'s `repoFor` fills `Detail` from `github.Repo`.
  Offline, the data comes from the cache as usual.
- **The demo.** The demo garden gets made-up details: commit headlines from
  its fake history, a few PR and issue titles, and release tags. With them,
  `gag -demo` shows real-looking cards.

## 4. Polish (the v0.5 deferred minors)

1. **Decay.** `decay` in the config file rejects `nan` and `±inf` with the
   usual warning.
2. **Queued shooting stars.** A refresh's shooting stars queue after any
   still waiting from an earlier refresh, keeping the 2 s gap, instead of
   overlapping them.
3. **Late start.** A newly queued shooting star starts no sooner than one
   idle frame (500 ms) after it's queued. By then the fast frame rate has
   started, so it never hangs at the start of its flight.
4. **Tests pinning the wiring.** Tests check that the `sky` setting reaches
   the live view, `--once` and replay, and pin `gag config`'s output. This
   needs a small refactor: `--once` builds its `scene.View` in a function a
   test can call, and `gag config` writes to an `io.Writer`.
5. **Merged notes.** New stars for a repo whose ticker note is still showing
   are added to that note (`⭐ 3 new stars on gag-core`), and its minute
   restarts, instead of adding a second note.
6. **Spec wording.** The v0.5 spec's §1 is corrected: repos served from
   cache within the TTL take the fresh star count from the listing.

## 5. Units

- **`internal/scene`**
  - `View.Selected` (the selected plot's index, −1 for none) and
    `View.Card` (a `*Card`, nil for none).
  - The highlight on the selected plant's ground strip and label.
  - `Card`: a title, a right-hand title note, and lines, each with a label,
    a value, an optional right-hand value and a value colour.
  - `DrawCard`: draws the box, placed beside the selected plant or centred.
  - `PlotAt(v View, col, row int) (index int, ok bool)`: maps a terminal
    cell to the plot drawn there, using the same slot layout as `Draw`,
    pan included.
  - With `Selected` −1 and `Card` nil, `Draw` produces exactly today's frame.
- **`internal/live`**
  - `CardFor(r Repo, now time.Time, decay float64, rows, cols int) scene.Card`
    formats the lines, cleans the text, and drops lines to fit.
  - The model holds the selection (by repo name), whether the card is open,
    the last input time, and the camera's pause and resume. It also handles
    the keys and mouse clicks above.
- **`cmd/gag`**
  - `repoFor` fills `Detail`, and the demo repos get details.
  - The live program starts with mouse cell motion on.
  - The README gets the keys, the mouse and the Option-drag note.
  - The polish refactors from §4.

## 6. Testing

- **scene**
  - `PlotAt` agrees with where `Draw` puts each plant: every cell of a
    plant's slot maps to it, and sky or ground outside the slots maps to
    none. It holds in a garden of several beds and mid-pan.
  - Only the selected plant's ground strip and label change.
  - A card stays inside the window at 80×24, at the 24×12 minimum and at
    240×65. It sits beside the plant when there's room and is centred when
    there isn't.
  - A card never writes a rune wider than one column.
  - The golden frame is unchanged with nothing selected.
- **live**
  - `←`/`→` order, including wrapping and a panning garden.
  - Clicks on a plant, on the sky and on the ticker.
  - `enter` and `esc`, including `esc` quitting with nothing selected.
  - The 60 s timeout.
  - The selection following its repo across refreshes, and clearing when the
    repo leaves.
  - The camera pausing, sliding to an off-screen selection, and resuming
    without a jump.
  - Card wording for every "nothing" case, and text cleanup of emoji,
    control characters and long titles.
  - Lines dropping in the documented order in short windows.
  - The frame still exactly fitting the window with a card open.
- **cmd**
  - `repoFor` fills `Detail` from a `github.Repo`: last commit, daily
    counts, PRs with drafts left out, open issues, latest release.
  - Every demo repo has details.
- **Polish.** Each of the fixes in §4 comes with a failing test first.
- **Frame budget.** A 240×65 live frame with a card open renders in under
  10 ms.

## Out of scope

- The change animations: v0.7.
- An idle tour, hover effects, `↑`/`↓` navigation, and scrolling inside the
  card.
- Opening a repo, PR or issue in the browser from the card.
- A setting to turn the mouse off.
