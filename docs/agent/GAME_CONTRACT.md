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
- [Live SSE](#live-sse)
- [Player record](#player-record)
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
  - Board BoardButtons Phone Play
  - Pause Resume Stop Shutdown
game_calls_on_helper:
  - Seated Waiting Audience Player PlayerFromRequest
  - DataDir KVGet KVSet
  - Finish Pause Resume Publish Log
  - Theme HasAdmin
mounts:
  phone_document: /
  board_document: /board
  operator: /settings
  game_settings: /settings/game/   # after Load
  game_play: /play/                # GET after Load; POST only after Start
urls_game_must_not_own: ["/", "/board", "/settings"]
nil_ok: [Settings, Play]
sse:
  endpoint: GET /lobby/events
  connect: body hx-ext=sse sse-connect=/lobby/events on ui-start
  helper: Publish(name)  # data payload is always "update"
  reserved: [roster, round, pause, theme, log, notice]
  pattern: named event then hx-get current HTML, not a delta
  drop: slow pages miss events (non-blocking send, buffer 16)
```

## Who owns what

Lobby owns roster, join, wait list, audience, seats, identity, operator knobs, and the start gate. Games read those facts through Helper. They never write them.

A game owns run logic, per-player run state keyed by Lobby player id, optional extra pages, and the `/board` document plus the inner `/` phone body after Start.

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
| `Board(w, r)` | GET `/board` after Start | Full HTML document. Stamp theme from `h.Theme()`. |
| `BoardButtons() []BoardButton` | Lobby `/board` after Load, before Start | Up to three `{Label, Path, HostOnly}`. Paths are usually under `/play`. |
| `Phone(w, r)` | GET `/` after Start, any signed-in player | Inner body only. Host wraps Leave and the claimed-host drawer. Unknown cookies stay on Lobby join. |
| `Play() http.Handler` | `/play/` | Game POSTs, partials, static. Nil is fine. StripPrefix leaves paths like `/tap`. |
| `Pause() error` | operator or auto-pause on seated disconnect | Stop accepting play if that is the game's rule. May no-op. |
| `Resume() error` | operator | Restart ticks. May no-op. |
| `Stop() error` | round end | Drop in-memory run state. Keep KV. Keep helper. |
| `Shutdown() error` | unload | Drop helper. Stop goroutines. |

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
| `Theme()` | `string` | `neon-light` or `neon-dark`. Stamp full documents so first paint matches `/settings`. |
| `HasAdmin(r)` | `bool` | Valid admin session. Use for board End game. Do not read the admin cookie yourself. |

## HTTP mounts

Public paths stay host-owned. Lobby vs game is a state swap on `/` and `/board`.

| Path | Owner after Load | Owner after Start |
|------|------------------|-------------------|
| `/` | Lobby phone | Game `Phone` body inside host chrome, any signed-in player |
| `/board` | Lobby TV, plus `BoardButtons` | Game `Board` full document |
| `/settings` | Host operator | Host operator, Game Settings fragment inlined |
| `/settings/game/*` | `Settings()` mux | same |
| `/play/*` | GET from `Play()` | GET and POST from `Play()` |

Forms in the settings fragment must post under `/settings/game/...`. Play forms post under `/play/...`.

After Stop, a GET to `/play` may still hit `Play()`. POST `/play` is 404 until the next Start.

### Extra pages

Full extra documents after Load are `Play()` GET routes (`GET /play/picker`, `GET /play/info`). Mutations that must run before Start POST through `Settings()` at `/settings/game/`. `/settings/game/` stays the inlined settings column. It is not those documents.

Do not add a Game method or host mount for extra pages. Do not open `Play()` POST before Start. v1 leaves the CoreHost mux unchanged.

Operator-only extra GET pages check `HasAdmin` in the game. Signed-out GET is plain `401` text, the same idea as host Settings with no admin session. Host does not make every `Play()` GET admin-only. Public extra pages (Testing `/play/info`) stay unsigned. Hiding a `HostOnly` `BoardButton` does not protect the URL.

Apples for Humanity is the first extra operator page: `GET /play/picker`, Lobby `HostOnly` button `Deck Library`, pack enable POSTs to `/settings/game/`. Success is `303` to `/play/picker`. A failed write or a refused-after-Start toggle returns `200` HTML so the checkboxes match stored state. `GET /play/howto` is a public extra document (Lobby `How to play` button, no `HasAdmin`). The in-round phone `?` loads `GET /play/howto-sheet`.

## Live SSE

One EventSource per page. Host chrome (`ui-start`) sets `hx-ext="sse"` and `sse-connect="/lobby/events"` on `body`. `GET /lobby/events` is the in-process hub. `ui-start-quiet` (setup and `/docs`) does not connect.

`Helper.Publish(name)` writes an SSE event with that name and data `update`. Games cannot set the data line. Host uses `PublishData` for `theme` (palette id), `log` (the log line), and `notice` (toast JSON). Put scores and names in a GET partial, not in the event body. Toasts are fire-and-forget. A missed `notice` is not replayed.

The page does not apply a delta from the event. It hears the name, then `hx-get`s current HTML. Include `htmx:sseOpen from:body` on those triggers so a reconnect refetches.

A subscriber channel buffers 16 events. A further publish to a slow page is dropped. Design for missed ticks. The next event, or `sseOpen`, should paint the truth.

Pings are SSE comments every 15s. They keep the stream alive. They are not named events.

### Reserved names

Do not publish these for game ticks. Host already owns them.

| Name | Who publishes | Typical refresh |
|------|---------------|-----------------|
| `roster` | host, Lobby, or a game knob that other pages should reread | roster partials |
| `round` | host on Start, Stop, Shutdown | reload `/` or `/board` via `round-swap` |
| `pause` | host on Pause and Resume | pause chrome and game partials |
| `theme` | host | `html[data-theme]` |
| `log` | host log ring | settings log tail |
| `notice` | host via `Helper.Notify` | chrome toast; client filters by target |

Publishing `roster` is fine when a settings knob changes a board that also shows Lobby facts. Publishing `round` from a game reloads the whole document. Do not do that for a score tick.

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

Register `GET /partials/board-list` on `Play()`. Do not add a second `sse-connect`. Do not open `EventSource` in your own script. Full board documents that use `ui-start` should also include `{{template "round-swap" "/board"}}` so Stop returns the TV to Lobby.

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

### Phone body vs board document

`Phone` writes the inner column. Do not send `<html>`. Host wraps Leave and the Host drawer.

`Board` writes a full document. Testing stamps chrome and listens for `sse:pause` plus its own `sse:tap` / `sse:testing` events.

### End the round from the game

```go
if h.HasAdmin(r) || player.ClaimedHost {
	h.Finish()
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
```

Board End in Testing also posts `return=/board` so the TV lands on `/board`.

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
- Clear KV on Stop or Shutdown (operator clear does that).
- Scan a games folder at runtime.
- Treat `/docs` as this contract. That URL is how to run the binary. The human API reference is `/docs/game-contract.html`.
