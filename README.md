# Grab Bag

A room, a TV, and a pile of phones. Grab Bag is a Go host that puts the game on the big screen and the buttons in everyone's pocket. The public name is [grabbag.gg](https://grabbag.gg).

One process. Browser only. No app store, no cloud account, no room code on someone else's website. Default night is the same Wi-Fi. Remote phones can join through a tunnel you run on the PC.

The cabinet is neon on purpose. This build ships two party games, Apples for Humanity and Quick Quips, plus Testing as a wiring check.

![The TV board with four seated players, a join QR, and Testing loaded](docs/shots/board.png)

The board is `/board`. Fullscreen that on the TV. The QR is the join URL for phones.

![Phone join. Who are you, reroll face, name, Join](docs/shots/phone-join.png)

Join is a face, a name, and one button. The first person who also types the admin password becomes the claimed host.

## Contents

- [What you get](#what-you-get)
- [Games](#games)
- [Status](#status)
- [Run a room](#run-a-room)
- [Hosting](#hosting)
- [Write a game](#write-a-game)
- [Docs](#docs)
- [Tests](#tests)
- [License](#license)

## What you get

- A Windows PC that stays on for the party. That machine is the server. Closing the console ends the room.
- A TV board at `/board` with seats, a QR, and the join URL.
- Phones as controllers. Players scan the QR, pick a face, type a name, tap Join.
- A claimed-host phone with Load, Start, Pause, Stop, kick, and settings.
- Room state in SQLite. Quit and come back, then Keep this room or Clear room.
- Two palettes, `neon-light` and `neon-dark`.
- Operator docs inside the binary at `http://127.0.0.1:8654/docs`.
- Two party games, Apples for Humanity and Quick Quips, plus Testing.

What it is not:

- A phone app. Players use a browser.
- A public matchmaking service. Default night is LAN. Advertised hostname is only for a tunnel you set up yourself.

## Games

**Apples for Humanity.** One seated player is the judge each round. Everyone else fills the prompt with cards from a hand. The judge reveals the plays and picks a winner. After Load, Deck Library is on the board rail. White and Black is fill-in-the-blank. Oranges to Oranges is adjective and noun. Wildcard is a tiny typed-answer set. The picker can also pull the JSON Against Humanity dump if you want those cards. Bots can sit in empty chairs. How to play is on the rail after Load, and on phones during a round.

**Quick Quips.** Two or more seated people write lines into rotating prompts, then vote. Audience can vote if you leave those points on. After Load, Prompt Library is on the board rail. Comedy ships in the binary.

**Testing.** A seated list, a plunger, and latency. Use it when the host looks broken. It is not the party.

## Status

This is still moving. Interfaces can change. There is no "forgot password" in the binary yet. Write the admin password down.

Games are compile-time. A package under `internal/games/<name>` calls `games.Register` from `init`. Host blank-imports it. There is no runtime folder scan and no separate game binary.

## Run a room

You need Go 1.26 or newer, a Windows computer, a TV or second window, and phones. For a living-room night, phones share the **same Wi-Fi** as the PC. Guest Wi-Fi often blocks that. For people off the house network, see [Hosting](#hosting).

```text
git clone https://github.com/KroniK907/grabbag.git
cd grabbag
go run ./cmd/grabbag
```

Or build a binary and double-click it:

```text
go build -o grabbag.exe ./cmd/grabbag
.\grabbag.exe
```

Leave the console window open. Grab Bag always binds port **8654**. If another copy is already running, this one prints an error and exits.

On the host machine, open `http://127.0.0.1:8654`. First visit is setup. Pick an admin password (at least 8 characters), tap Finish, fullscreen `/board` on the TV.

Phones use the URL or QR on the board. They must not use `127.0.0.1`. That address is this device, so a phone would look for Grab Bag inside itself.

The step-by-step operator guide is [How to run Grab Bag](docs/index.html). After the process starts it is also `http://127.0.0.1:8654/docs`. Hosting remote phones is in that same page.

Room files live under `%LOCALAPPDATA%\grabbag` (`host.sqlite`, optional `host.log`, a `games` folder per game id).

## Hosting

Leave advertised hostname blank on LAN. The board then shows the IPv4 Grab Bag found at start.

To let phones on the public internet join without buying a domain or opening a router port, run [Tailscale Funnel](https://tailscale.com/docs/features/tailscale-funnel) on the same PC. Install Tailscale, sign in, start Grab Bag, then:

```text
tailscale funnel 8654
```

Paste the printed `https://something.ts.net` URL into advertised hostname. Include `https://`. Guests do not install Tailscale. The PC has to stay awake. Grab Bag still listens on 8654 locally. The public URL is HTTPS on 443.

Do not use Cloudflare Quick Tunnels (`trycloudflare.com`). Those tunnels do not carry SSE, and Grab Bag's lobby and board live on SSE.

If you already have a domain on Cloudflare, a named Cloudflare Tunnel is a stable hostname you control. That is a different product from Quick Tunnels. Details are in the operator guide.

## Write a game

A game is a Go package that implements `games.Game`. Host calls Load, Start, Pause, Resume, Stop, Shutdown. The game renders the TV through `Board` and the phones through `Phone` / `Play`. Persistence goes through Helper (`KVGet` / `KVSet`, `DataDir`). Live updates go through `Publish` on the lobby SSE hub.

The wiring reference is [Testing](internal/games/testing). Copy that shape. A scored match looks more like [Apples for Humanity](internal/games/apples) or [Quick Quips](internal/games/quips). Register in `init`, then blank-import the package from `internal/host/handler.go` so the catalog sees it.

```text
# after you add the package and the blank import
go test ./internal/games/... ./internal/host/...
go run ./cmd/grabbag
```

Load the game from the Host drawer. Start needs at least one seated player.

## Docs

| Page | Who it is for |
| --- | --- |
| [How to run](docs/index.html) | Person with the PC. Also `GET /docs`. |
| [Game API reference](docs/game-contract.html) | Human writing a package. Signatures, examples, SSE. Also `GET /docs/game-contract.html`. |
| [Game / host contract](docs/agent/GAME_CONTRACT.md) | Agent (and anyone who wants the short tables). |
| [Codebase layout](docs/agent/CODEBASE.md) | Packages, embeds, import rules. |

`docs/*.html` files carry their own CSS. Host serves the embedded bytes. It does not restyle them through `live.css`.

## Tests

```text
go test ./...
```

Tests sit next to the code they cover. Prefer `package foo_test` unless a test must see unexported details.

## License

[GNU GPLv3](LICENSE).
