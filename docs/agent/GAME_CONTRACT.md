---
id: game-host-contract
status: binding
go_package: github.com/KroniK907/grabbag/internal/games
game_type: games.Game
helper_type: games.Helper
host_helper: internal/host/helper.go
host_lifecycle: internal/host/runtime.go
catalog_import: internal/host/handler.go
reference_game: internal/games/testing
decisions: CORE-HOST-GM-011 through CORE-HOST-GM-020
---

# Game / host contract

Binding. The Go types in `internal/games/game.go` win if this page and the code disagree. Update this file in the same change as the interfaces.

This page is the short, agent-parseable contract. The human API reference is [`docs/game-contract.html`](../game-contract.html), also at `GET /docs/game-contract.html` on a running host. That page has a jump list, a signature box, parameters, return value, a longer explanation, and named examples for every Game method, Helper method, loader func, and host HTTP route.

Operator playbook is `/docs`.

## Jump list

- [Machine summary](#machine-summary)
- [Who owns what](#who-owns-what)
- [What host needs to load a game](#what-host-needs-to-load-a-game)
- [Lifecycle](#lifecycle)
- [Game hooks](#game-hooks)
- [Helper hooks](#helper-hooks)
- [HTTP mounts](#http-mounts)
- [Extra pages](#extra-pages)
- [Shells](#shells)
- [Live SSE](#live-sse)
- [Player record](#player-record)
- [Disconnect policy](#disconnect-policy)
- [UI previews](#ui-previews)
- [Runtime kit](#runtime-kit)
- [Examples](#examples)
- [Must not](#must-not)

## Machine summary

```yaml
load:
  compile_time:
    - package under internal/games/<name>
    - init calls games.Register with non-empty ID and New
    - host blank-imports the package (see internal/host/handler.go)
    - New() returns a value that implements games.Game
  runtime:
    - POST /settings/load game_id=<ID>
    - catalog contains ID
    - round is not started
    - if MaxPlayers > 0, seated count <= MaxPlayers
    - host mkdir <dataDir>/games/<ID>
    - Game.Load(helper) returns nil
lifecycle_order:
  - Load
  - Start
  - Pause or Resume (optional, may no-op)
  - Stop   # keeps package loaded; KV stays
  - Shutdown  # unloads; KV stays
host_calls_on_game:
  - ID Name MinPlayers MaxPlayers
  - Load Settings Start
  - Board BoardButtons Phone Play   # Board and Phone write fragments
  - Assets Scenarios                # ui.Tenant, embedded in games.Game
  - Pause Resume Stop Shutdown
game_calls_on_helper:
  - Seated Waiting Audience Player PlayerFromRequest
  - DataDir KVGet KVSet
  - Finish Pause Resume Publish Log
  - Theme HasAdmin
shells:
  documents: [/, /board]           # load once; tenants swap inside #shell-stage
  tenant_fetch: [GET /tenant, GET /board/tenant]
  tenants: [lobby, locked, <game id>]
  js: grabbagShell.register(id, {board: {mount, unmount}, phone: {mount, unmount}})
  refetch_on: [sse round, sse tenant, "HX-Trigger: grabbag:tenant", sse reconnect]
  reloads_only: [hardReload("kicked"), hardReload("host-restarted")]
mounts:
  phone_shell: /
  board_shell: /board
  operator: /settings
  game_settings: /settings/game/   # after Load
  game_play: /play/                # GET after Load; POST only after Start
urls_game_must_not_own: ["/", "/board", "/settings"]
nil_ok: [Settings, Play]
optional:
  games.DisconnectPolicy: PauseOnDisconnect() bool  # false opts out of auto-pause on seated disconnect
sse:
  endpoint: GET /lobby/events
  connect: the shell owns the one EventSource; operator pages use ui-start
  locked_board: GET /shell/events/locked  # tenant and theme only
  helper: Publish(name)  # data payload is always "update"
  reserved: [roster, round, tenant, pause, theme, log, notice]
  pattern: named event then hx-get current HTML, not a delta
  drop: slow pages miss events (non-blocking send, buffer 16)
```

## Who owns what

Lobby owns roster, join, wait list, audience, seats, identity, operator knobs, and the start gate. Games read those facts through Helper. They never write them.

A game owns run logic, per-player run state keyed by Lobby player id, optional extra pages, and after Start the board and phone fragments the shells show.

Host drives Load, Start, Stop, and Shutdown. Load is in-process. No folder scan. No child process.

## What host needs to load a game

Compile-time, in this order:

1. Add `internal/games/<name>` with a package that is not named `testing` (stdlib clash). Product Testing uses package `testinggame`.
2. Implement `games.Game`.
3. Call `games.Register` from `init`. `ID` is the catalog key. `New` must not be nil. Empty ID or nil `New` is dropped with no error.
4. Blank-import the package from `internal/host/handler.go` so `games.Catalog()` includes it. `internal/games` cannot import children. Children already import `games` for Helper.

Runtime Load (`POST /settings/load`):

1. Admin session on the request.
2. Room restore prompt is not pending.
3. No round is started. Load during a round is refused.
4. `game_id` matches a catalog Factory ID.
5. If `MaxPlayers() > 0` and seated count is above that max, Load is refused. `MaxPlayers() == 0` means the game does not declare a cap. The Lobby seat cap still applies.
6. Host shuts down any previously loaded game, then constructs `Factory.New()`.
7. Host creates `<dataDir>/games/<ID>` (mode 0700).
8. Host calls `Load(helper)`. A non-nil error rolls the previous loaded id back.
9. Host stores the selected game id and applies the game's max to Lobby.

After Load, `/board` is still Lobby. Settings HTML from `GET /settings/game/` is inlined on `/settings` under Game Settings. `BoardButtons` may paint on the Lobby TV rail (up to three, skip empty Label or Path, omit `HostOnly` unless the request has an admin session).

Start needs a loaded game and at least one seated player. If `MinPlayers() > 0` and seated count is below that min, Start is refused. `MinPlayers() == 0` means the game does not declare a floor. Host still requires one seated player.

## Lifecycle

| Host action | Game method | `/` and `/board` | `/settings/game/` | `/play/` POST | KV |
|-------------|-------------|------------------|-------------------|---------------|----|
| Load | `Load` | Lobby | mounted | GET allowed, POST 404 | kept |
| Start | `Start` | game | mounted | mounted | kept |
| Pause | `Pause` | game | mounted | mounted | kept |
| Resume | `Resume` | game | mounted | mounted | kept |
| Stop | `Stop` | Lobby | mounted | GET allowed, POST 404 | kept |
| Shutdown | `Stop` if started, then `Shutdown` | Lobby | gone | 404 | kept |

Finish on Helper is Stop. Operator abort is the same Stop. If After a game is Cycle seats, Stop rotates the table. If it is Keep seats, sitters stay and empty seats fill from the wait list. Picking another game or Unload is Shutdown.

Stop wipes run state in the game. It does not unload the package. Start can run again. Shutdown drops the helper and clears the selected game id.

Operator "Clear game data" deletes that id's `game_kv` rows. Stop and Shutdown do not.

A process restart re-Loads the selected id with Lobby still on `/board`. It does not restore an in-progress Start.

## Game hooks

Host calls these on the Game value.

| Method | When | What to do |
|--------|------|------------|
| `ID() string` | catalog, logs, KV namespace, data dir | Stable lowercase key. Testing uses `testing`. |
| `Name() string` | Lobby rail after Load | Player-facing label. Empty Name falls back to ID. |
| `Description() string` | Host game library cards | Short blurb under the title. Empty hides the line. |
| `MinPlayers() int` | Start gate | 0 means no extra floor. |
| `MaxPlayers() int` | Load gate and Lobby max | 0 means no extra ceiling. |
| `Load(h Helper) error` | after construct | Store `h`. Read KV if knobs persist. Return nil on success. |
| `Settings() http.Handler` | after Load | Fragment mux stripped at `/settings/game`. Nil is fine. |
| `Start(h Helper) error` | operator or auto-start | Store `h` again. Begin ticks or round state. |
| `Board(w, r)` | board shell after Start | HTML fragment. No `<html>`, `<link>`, or `<script>`. The shell owns the document and the theme. |
| `BoardButtons() []BoardButton` | Lobby `/board` after Load, before Start | Up to three `{Label, Path, HostOnly}`. Paths are usually under `/play`. |
| `Phone(w, r)` | phone shell after Start, any signed-in player | Inner body fragment. Host wraps Leave and the claimed-host drawer. Unknown cookies stay on Lobby join. |
| `Play() http.Handler` | `/play/` | Game POSTs, partials, static. Nil is fine. StripPrefix leaves paths like `/tap`. |
| `Pause() error` | operator or auto-pause on seated disconnect (unless the game opts out, see [Disconnect policy](#disconnect-policy)) | Stop accepting play if that is the game's rule. May no-op. |
| `Resume() error` | operator | Restart ticks. May no-op. |
| `Stop() error` | round end | Drop in-memory run state. Keep KV. Keep helper. |
| `Shutdown() error` | unload | Drop helper. Stop goroutines. |
| `Assets() ui.Assets` | whenever a shell shows the game | `{CSS, JS, External}` for both surfaces. `runtimekit.Assets(id, version)` builds `/play/static/game.css` and `game.js` with the id in the query. External is web fonts and the like. |
| `Scenarios() []ui.Scenario` | `/dev/ui/` gallery, uishots, tests | Preview states. See [UI previews](#ui-previews). |

## Helper hooks

Host implements Helper. Games do not parse cookies, open `host.sqlite`, or write `host.log` themselves.

| Method | Returns | Notes |
|--------|---------|-------|
| `Seated()` | `[]Player` | Occupied seats. |
| `Waiting()` | `[]Player` | Wait list. Join during a round can land here and in audience. |
| `Audience()` | `[]Player` | Not seated. Includes waiters. |
| `Player(id)` | `(Player, bool)` | One roster row. |
| `PlayerFromRequest(r)` | `(Player, bool, error)` | Player cookie. Do not read the admin cookie. |
| `DataDir()` | `string` | `<dataDir>/games/<ID>` while loaded. Files you write here are yours. |
| `KVGet(key)` | `([]byte, bool, error)` | Opaque bytes. Namespaced by game id. Host does not parse values. |
| `KVSet(key, value)` | `error` | Same namespace. Survives Stop and Shutdown. |
| `Finish()` | | Graceful Stop. Use for End game. |
| `Pause()` | | Same as operator Pause. |
| `Resume()` | | Same as operator Resume. |
| `Publish(name)` | | Named SSE on the room hub. One EventSource per page. A slow page may miss an event. Clients fetch current state. |
| `Notify(target, typ, message, seconds)` | | Host toast. `target` is `board`, `host`, `seated`, `audience`, or `waiting`. Reserved SSE name `notice` with JSON `target`, `type`, `message`, `duration`. Empty message or unknown target is a no-op. Does not write the log. `seconds`: 0 until close, negative is 3s, above 30 clamps to 30. |
| `Log(line)` | | Prefixed with `<ID>: ` in the host log ring. No-op if nothing is loaded. |
| `Theme()` | `string` | `neon-light` or `neon-dark`. Stamp extra full documents (`/play/info`) so first paint matches `/settings`. Fragments inherit the shell's theme. |
| `HasAdmin(r)` | `bool` | Valid admin session. Use for board End game. Do not read the admin cookie yourself. |

## HTTP mounts

Public paths stay host-owned. Lobby vs game is a tenant swap inside the `/` and `/board` shells.

| Path | Owner after Load | Owner after Start |
|------|------------------|-------------------|
| `/` | phone shell, Lobby tenant | phone shell, game `Phone` wrapped by host, any signed-in player |
| `/board` | board shell, Lobby tenant plus `BoardButtons` | board shell, game `Board` |
| `/settings` | Host operator | Host operator, Game Settings fragment inlined |
| `/settings/game/*` | `Settings()` mux | same |
| `/play/*` | GET from `Play()` | GET and POST from `Play()` |

Forms in the settings fragment must post under `/settings/game/...`. Play forms post under `/play/...`.

After Stop, a GET to `/play` may still hit `Play()`. POST `/play` is 404 until the next Start.

### Extra pages

Full extra documents after Load are `Play()` GET routes (`GET /play/picker`, `GET /play/info`). Mutations that must run before Start POST through `Settings()` at `/settings/game/`. `/settings/game/` stays the inlined settings column. It is not those documents.

Do not add a Game method or host mount for extra pages. Do not open `Play()` POST before Start. v1 leaves the CoreHost mux unchanged.

Operator-only extra GET pages check `HasAdmin` in the game. Signed-out GET is plain `401` text, the same idea as host Settings with no admin session. Host does not make every `Play()` GET admin-only. Public extra pages (Testing `/play/info`) stay unsigned. Hiding a `HostOnly` `BoardButton` does not protect the URL.

Apples for Humanity is the first extra operator page: `GET /play/picker`, Lobby `HostOnly` button `Deck Library`, pack enable POSTs to `/settings/game/`. Success is `303` to `/play/picker`. A failed write or a refused-after-Start toggle returns `200` HTML so the checkboxes match stored state. `GET /play/howto` is a public extra document (Lobby `How to play` button, no `HasAdmin`). The in-round phone `?` loads `GET /play/howto-sheet`. Quick Quips uses the same picker mount as Prompt Library (`GET /play/picker`, `HostOnly`).

## Shells

`/board` (TV) and `/` (phones) are persistent shell documents. They load once. The shell owns the document, the one SSE connection, theme, notices, the connection overlay, and the heartbeat. Tenants (the Lobby, each game, and the host's `locked` board) swap in and out of `#shell-stage`. Go side: `ui.Tenant` is `Board`, `Phone`, `Assets`, `Scenarios`.

A transition fetches `GET /board/tenant` or `GET /tenant`: JSON with the fragment, assets, tenant id, a host generation, and the host boot ID. The shell loads assets it lacks, fades out, unmounts, swaps, mounts, and fades in (TV Lobby to game and back: 1.5s out, 0.5s in; other TV swaps 200ms; phones 150ms; instant under `prefers-reduced-motion`). Transitions run one at a time, the latest wins, older generations are dropped, and a fetch for the same id and generation (or the same HTML) swaps nothing.

### Register

`game.js` calls `register` at the top level and does nothing else there:

```js
grabbagShell.register("wordbox", {
  board: { mount(root, ctx) { ctx.on(document, "htmx:afterSwap", onSwap); ctx.every(1000, tick); } },
  phone: { mount(root, ctx) { ctx.on(window, "resize", sync); }, unmount() {} },
});
```

`unmount` is optional. The Lobby registers as `lobby` from `/lobby/static/lobby.js`.

### ctx

| Member | What it is |
|--------|-----------|
| `root`, `surface`, `tenant` | The stage, `board` or `phone`, and your id. |
| `signal` | `AbortSignal` that aborts when unmount begins. |
| `on(target, type, fn, opts)` | `addEventListener` with `signal`. Returns `off()`. |
| `every(ms, fn)`, `after(ms, fn)` | Timers cleared on unmount. Return `cancel()`. |
| `frame(fn)` | `requestAnimationFrame` loop. Return `false` to stop. Stops on unmount. |
| `cleanup(fn)` | Your own teardown. |
| `audio` | Reserved for board audio. |

Unmount order: abort `signal`, cancel ctx timers and frames, run `cleanup` callbacks newest first, call your `unmount()`, then the shell removes your CSS and swaps the markup. Listeners on elements inside your fragment and `hx-*` attributes need nothing. A `document` or `window` listener, timer, or frame you start outside ctx fails the browser suite's leak check.

### No reloads

Inside a shell nothing reloads. Every form is `hx-post`, never `<form method="post">`, and nothing calls `location.reload`. Answer htmx with a partial, or 204 when an event or the tenant trigger repaints. Do not redirect an htmx POST; htmx follows it and fetches a shell document. The shell reloads only through `grabbagShell.hardReload(reason)`, for `kicked` (a kick or a room clear) and `host-restarted` (a new boot ID after an SSE reconnect). A reconnect with the same boot ID refetches the tenant and theme, remounts, restarts the heartbeat, and hides the overlay without a reload. The overlay shows only after 3s down.

Extra pages (`/play/howto`, pickers, `/settings`, `/setup`) are normal documents and may reload.

### Changing the tenant

The shell refetches on the `round` and `tenant` SSE events, on reconnect, and on `HX-Trigger: grabbag:tenant`. Host sends the trigger and bumps the generation for Start, Stop, Load, Unload, Join, Leave, Stand, Sit, take-host, sign-in, auto-start, Keep and Clear, and the admin-only board switch. A game ends the round with `Helper.Finish`, which publishes `round`.

If a mount throws, the shell logs it to `/settings/log` through `POST /shell/error` and leaves the server-rendered fragment. If a tenant fetch fails, the current tenant stays and the shell retries (1, 2, 4, then every 10s) and says so after about 10s.

### Locked board

With admin-only board on, a board request with no admin session gets the `locked` tenant (a "Host screen only" card). The locked shell connects to `GET /shell/events/locked`, which carries only `tenant` and `theme`, never `log`, `notice`, or `roster`. Turning the switch off, or signing in from that browser, swaps it live and moves it to the full stream.

## Live SSE

One EventSource per page. On the shells it is the shell's own connection to `GET /lobby/events`, the in-process hub. For each named event the shell fires `sse:<name>` on every element whose `hx-trigger` lists it, as HTMX's SSE extension does. Operator pages use `ui-start`, which sets `hx-ext="sse"` and `sse-connect="/lobby/events"` on `body`. `ui-start-quiet` (setup and `/docs`) does not connect.

`Helper.Publish(name)` writes an SSE event with that name and data `update`. Games cannot set the data line. Host uses `PublishData` for `theme` (palette id), `log` (the log line), and `notice` (toast JSON). Put scores and names in a GET partial, not in the event body. Toasts are fire-and-forget. A missed `notice` is not replayed.

The page does not apply a delta from the event. It hears the name, then `hx-get`s current HTML. Include `htmx:sseOpen from:body` on those triggers; it fires when the shell's stream first opens. After a drop the shell remounts the whole tenant.

A subscriber channel buffers 16 events. A further publish to a slow page is dropped. Design for missed ticks. The next event, or `sseOpen`, should paint the truth.

Pings are SSE comments every 15s. They keep the stream alive. They are not named events.

### Reserved names

Do not publish these for game ticks. Host already owns them.

| Name | Who publishes | Typical refresh |
|------|---------------|-----------------|
| `roster` | host, Lobby, or a game knob that other pages should reread | roster partials |
| `round` | host on Start, Stop, Shutdown | shells refetch their tenant |
| `tenant` | host and Lobby when what shells show changed | shells refetch their tenant |
| `pause` | host on Pause and Resume | pause chrome and game partials |
| `theme` | host | `html[data-theme]` |
| `log` | host log ring | settings log tail |
| `notice` | host via `Helper.Notify` | chrome toast; client filters by target |

Publishing `roster` is fine when a settings knob changes a board that also shows Lobby facts. Publishing `round` or `tenant` from a game makes every shell refetch its tenant. Do not do that for a score tick.

### Your own names

Pick short lowercase names. Prefix with the game id when the word is generic (`wordbox-score`, not `score` if another package might reuse it). Only one game is loaded, but reserved host names still apply.

Call `h.Publish("tap")` after you mutate run state. In the template, listen on the same page's existing EventSource.

```html
<div
  id="board-list"
  hx-get="/play/partials/board-list"
  hx-trigger="sse:tap, sse:wordbox-tick, sse:roster, htmx:sseOpen from:body"
  hx-swap="outerHTML"
></div>
```

Register `GET /partials/board-list` on `Play()`. Do not add `sse-connect`. Do not open `EventSource` in your own script.

## Player record

Games may read these fields. They never write them.

| Field | Meaning |
|-------|---------|
| `ID` | Stable Lobby player id. Key run state by this. |
| `DisplayName` | Nickname. |
| `AvatarSeed` | Procedural SVG seed. Render with `ui.AvatarSVG`. Do not store image blobs. |
| `Seated` | Occupies a seat. |
| `Waiting` | On the wait list. |
| `Audience` | Not seated (`!Seated` in the host helper). |
| `ClaimedHost` | This player claimed host. Phone End game in Testing keys off this. |
| `Connected` | Heartbeat currently live. |
| `LastHeartbeatRTT` | Last RTT. Zero means none yet. |

## Disconnect policy

Optional. The operator's Auto-pause on seated disconnect is a room setting, off by default. When it is on, host pauses the round as soon as a seated phone drops. A Game that also implements `games.DisconnectPolicy` and returns false from `PauseOnDisconnect()` is skipped: host does not pause it, and `/settings` notes under the switch that the loaded game keeps playing.

Opt out when play never waits on one phone, so a phone that sleeps or drops should not stop the room. The game must then cope with a missing player itself, usually by letting the host Continue past them and by keeping run state on the server so the phone repaints the same moment when it returns. Borrowed Truths opts out. A Game without the method follows the operator setting. `games.PausesOnDisconnect(g)` is the check host uses.

```go
func (g *Game) PauseOnDisconnect() bool { return false }
```

Operator Pause from `/settings` and `Helper.Pause` still reach every game.

## UI previews

Every tenant lists `ui.Scenario` values from `Scenarios()`. Host serves them at `/dev/ui/` when started with `-dev-preview`, and `cmd/grabbag-uishots` screenshots them. Host calls `Scenarios()` on a fresh `New()` value. There is no Load, no Helper, and no data dir.

Board and phone scenarios are tenant fragments. Host wraps each in the real shell template in static mode (`ui.Chrome.Static`): no htmx, SSE, heartbeat, or transitions, with the tenant's `Assets()` linked. The static shell mounts the tenant once so layout code runs; skip animations when `window.grabbagStatic` is set. `uitest.RenderTenant` renders every scenario inside the shell and fails on a fragment that is a full document or carries `<script>` or `<link>`.

| Scenario field | Rule |
|----------------|------|
| `Surface`, `Name` | Unique pair. Lowercase, digits, dashes. |
| `Group` | Same value on the board and every phone of one moment. Drives the table view. |
| `Viewer` | `tv`, `judge`, `seated`, `host`, `audience`, `guest`, or `operator`. |
| `Frame` | `ui.FramePhone`, `ui.FrameTV`, or `ui.FramePage`. |
| `Shell` | Empty for a full document (extra pages). `ui.ShellBoard` for a board fragment. `ui.ShellPhone` for a whole phone tenant fragment (the Lobby). `ui.ShellPlayPhone` for a game phone body host wraps in the Lobby play-phone fragment. `ui.ShellSettings` for a settings fragment host inlines on `/settings`. |
| `Open` | Element ids the static shell opens when the request names none, such as a drawer. |
| `MinPlayers`, `MaxPlayers` | `MinPlayers > 0` turns on the seated sweep: min, middle, 12, max. `MaxPlayers 0` sweeps to the Lobby cap, 64. |
| `Sample` | Include in the phone and tablet device matrix. Keep it to one or two phone screens per game. |
| `Render` | Build the view from fixed data, then call `ui.RenderScenario`. Extra pages set `GameCSS` from `Preview.Asset`. No disk, network, or wall clock. |

Drive the real engine to each state where one exists, so previews cannot drift from play. Saved JSON merge patches go in `internal/games/<name>/previews/<surface>.<name>.<variant>.json` and load through `ui.WithVariants`. Add a `preview_test.go` that calls `uitest.RenderTenant(t, id, New())`.

## Runtime kit

Optional. `internal/games/runtimekit` is the match runtime most games would otherwise rebuild. Host never sees it. A game that wants its own runtime implements `games.Game` directly and does not import the kit. Apples for Humanity, Quick Quips, and Borrowed Truths use it. Testing does not.

Split a game in two:

- **Engine.** Rules and run state. No HTTP, no Helper, no goroutines. The clock comes in as an argument. A phase countdown is a `runtimekit.Timer` field.
- **Game.** Holds a `*runtimekit.Runner[*engine]` built from a `runtimekit.Config`, plus views, templates, and extra routes. `Load`, `Start`, `Pause`, `Resume`, `Stop`, and `Shutdown` hand off to the Runner.

The Runner owns:

| Piece | What it does |
|-------|--------------|
| Lock | One mutex for run state. Hooks run with it held. Game code outside a hook calls `Lock` / `Unlock`. |
| Tick | Calls `Config.Advance(e, now)` every `Config.Tick` (default 250ms) while a match runs and is not paused. |
| `Act`, `HostAct` | Player or host POST: sign-in or host check, form parse, 404 with no match, `Config.Gate`, the action, publish, host calls, `Config.Reply` rendered after unlock. |
| `HostPause`, `HostResume` | Ready-made phone host buttons. Resume skips the Gate. |
| `Result` | `Err` is the phone error. `Changed` publishes `Config.Event`. `Events` publish too. `Pause`, `Resume`, `Finish` call Helper with the lock released, because host calls back into the Game. |
| `Apply`, `Defer` | Apply a `Result` from game code outside a hook. `Defer` publishes now and holds the host calls for the next Apply or tick, for code that must keep the lock, such as a view. |
| `Hold` | `Config.Hold(e, paused, now)` on Pause and Resume. Fold in any other hold, such as a reshuffle overlay, and call `Runner.Hold` when it changes. |
| `Cleanup` | Runs on Stop and Shutdown before the engine is dropped. |
| Pages | `Render`, `Page` / `PageLocked` (chrome, theme, asset links for extra pages), `Static` for `GET /static/`, `Assets(id, version)` for `Game.Assets`. |
| Tests and previews | `SetClock`, `SetRand`, and `Install(e)` for a running match with no helper and no tick. Call them without the lock. `Tick()` runs one tick now. |

`Gate` is per game. Leave it nil to let actions run while paused. `Reply` gets the phone error and decides where it goes: a view field, or a per-player map on the engine.

### Countdown

Every game timer draws through one client script, `internal/ui/static/countdown.js`, which the shells and `ui-start` load.

1. Keep the countdown in the engine as a `runtimekit.Timer`. `Hold` freezes it on pause.
2. Put `Timer.View(now)` (a `runtimekit.TimerView`) on the board or phone view.
3. In the template, call `ui-countdown` inside the element's opening tag and mark the text with `data-countdown-value`:

```html
{{if .Timer.On}}
<div class="wordbox-timer" {{template "ui-countdown" .Timer}}>
  <strong data-countdown-value>{{.Timer.Clock}}</strong>
</div>
{{end}}
```

The script paints `m:ss` every 250ms from `data-countdown-end`, or shows the rendered seconds while `data-countdown-frozen` is set or in a static preview. It sets `--countdown-left` (1 to 0) and `--countdown-turn` on the element for bars and conic rings. Seconds round up, so the last second shows `0:01`.

A running countdown fires `grabbag:countdown-tick` when the shown second changes and `grabbag:countdown-done` once at zero. Both bubble with `detail.seconds` and `detail.kind` (`Timer.Kind`). Repaints of the same countdown fire nothing new, and a page that loads after the end fires nothing. Use these for sound or animation cues. Do not write a second timer loop in `game.js`.

## Examples

### Register so host can Load

```go
package wordbox

import "github.com/KroniK907/grabbag/internal/games"

const id = "wordbox"

func init() {
	games.Register(games.Factory{ID: id, New: func() games.Game { return New() }})
}
```

In `internal/host/handler.go`:

```go
_ "github.com/KroniK907/grabbag/internal/games/wordbox"
```

Without that import, Catalog is empty for this id and Load returns conflict.

### Store Helper on Load and Start

```go
func (g *Game) Load(h games.Helper) error {
	g.helper = h
	return nil
}

func (g *Game) Start(h games.Helper) error {
	g.helper = h
	g.paused = false
	return nil
}
```

### Settings fragment after Load

Host inlines `GET /settings/game/` into `/settings`. Redirect back to `/settings` after a POST.

```go
func (g *Game) Settings() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", g.getSettings)
	mux.HandleFunc("POST /hide-latency", g.postHideLatency)
	return mux
}

func (g *Game) postHideLatency(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Could not read the form.", http.StatusBadRequest)
		return
	}
	value := []byte("0")
	if r.FormValue("hide") == "1" {
		value = []byte("1")
	}
	if err := g.helper.KVSet("hide-latency", value); err != nil {
		http.Error(w, "Could not save Hide latency.", http.StatusInternalServerError)
		return
	}
	g.helper.Publish("roster")
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}
```

The live package that does this is Testing (`internal/games/testing`).

### Phone and board fragments

`Phone` writes the inner column. Do not send `<html>`. Host wraps Leave and the Host drawer.

`Board` writes a fragment too. Testing's listens for `sse:pause` plus its own `sse:tap` / `sse:testing` events. The shell loads `game.css` and `game.js` from `Assets`.

### End the round from the game

```go
if h.HasAdmin(r) || player.ClaimedHost {
	h.Finish()
	w.WriteHeader(http.StatusNoContent)
}
```

The End game form is `<form hx-post="/play/end" hx-swap="none">`. Finish publishes `round`, and every shell swaps back to the Lobby.

### Lobby rail button before Start

```go
func (g *Game) BoardButtons() []games.BoardButton {
	return []games.BoardButton{{
		Label: "About Testing",
		Path:  "/play/info",
	}}
}
```

GET `/play/info` is allowed after Load. POST `/play/tap` is not, until Start.

### Publish a live tick

```go
h.Publish("tap")
```

The Testing list listens with `hx-trigger="sse:testing, sse:tap, sse:roster, htmx:sseOpen from:body"` and `hx-get="/play/partials/board-list"`. One EventSource already exists on the page via host chrome. Do not open a second one. See [Live SSE](#live-sse).

## Must not

- Import `internal/host`, `internal/lobby`, or `internal/store` from a game package.
- Write seats, name, wait, audience, or claimed-host.
- Parse the player cookie or the admin cookie.
- Register `/`, `/board`, or `/settings` as mux roots.
- Send a full document, `<link>`, or `<script>` from `Board` or `Phone`.
- Use `<form method="post">` or `location.reload` inside a shell.
- Leave a `document` or `window` listener, timer, or frame running after unmount. Use `ctx`.
- Clear KV on Stop or Shutdown (operator clear does that).
- Scan a games folder at runtime.
- Treat `/docs` as this contract. That URL is how to run the binary. The human API reference is `/docs/game-contract.html`.
