// Package quips is Quick Quips (catalog id quips).
package quips

import (
	"embed"
	"net/http"
	"time"

	"github.com/KroniK907/grabbag/internal/games"
	"github.com/KroniK907/grabbag/internal/games/runtimekit"
	"github.com/KroniK907/grabbag/internal/ui"
)

const (
	id           = "quips"
	assetVersion = "playtest-3"
)

//go:embed templates/*.html
var templateFiles embed.FS

//go:embed static/*
var staticFiles embed.FS

var pages = ui.MustParse(templateFiles, "templates/*.html")

// Game is one Quick Quips instance. The runtime kit owns the lock, the
// tick, and the POST pipeline. The engine owns the rules.
type Game struct {
	run *runtimekit.Runner[*engine]

	burns           []burnEntry
	burnCorrupt     bool
	played          map[string]playedRow
	discardCorrupt  bool
	burnWriter      *fileWriter
	discardWriter   *fileWriter
	burnDrawer      []burnFace
	burnChecks      map[string]bool
	burnErr         string
	overlay         string
	overlayTooSmall bool
}

// New constructs an unloaded Quick Quips package.
func New() *Game {
	g := &Game{played: map[string]playedRow{}}
	g.run = runtimekit.New(runtimekit.Config[*engine]{
		Game:         "Quick Quips",
		Event:        eventQuips,
		Pages:        pages,
		AssetVersion: assetVersion,
		Tick:         200 * time.Millisecond,
		Advance: func(e *engine, now time.Time) runtimekit.Result {
			return g.resultLocked(e.Advance(now))
		},
		Hold: func(e *engine, paused bool, now time.Time) {
			e.SetPaused(paused || g.overlayActive(), now)
		},
		Gate: func(_ *engine, _ games.Player, paused bool) string {
			if g.overlayActive() {
				return overlayWaitCopy
			}
			if paused {
				return "Match is paused."
			}
			return ""
		},
		Reply: func(e *engine, p games.Player, r *http.Request, msg string) (string, any) {
			if msg != "" && e != nil {
				e.PhoneErr[p.ID] = msg
			}
			return "phone.html", g.phoneViewLockedWithRequest(p, r)
		},
		Cleanup: g.cleanupLocked,
	})
	return g
}

// fixedGame is a running Game on e with a frozen clock and no helper.
// Previews and tests drive views from it.
func fixedGame(e *engine, now func() time.Time) *Game {
	g := New()
	g.run.SetClock(now)
	g.run.Install(e)
	return g
}

// ID is the catalog id quips.
func (g *Game) ID() string { return id }

// Name is the player-facing label Quick Quips.
func (g *Game) Name() string { return "Quick Quips" }

func (g *Game) Description() string {
	return "Write quick answers to rotating prompts and vote on the best lines."
}

// MinPlayers is 2. Quick Quips needs at least two seated writers.
func (g *Game) MinPlayers() int { return 2 }

// MaxPlayers is 0. The host seat cap is the ceiling.
func (g *Game) MaxPlayers() int { return 0 }

// Load stores the helper, copies missing shipped libraries, and opens journal paths.
func (g *Game) Load(h games.Helper) error {
	g.run.Load(h)
	g.run.Lock()
	g.openJournalsLocked(h)
	g.run.Unlock()
	if err := copyShippedLibraries(h.DataDir()); err != nil {
		return err
	}
	return g.persistSettings()
}

// Settings is match knobs and library link after Load.
func (g *Game) Settings() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", g.getSettings)
	mux.HandleFunc("POST /round-count", g.postRoundCount)
	mux.HandleFunc("POST /last-quip", g.postLastQuip)
	mux.HandleFunc("POST /round-multiplier-increase", g.postRoundMultiplierIncrease)
	mux.HandleFunc("POST /seated-vote-points", g.postSeatedVotePoints)
	mux.HandleFunc("POST /audience-vote-points", g.postAudienceVotePoints)
	mux.HandleFunc("POST /timer-write", g.postTimerWrite)
	mux.HandleFunc("POST /timer-vote", g.postTimerVote)
	mux.HandleFunc("POST /timer-winner-screen", g.postTimerWinnerScreen)
	mux.HandleFunc("POST /timer-final-scores", g.postTimerFinalScores)
	mux.HandleFunc("POST /host-controlled-reveals", g.postHostControlledReveals)
	mux.HandleFunc("POST /allow-self-vote", g.postAllowSelfVote)
	mux.HandleFunc("POST /live-counts", g.postLiveCounts)
	mux.HandleFunc("POST /quip-char-cap", g.postQuipCharCap)
	mux.HandleFunc("POST /banned-words", g.postBannedWords)
	mux.HandleFunc("POST /show-matched-word", g.postShowMatchedWord)
	mux.HandleFunc("POST /pack", g.postPack)
	mux.HandleFunc("POST /select-all", g.postSelectAll)
	mux.HandleFunc("POST /select-none", g.postSelectNone)
	mux.HandleFunc("POST /unburn", g.postUnburn)
	mux.HandleFunc("POST /reshuffle-discard", g.postReshuffleDiscard)
	mux.HandleFunc("POST /reshuffle-on-unload", g.postReshuffleOnUnload)
	return mux
}

