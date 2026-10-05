# Borrowed Truths design language

The rules are in [#137](https://github.com/KroniK907/grabbag/issues/137). The look is locked: **Sticker bomb, in navy and yellow**. The reference is `prototype-surfaces.html`. Double-click it. Use `?mode=light` or the M key for light mode.

**Brief:** a game-studio party game for friends on a couch. Vibrant and playful, with a bit of an edge. Dark mode first, with a light mode that works too. It should not look like a stock app or a generic AI layout. The TV is read from about three metres away. Phones are held in one hand under social pressure.

## The look

Everything is a sticker slapped on a wall. The wall is navy with a halftone dot pattern that fades in from the right. Stickers are die-cut: a thick white edge, fat rounded corners, a small tilt, and a hard shadow. The card is the biggest sticker. It has a peeling corner and two shadows printed slightly off register, one yellow and one blue, like a two-ink riso print.

- **Card.** A white sticker, tilted about -2 degrees, with the off-register yellow and blue shadows.
- **Teller.** A "HELLO my name is" name tag. Yellow header, white strip, name in marker.
- **Voters.** Round avatar stickers at alternating tilts. Waiting is greyed out. Locked gets a yellow tape strip that says LOCKED.
- **Labels.** Tape strips (tell count) and small mono text with a wavy underline.
- **Private type badge.** The same name tag, reworded: "HELLO, this truth belongs to Ana". A voter phone never shows a name tag, so the teller and insider phones read as private at a glance.

## Rules

1. **The card is the hero.** During the public read, the card text is the biggest thing on the board. Aim for about 4% of board width, first person, with at most 3 lines at 120 characters.
2. **Two inks, two meanings.** Yellow (sun) is True. Blue (sky) is Lie. Both Lie buttons use blue: "pure lie" is solid, and "someone else's truth" is a dashed cut-line outline. Yellow also carries the brand (logo, name tag, tape). Blue is never used for decoration apart from the card's off-register shadow.
3. **Votes show who, never what.** Voter stickers have two states: waiting (grey) and locked (yellow tape). The board never shows a vote's colour before the reveal.
4. **Private means it looks private.** Only teller and insider phones get the name-tag badge.
5. **Every phone looks alike from the couch.** Insider phones get the same vote buttons as everyone else (#137). Keep each phone state's layout the same size, so a neighbour cannot spot the odd phone from its shape.
6. **Stickers, not cards.** Rounded corners (16 to 18px on phones, about 2% of width on the board), a white die-cut edge, a small tilt, and a hard offset shadow. No soft blurred shadows. No square corners.
7. **Dark first, light second.** Every colour is a token with a dark and a light value. Light mode swaps the wall and adds a thin navy outline round each sticker. The inks stay the same.
8. **Motion is there for the reveal.** Stickers slap on, and tape strips land. Use motion for the True and Lie split, the pause, the answer word, and the owner standing up. Nothing loops while people talk. Everything respects `prefers-reduced-motion`.

## Type

| Role | Font | Use |
| --- | --- | --- |
| Display | Dela Gothic One | Card text, buttons, logo, HELLO, big counts |
| UI | DM Mono | Labels, subtitles, hints, tape text |
| Hand | Permanent Marker | Names written on name tags only |

## Tokens

The game builds on the host `--ui-*` tokens (`internal/ui/static/live.css`) for host chrome. The game's own surfaces use `--bt-*` tokens in `static/game.css`, keyed off the host theme (`neon-dark` or `neon-light`).

| Token | Dark | Light | Use |
| --- | --- | --- | --- |
| `--bt-wall` | `#0b1430` | `#eef2fb` | Board and phone background |
| `--bt-fg` | `#fff` | `#0b1430` | Text on the wall |
| `--bt-muted` | `#aab6d6` | `#4a5578` | Hints and small labels |
| `--bt-dot` | `rgb(255 210 63 / .22)` | `rgb(61 123 255 / .3)` | Halftone dots |
| `--bt-truth` (sun) | `#ffd23f` | `#ffd23f` | True, brand, name tag, tape |
| `--bt-on-truth` | `#0b1430` | `#0b1430` | Text on yellow |
| `--bt-lie` (sky) | `#3d7bff` | `#3d7bff` | Lie buttons, the card's second shadow |
| `--bt-on-lie` | `#fff` | `#fff` | Text on blue |
| `--bt-sticker` | `#f7f9ff` | `#f7f9ff` | Card and name-tag body |
| `--bt-edge` | `transparent` | `#0b1430` | Outline round each sticker |
| `--bt-count` | `#ffd23f` | `#2a5fd6` | Big counts on the wall, such as `4/6 LOCKED` |

Avatars stay `ui.AvatarSVG`, set in a round sticker.

## Board elements

| Element | Phases | Notes |
| --- | --- | --- |
| Card | public read, questioning, vote, reveal | Absent during the private read (#137 leak rule). |
| Teller name tag | every tell phase | "Reading their card" under it during the private read. |
| Tell tape | all | `Tell 5 of 14`. |
| Prompt and count | vote, owner vote | "Ask Marisol questions and see if you can determine if this is true or a lie." and `4/6 LOCKED`. |
| Voter stickers | vote, owner vote | Waiting or locked. |
| Standings | between tells | The full leaderboard as a wall of stickers. |
| Timer | only with timers on | A tape strip. Hidden by default. |
| Reveal word | reveal | TRUE (yellow) or LIE (blue) slapped on over the card. For a borrowed card, a second sticker: "But it's true for someone in this room." |
| Split bar | reveal | True vs Lie counts. The crowd bar sits separately under it. |
| Photo frame | This Is My | The photo as a big tilted sticker with the three claimants' name tags under it, the speaker's tag highlighted. |

## Phone elements

| Element | Who | Notes |
| --- | --- | --- |
| Card echo | voters, teller | A smaller copy of the board card. |
| Vote stack | voters, insiders | TRUE (yellow), LIE pure lie (blue), LIE someone else's truth (dashed blue). Tap targets of 56px or more. |
| I knew it | voters, public read | Secondary button, then a private pick sheet. |
| Name-tag badge | teller, insider | "HELLO, this truth belongs to Ana", "HELLO, this lie is yours", "HELLO, that's yours. Sell it." |
| Skip / Lock in | teller, private read | Skip is dashed and narrower. Lock in is yellow. |
| Reveal button | teller, end of questioning | Shows the line to say out loud. |
| Name list | voters, owner vote | The seated players as a grid of round stickers. |
| Host strip | claimed host | Continue, Pause, Void card, phase elapsed time. Docked at the bottom as a tape strip, apart from the vote. |

## Dropped looks

Neon cabinet, Studio (after *Would I Lie to You?*), Face-off, and Bluff table were tried and dropped on 2026-09-27. Case file, Polygraph, and Tabloid were dropped before them.
