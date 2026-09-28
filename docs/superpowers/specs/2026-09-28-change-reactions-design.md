# Change reactions: design

Date: 2026-09-28
Status: approved in conversation, pending written-spec review
Builds on: `2026-09-27-living-garden-design.md` (the second half of its
"Reactions" phase) and `2026-09-28-selection-and-detail-card-design.md`
(v0.6)

## Intent

Until now the garden shows *how things are*. v0.7 makes it react when
something *happens*: a push waters the plant, a merge bursts a bud, a release
sparkles, a new issue sprouts a weed, and a failing build rolls in a storm.
The animation catches your eye, and a ticker note says what it was. The
screen gets more alive and more informative together.

**Decisions made in conversation**

| Topic | Decision |
|---|---|
| Telling you what happened | A ticker note for every change, a minute long, shown first |
| Changes on plants off screen | The camera goes to them, unless a plant is selected |
| The demo | Acts out a change every 30 s, cycling through all seven kinds |
| How it's built | Reactions are timed effects in `scene`: each frame is a pure function of time |
| Easter egg | Commits by an AI coding agent are watered by a small drone, not a can |

## 1. Changes

Each **online** refresh is compared with the previous one, repo by repo:

| Change | Detected when | Ticker note |
|---|---|---|
| push | the newest default-branch commit is newer than before | `💧 gag-core: "Draw the card beside the plant"`, then ` · 3 commits` when there are several and ` · by Claude` for an agent's commit |
| merge | the plant counts more merges than before | `🌸 gag-core: merged #41 Add a detail card`, then ` · +1 more` when there are several |
| release | the latest release has a new tag | `✨ gag-core: released v0.6.0` |
| new issue | the newest open issue is newer than before | `🐛 gag-core: #31 Crash on empty repo` |
| closed issue | the most recently closed issue is newer than before | `✅ gag-core: closed #30 Old bug` |
| CI red | CI is failing now and wasn't before | `⚡ gag-core: CI failing on main` |
| CI green | CI was failing and now passes | `🌈 gag-core: CI passing again` |

- **Never a change:**
  - the first load, which is the baseline
  - offline or cached loads
  - repos that just joined the garden
  - repos sharing a short name with another
  - finished repos, which rest under glass
  - the switch from the no-token demo garden to a real one, which starts a
    fresh baseline

  These are the same rules the shooting stars follow.
- **Bursts collapse:** a refresh gives each plant at most one reaction of
  each kind. The note names the newest item, and the counts cover the rest.
- **Order:** a plant's reactions from one refresh play in the order push,
  merge, release, new issue, closed issue, CI. They play one at a time,
  starting at least 3 s apart.
- **Several plants at once:** plants on screen play together.
- **Plants off screen:** when the garden pans, the camera visits each changed
  plant that's off screen, one after another in garden order.
  - It slides to show the plant, and that plant's reactions start when the
    camera arrives.
  - The camera holds until they end, plus a second, then moves on.
  - After the last visit, automatic panning resumes from there.
  - With a plant selected, the camera never moves. The reactions play
    unseen, and their notes still appear.
- **Notes:**
  - A change's note appears when its reaction starts and lasts a minute. It
    sits ahead of the attention items, and a new note restarts the ticker's
    rotation, as the star notes do today.
  - Star notes become one kind of change note, with one shared rule.
- **Frame rate:** fast (125 ms) while any reaction is playing or queued.
- **Where:** only in the live view. `--once` and replay never react.

## 2. The animations

Each reaction is drawn over its plant's slot, lasts 2–4 s, and plays by day
and night.

| Kind | Animation | The plant takes its new shape |
|---|---|---|
| push 💧 | a small watering can tips above the plant; an arc of steel-blue drops falls onto it | when the drops land (0.8 s); new cells unfurl from the stem outward, fading in over 1 s |
| push by an agent 💧 | a small grey drone with blurred rotors flies in from the side, hovers over the plant, sprinkles the same arc of drops, and flies off | when the drops land |
| merge 🌸 | a bud swells on the new flower's spot and bursts into petals; a butterfly lifts off and flies away | at the burst (0.6 s) |
| release ✨ | a burst of gold and white sparkles around the plant; two bees circle it for 3 s and leave | at the start |
| new issue 🐛 | the new weed grows up from the pot's base out of nothing, with a small puff of dirt | at the start; the weed's cells grow in over 1 s |
| closed issue ✅ | the pulled weed lifts out of the soil, flies up and fades | at the end |
| CI red ⚡ | the storm cloud slides in from above and darkens; rain starts when it arrives, with one lightning flash | when the cloud arrives (1.5 s); until then, no steady storm |
| CI green 🌈 | the storm cloud drifts off and fades; a small rainbow arcs over the plant for 4 s, then fades | at the start |

- **Old shape, new shape:** a reaction knows the plant's shape from before
  its refresh.
  - The plant's new shape arrives with the first of its reactions from that
    refresh. Later ones in the same refresh use the old shape only to find
    their spot, such as the new flower or the new weed.
  - Until the "new shape" moment the old shape is drawn. After it, cells new
    since the old shape fade or grow in, and cells gone from it vanish,
    except a pulled weed, which lifts away.
- **Where effects draw:**
  - They stay inside their plant's slot, except the butterfly, bees and
    drone, which fly beyond it.
  - A detail card drawn over the plant stays on top of everything.
- **Colours stand out:** drops, sparkles, rainbow and drone stand out
  against both the day and the night sky, checked by colour distance as the
  storm and shooting stars are.
- **Finished plants** never react.