// Start deals prompts and opens the write phase.
func (g *Game) Start(h games.Helper) error {
	return g.run.Start(h, g.beginMatchLocked)
}

// Board is the TV document after Start.
func (g *Game) Board(w http.ResponseWriter, r *http.Request) {
	g.run.Render(w, "board.html", g.boardView(), http.StatusOK)
}

// BoardButtons publishes How to play and Prompt Library on the Lobby rail.
func (g *Game) BoardButtons() []games.BoardButton {
	return []games.BoardButton{
		{Label: "How to play", Path: "/play/howto"},
		{Label: "Prompt Library", Path: "/play/picker", HostOnly: true},
	}
}

// Phone is the seated player column after Start.
func (g *Game) Phone(w http.ResponseWriter, r *http.Request) {
	g.run.Render(w, "phone.html", g.phoneView(r), http.StatusOK)
}

// Play mounts prompt library and static assets after Load.
func (g *Game) Play() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /picker", g.getPicker)
	mux.HandleFunc("GET /howto", g.getHowto)
	mux.HandleFunc("GET /partials/board", g.getBoardPartial)
	mux.HandleFunc("GET /partials/phone", g.getPhonePartial)
	mux.HandleFunc("POST /draft", g.postDraft)
	mux.HandleFunc("POST /lock", g.postLock)
	mux.HandleFunc("POST /vote", g.postVote)
	mux.HandleFunc("POST /host/reveal", g.postHostReveal)
	mux.HandleFunc("POST /host/next-segment", g.postHostNextSegment)
	mux.HandleFunc("POST /host/skip-hold", g.postHostSkipHold)
	mux.HandleFunc("POST /host/end-match", g.postHostEndMatch)
	mux.HandleFunc("POST /burn", g.postBurn)
	mux.HandleFunc("POST /reshuffle-yes", g.postReshuffleYes)
	mux.HandleFunc("POST /end-game", g.postEndGame)
	mux.Handle("GET /static/", runtimekit.Static(staticFiles))
	return mux
}

// Pause freezes the match timer and blocks compose POSTs.
func (g *Game) Pause() error { return g.run.Pause() }

// Resume thaws the match timer unless the reshuffle overlay still holds it.
func (g *Game) Resume() error { return g.run.Resume() }

// Stop ends the match freeze. KV and library files stay.
func (g *Game) Stop() error { return g.run.Stop() }

// Shutdown unloads the helper.
func (g *Game) Shutdown() error { return g.run.Shutdown() }

// cleanupLocked drains the journals and drops the overlay on Stop. Shutdown
// also wipes the discard when the operator asked for that, closes the
// journal writers, and forgets the journal contents.
func (g *Game) cleanupLocked(_ *engine, unload bool) {
	if unload {
		if h := g.run.HelperLocked(); h != nil {
			settings, ok := g.loadSettings(h)
			if ok && settings.ReshuffleDiscardOnUnload && !g.discardCorrupt {
				g.wipeDiscardLocked()
			}
		}
	}
	g.drainJournalsLocked()
	g.overlay = ""
	g.overlayTooSmall = false
	g.burnDrawer = nil
	if !unload {
		return
	}
	g.stopWritersLocked()
	g.burns = nil
	g.played = map[string]playedRow{}
	g.burnCorrupt = false
	g.discardCorrupt = false
}

func (g *Game) getSettings(w http.ResponseWriter, r *http.Request) {
	g.run.Render(w, "settings.html", g.settingsView(settingsErr{}), http.StatusOK)
}

func (g *Game) getHowto(w http.ResponseWriter, r *http.Request) {
	if g.run.Helper() == nil {
		http.NotFound(w, r)
		return
	}
	g.run.Render(w, "howto.html", g.run.Page("How to play Quick Quips"), http.StatusOK)
}
