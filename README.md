# Hackbox

A room, a TV, and a pile of phones. Hackbox is a Go host that puts the game on the big screen and the buttons in everyone's pocket. One process. Same Wi-Fi. No app store.

The cabinet is neon on purpose. The first compiled-in package is Testing. It is a diagnostics table, not a trivia night. Party games land in `internal/games/` the same way.

![The TV board with four seated players, a join QR, and Testing loaded](docs/shots/board.png)

The board is `/board`. Fullscreen that on the TV. The QR is the join URL for phones.

![Phone join. Who are you, reroll face, name, Join](docs/shots/phone-join.png)

Join is a face, a name, and one button. The first person who also types the admin password becomes the claimed host.

## Run a room

- [How to run Hackbox](docs/index.html) is the operator guide. After you start the binary it is also `http://127.0.0.1:8654/docs`.
- From this repo: `go run ./cmd/hackbox`, then open `http://127.0.0.1:8654` on the host machine. Phones use the LAN URL on the board, not 127.0.0.1.

## Write a game

- [Game API reference](docs/game-contract.html) is the human page. Method signatures, parameters, examples, and live SSE. On a running host it is `http://127.0.0.1:8654/docs/game-contract.html`.
- [Game / host contract](docs/agent/GAME_CONTRACT.md) is the short agent page. Same contract, tighter tables.
- [Codebase layout](docs/agent/CODEBASE.md) is where packages, embeds, and imports live.
