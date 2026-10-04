// Package borrowedtruths is Borrowed Truths (catalog id borrowedtruths): Would
// I Lie to You? for the couch, where some true stories belong to someone else.
package borrowedtruths

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/KroniK907/grabbag/internal/games"
	"github.com/KroniK907/grabbag/internal/games/runtimekit"
	"github.com/KroniK907/grabbag/internal/ui"
)

const (
	id           = "borrowedtruths"
	assetVersion = "shell-1"
	// event is the one SSE name this game publishes after any run state change.
	event = "borrowedtruths"
	// eventFacts is a facts tick. Only the board listens, so phones mid-write
	// are not swapped.
	eventFacts = "borrowedtruths-facts"
)

//go:embed templates/*.html
var templateFiles embed.FS

//go:embed static/*
var staticFiles embed.FS

//go:embed shipped/lies.txt
var bankFile string

var pages = ui.MustParse(templateFiles, "templates/*.html")

// bank is the built-in lie bank, used when players run out of lies.
var bank = parseBank(bankFile)

func parseBank(raw string) []string {
	var out []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			out = append(out, line)
		}
	}
	return out
}

func init() {
	games.Register(games.Factory{ID: id, New: func() games.Game { return New() }})
}

// Game is one Borrowed Truths instance. The runtime kit owns the lock, the
// tick, and the POST pipeline. The engine owns the rules.
type Game struct {
	run *runtimekit.Runner[*engine]
	// runDir holds this match's This Is My photos. Stop and Shutdown delete it.
	runDir string
	// photoSrc replaces photo URLs in previews, which have no files.
	photoSrc func(id string) template.URL
}

// New constructs an unloaded Borrowed Truths package.
func New() *Game {
	g := &Game{}
	g.run = runtimekit.New(runtimekit.Config[*engine]{
		Game:         "Borrowed Truths",
		Event:        event,
		Pages:        pages,
		AssetVersion: assetVersion,
		Advance: func(e *engine, now time.Time) runtimekit.Result {
			finish, changed := e.Advance(now)
			return runtimekit.Result{Changed: changed, Finish: finish}
		},
		Hold: func(e *engine, paused bool, now time.Time) { e.SetPaused(paused, now) },
		Gate: func(_ *engine, _ games.Player, paused bool) string {
			if paused {
				return "The match is paused."
			}
			return ""
		},
		Reply: func(_ *engine, p games.Player, _ *http.Request, msg string) (string, any) {
			return "phone-frame", g.phoneViewLocked(p, msg)
		},
		Cleanup: func(*engine, bool) { g.dropRunLocked() },
	})
	return g
}

// fixedGame is a running Game on e with a frozen clock and no helper.
// Previews and tests drive views from it.
func fixedGame(e *engine, now time.Time) *Game {
	g := New()
	g.run.SetClock(func() time.Time { return now })
	g.run.Install(e)
	return g
}

// ID is the catalog id borrowedtruths.
func (g *Game) ID() string { return id }

// Name is the player-facing label Borrowed Truths.
func (g *Game) Name() string { return "Borrowed Truths" }

// Description is the game library blurb.
func (g *Game) Description() string {
	return "Read a card about yourself and sell it. Some are true, some are lies, and some are true for someone else on the couch."
}

// MinPlayers is 4. Fewer leaves nobody to borrow from.
func (g *Game) MinPlayers() int { return 4 }

// MaxPlayers is 20.
func (g *Game) MaxPlayers() int { return 20 }

// PauseOnDisconnect is false. Nothing in a tell waits on one phone: the host's
// Continue covers every teller action, and a phone that sleeps reloads the
// same moment when it wakes. So a dropped phone never pauses the room.
func (g *Game) PauseOnDisconnect() bool { return false }

// Load stores the helper and drops photo folders a crash left behind.
func (g *Game) Load(h games.Helper) error {
	g.run.Load(h)
	clearRuns(h.DataDir())
	return nil
}

// Settings is the match knobs form after Load.
func (g *Game) Settings() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", g.getSettings)
	mux.HandleFunc("POST /match", g.postMatch)
	return mux
}