### Agents water by drone

- **Which pushes:** a push counts as an agent's when the newest new commit
  has an AI coding agent among its authors.
- **Authors:** GitHub's commit data lists them all, including those named in
  `Co-Authored-By:` trailers. The history query asks for up to three
  authors per commit.
- **Matching:** each author is matched by **email or GitHub login, never by
  name alone**, so a person named Claude still gets the watering can. The
  one table of patterns, as known at writing:

  | Agent | Email or login |
  |---|---|
  | Claude | `noreply@anthropic.com`; `claude`, `claude[bot]` |
  | Codex | any `@openai.com` address; `chatgpt-codex-connector[bot]` |
  | Copilot | `Copilot`, `copilot-swe-agent[bot]` |
  | Devin | `devin-ai-integration[bot]` |
  | Cursor | `cursoragent@cursor.com` |
  | Gemini | `gemini-code-assist[bot]` |
  | Jules | `google-labs-jules[bot]` |
  | aider | an author name ending in `(aider)`: aider's own marker, not a person's name |

  A GitHub bot's commit email (`<id>+<login>@users.noreply.github.com`)
  counts as that login.
- **Older caches:** commits cached before v0.7 carry no agent. Only new
  pushes matter, and those are fetched fresh.

## 3. The demo

- **A fake world:** `gag garden -demo` and the browser demo keep a small fake
  world for the session. The first load shows it as today. Every load after
  it applies **one scripted change** to the world, and that change goes
  through the same comparison, queue and notes as a real garden.
- **Pace:** the demo refreshes every **30 s**, so pressing `r` plays the next
  change at once. The no-token fallback garden stays still.
- **The cycle:** push, merge, release, new issue, closed issue, CI red, CI
  green, and round again.
  - Each change goes to a random unfinished demo plant for which it makes
    sense: a closed issue needs an open one, CI red a plant not failing, CI
    green a failing one.
  - Every other push is by Claude, so the drone shows up.
- **Real-looking details:** headlines, PR and issue titles and version
  numbers come from small lists, so notes read like
  `💧 tiny-cli: "Handle empty input" · by Claude` and
  `✨ gag-core: released v0.7.0`. The plants really grow, and their cards
  update.
- **Unchanged:** `--once -demo` still prints one still frame.

## 4. Data and units

- **`internal/github`**
  - The history query asks for each commit's authors.
  - `Commit.Agent` (JSON `agent`, omitted when empty) holds the agent's
    display name.
  - The agent table and `agentOf(authors)` live here.
- **`internal/live`**
  - `Entry.By` holds the agent's name.
  - `Detail` gains `LastMerge` (the newest merged PR) and `LastClosed` (the
    most recently closed issue). `LastCommit.By` is filled in, and the
    plant's counters give how many pushes and merges there were.
  - `changes.go`: a pure comparison of two snapshots' repos into
    `[]Change`, with the rules and note wording of §1.
  - The model:
    - queues each plant's reactions and schedules them, with camera visits
      for plants off screen
    - adds a change's note when its reaction starts
    - keeps fast frames while anything is queued or playing
- **`internal/scene`**
  - `Reaction`: which plot, what kind, when it starts, the old shape, whether
    it's a drone, and a seed. `View.Reactions` holds them.
  - One drawing function per kind, driven by the reaction's age.
  - Painting the plant's old shape, then its new shape with cells fading in
    or lifting away.
  - Reactions draw after the plants and under the card.
  - With no reactions, `Draw` produces exactly today's frame.
- **`internal/garden`**
  - `Paint` learns to fade in the cells a previous shape lacked.
- **`cmd/gag`**
  - `detailFor` fills `LastMerge`, `LastClosed` and `LastCommit.By`.
  - The demo world and its scripted steps.
  - A 30 s refresh for `-demo` and the browser demo.

## 5. Testing

- **github:** a fake GraphQL server returns four commits:
  - one by Claude through a co-author trailer
  - one by the Codex bot
  - one by Copilot
  - one by a person named "Claude Monet" with their own email

  Only the first three are agents.
- **live**
  - Each of the seven kinds is detected and its note worded exactly,
    including ` · 3 commits`, ` · by Claude` and ` · +1 more`.
  - The first load, offline loads, joining repos, same-name repos and
    finished repos stay quiet. So does the demo-to-real switch.
  - Ten pushes in one refresh make one reaction.
  - A plant's reactions play in order, at least 3 s apart.
  - The camera visits an off-screen plant, the reaction starts when it
    arrives, then panning resumes. With a plant selected, the camera holds
    still and the notes still appear.
  - Frames are fast while anything is queued or playing.
- **scene**
  - Each kind draws during its span and nothing after.
  - The old shape is drawn before the "new shape" moment, and the new cells
    after it.
  - The drone replaces the can.
  - Drops, sparkles, rainbow and drone stand out against the day and night
    sky.
  - Effects stay inside their slot, except the flying ones, and a card
    still covers everything.
  - With no reactions, the golden frame is unchanged.
- **cmd**
  - Seven demo steps cover all seven kinds, each on a plant where it makes
    sense.
  - The demo's pushes alternate between drone and can.
  - `detailFor` fills the new fields.
- **Frame budget:** a 240×65 frame with a reaction playing on every plant
  renders in under 10 ms.

## Out of scope

- Sound, and notifications outside the terminal.
- Reactions for stars beyond v0.5's shooting stars, and for PRs opening or
  closing unmerged.
- A setting to turn reactions off.
- Mouse input in the browser demo (a separate follow-up).