// Start snapshots the seated players and opens the facts phase.
func (g *Game) Start(h games.Helper) error {
	return g.run.Start(h, func(h games.Helper) (*engine, error) {
		seated := h.Seated()
		if len(seated) < g.MinPlayers() {
			return nil, fmt.Errorf("Borrowed Truths needs at least %d seated players.", g.MinPlayers())
		}
		rows := make([]rosterRow, 0, len(seated))
		for _, p := range seated {
			rows = append(rows, rosterRow{ID: p.ID, Name: p.DisplayName, Seed: p.AvatarSeed})
		}
		g.dropRunLocked()
		return newEngine(g.loadSettings(h), rows, bank, g.run.Rand(), g.run.Now()), nil
	})
}

// Board is the TV document after Start.
func (g *Game) Board(w http.ResponseWriter, r *http.Request) {
	g.run.Render(w, "board.html", g.boardView(), http.StatusOK)
}

// BoardButtons publishes How to play on the Lobby rail.
func (g *Game) BoardButtons() []games.BoardButton {
	return []games.BoardButton{{Label: "How to play", Path: "/play/howto"}}
}

// Assets is game.css, game.js, and the web fonts for the shells. game.js
// registers the board and phone lifecycles.
func (g *Game) Assets() ui.Assets {
	a := runtimekit.Assets(id, assetVersion)
	a.External = []string{fontsURL}
	return a
}

// fontsURL is the Google Fonts sheet for the sticker-bomb look.
const fontsURL = "https://fonts.googleapis.com/css2?family=DM+Mono:wght@400;500&family=Dela+Gothic+One&family=Permanent+Marker&display=swap"

// Phone is the player column after Start.
func (g *Game) Phone(w http.ResponseWriter, r *http.Request) {
	g.run.Render(w, "phone.html", g.phoneView(r, ""), http.StatusOK)
}

// Play mounts the how-to page, partials, phone posts, and static assets.
func (g *Game) Play() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /howto", g.getHowto)
	mux.HandleFunc("GET /partials/board", g.getBoardPartial)
	mux.HandleFunc("GET /partials/phone", g.getPhonePartial)
	mux.HandleFunc("GET /photo/{id}", g.getPhoto)
	mux.HandleFunc("POST /photo", g.postPhoto)
	mux.HandleFunc("POST /facts", g.postFacts)
	mux.HandleFunc("POST /skip", g.postSkip)
	mux.HandleFunc("POST /lock-in", g.postLockIn)
	mux.HandleFunc("POST /knew", g.postKnew)
	mux.HandleFunc("POST /vote", g.postVote)
	mux.HandleFunc("POST /owner-vote", g.postOwnerVote)
	mux.HandleFunc("POST /reveal", g.postReveal)
	mux.HandleFunc("POST /host/continue", g.postHostContinue)
	mux.HandleFunc("POST /host/void", g.postHostVoid)
	mux.HandleFunc("POST /host/extend", g.postHostExtend)
	mux.HandleFunc("POST /host/pause", g.run.HostPause)
	mux.HandleFunc("POST /host/resume", g.run.HostResume)
	mux.Handle("GET /static/", runtimekit.Static(staticFiles))
	return mux
}

// Pause freezes the phase timer and blocks phone posts.
func (g *Game) Pause() error { return g.run.Pause() }

// Resume thaws the phase timer.
func (g *Game) Resume() error { return g.run.Resume() }

// Stop drops the match. Facts and photos are run state, so they go with it.
// KV stays.
func (g *Game) Stop() error { return g.run.Stop() }

// Shutdown unloads the helper.
func (g *Game) Shutdown() error { return g.run.Shutdown() }

func (g *Game) getHowto(w http.ResponseWriter, r *http.Request) {
	if g.run.Helper() == nil {
		http.NotFound(w, r)
		return
	}
	g.run.Render(w, "howto.html", g.run.Page("How to play Borrowed Truths"), http.StatusOK)
}
