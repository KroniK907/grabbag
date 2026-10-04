// Package apples is Apples for Humanity (catalog id apples).
package apples

import (
	"embed"
	"net/http"
	"time"

	"github.com/KroniK907/grabbag/internal/games"
	"github.com/KroniK907/grabbag/internal/games/runtimekit"
	"github.com/KroniK907/grabbag/internal/ui"
)

const (
	id           = "apples"
	assetVersion = "shell-4"
)

//go:embed templates/*.html
var templateFiles embed.FS

//go:embed static/*
var staticFiles embed.FS

var pages = ui.MustParse(templateFiles, "templates/*.html")

// Game is one Apples for Humanity instance. The runtime kit owns the lock,
// the tick, and the POST pipeline. The engine owns the rules.
type Game struct {
	run        *runtimekit.Runner[*engine]
	importErr  string
	fetchURL   string
	httpClient *http.Client

	burns           []burnEntry
	burnCorrupt     bool
	played          map[string]playedRow
	discardCorrupt  bool
	burnWriter      *fileWriter
	discardWriter   *fileWriter
	burnSnap        []burnFace
	burnChecks      map[string]bool
	burnErr         string
	overlay         string
	overlayTooSmall bool
}

// New constructs an unloaded Apples package.
func New() *Game {
	g := &Game{
		fetchURL: officialDumpURL,
		httpClient: &http.Client{
			Timeout: 45 * time.Second,
		},
	}
	g.run = runtimekit.New(runtimekit.Config[*engine]{
		Game:         "Apples for Humanity",
		Event:        eventMatch,
		Pages:        pages,
		AssetVersion: assetVersion,
		Tick:         200 * time.Millisecond,
		Advance: func(e *engine, now time.Time) runtimekit.Result {
			g.seatLocked()
			out := e.Advance(now)
			if !out.Publish {
				return runtimekit.Result{}
			}
			return g.resultLocked(e, out)
		},
		// The engine clock holds while host is paused or the reshuffle
		// overlay is up.
		Hold: func(e *engine, paused bool, now time.Time) {
			e.SetPaused(paused || g.overlayActive(), now)
		},
		// Play goes on while paused. Only the overlay refuses commands.
		Gate: func(*engine, games.Player, bool) string {
			if g.overlayActive() {
				return overlayWaitCopy
			}
			return ""
		},
		Reply: func(e *engine, p games.Player, _ *http.Request, msg string) (string, any) {
			if msg != "" && e != nil {
				e.PhoneErr[p.ID] = msg
			}
			return "phone.html", g.phoneViewLocked(p)
		},
		Cleanup: g.cleanupLocked,
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

// ID is the catalog id apples.
func (g *Game) ID() string { return id }

// Name is the player-facing label Apples for Humanity.
func (g *Game) Name() string { return "Apples for Humanity" }

func (g *Game) Description() string {
	return "Pick prompts and answers from your deck libraries, then judge the funniest match."
}

// MinPlayers is 1. Host still needs one seated player to Start.
func (g *Game) MinPlayers() int { return 1 }

// maxPlayers is the largest table whose board still fits a 1080p TV.
// The roster scrolls sideways at 15, so the game stops at 14.
const maxPlayers = 14

// MaxPlayers is 14. Above that the TV roster runs off the screen.
func (g *Game) MaxPlayers() int { return maxPlayers }

// Load stores the helper, copies missing shipped libraries, and seeds match-settings.
func (g *Game) Load(h games.Helper) error {
	g.run.Load(h)
	g.run.Lock()
	g.importErr = ""
	g.openJournalsLocked(h)
	g.run.Unlock()
	if err := copyShippedLibraries(h.DataDir()); err != nil {
		return err
	}
	return g.persistSettings()
}

// Settings is pack toggles and official import after Load.
func (g *Game) Settings() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", g.getSettings)
	mux.HandleFunc("POST /pack", g.postPack)
	mux.HandleFunc("POST /tag", g.postTag)
	mux.HandleFunc("POST /select-all", g.postSelectAll)
	mux.HandleFunc("POST /select-none", g.postSelectNone)
	mux.HandleFunc("POST /import-official", g.postImportOfficial)
	mux.HandleFunc("POST /hand-size", g.postHandSize)
	mux.HandleFunc("POST /win-score", g.postWinScore)
	mux.HandleFunc("POST /win-by-rounds", g.postWinByRounds)
	mux.HandleFunc("POST /round-limit", g.postRoundLimit)
	mux.HandleFunc("POST /winner-points", g.postWinnerPoints)
	mux.HandleFunc("POST /fav-1", g.postFav1)
	mux.HandleFunc("POST /fav-2", g.postFav2)
	mux.HandleFunc("POST /fav-3", g.postFav3)
	mux.HandleFunc("POST /multiplier", g.postMultiplier)
	mux.HandleFunc("POST /prompt-mode", g.postPromptMode)
	mux.HandleFunc("POST /bot-count", g.postBotCount)
	mux.HandleFunc("POST /voting", g.postVoting)
	mux.HandleFunc("POST /live-counts", g.postLiveCounts)
	mux.HandleFunc("POST /host-judge", g.postHostJudge)
	mux.HandleFunc("POST /host-reveals", g.postHostReveals)
	mux.HandleFunc("POST /timer-auto-draw", g.postAutoDraw)
	mux.HandleFunc("POST /timer-submit", g.postSubmitSec)
	mux.HandleFunc("POST /timer-between", g.postBetweenSec)
	mux.HandleFunc("POST /timer-judge-pick", g.postJudgePickSec)
	mux.HandleFunc("POST /timer-favorite-vote", g.postFavoriteVoteSec)
	mux.HandleFunc("POST /timer-finish-hold", g.postFinishHoldSec)
	mux.HandleFunc("POST /wildcard-cap", g.postWildcardCap)
	mux.HandleFunc("POST /wildcard-save", g.postWildcardSave)
	mux.HandleFunc("POST /wildcard-favorites", g.postWildcardFavorites)
	mux.HandleFunc("POST /wildcard-deal-previous", g.postDealPrevious)
	mux.HandleFunc("POST /wildcard-duplicate", g.postDuplicateBlock)
	mux.HandleFunc("POST /wildcard-banned", g.postBanned)
	mux.HandleFunc("POST /wildcard-show-word", g.postShowWord)
	mux.HandleFunc("POST /wildcard-journal-delay", g.postJournalDelay)
	mux.HandleFunc("POST /unburn", g.postUnburn)
	mux.HandleFunc("POST /reshuffle-discard", g.postReshuffleDiscard)
	mux.HandleFunc("POST /reshuffle-on-unload", g.postReshuffleOnUnload)
	return mux
}

// Start deals hands, fills bots, and freezes picker toggles.
func (g *Game) Start(h games.Helper) error {
	return g.run.Start(h, g.beginMatchLocked)
}

// Board is the full-bleed TV document after Start.
func (g *Game) Board(w http.ResponseWriter, r *http.Request) {
	g.run.Render(w, "board.html", g.boardView(r), http.StatusOK)
}

// BoardButtons publishes How to play and Deck Library on the Lobby rail after Load.
func (g *Game) BoardButtons() []games.BoardButton {
	return []games.BoardButton{
		{Label: "How to play", Path: "/play/howto"},
		{Label: "Deck Library", Path: "/play/picker", HostOnly: true},
	}
}

// Assets is game.css, game.js, and layout.js for the shells. game.js
// registers the board and phone lifecycles; layout.js fits the answer cards'
// text, also in previews.
func (g *Game) Assets() ui.Assets {
	a := runtimekit.Assets(id, assetVersion)
	a.Layout = []string{games.StaticPath(id) + "layout.js?v=" + assetVersion}
	return a
}

// Phone is the seated, judge, audience, or wait-list column. Host currently
// only mounts this for seated players (CoreHost #59).
func (g *Game) Phone(w http.ResponseWriter, r *http.Request) {
	g.run.Render(w, "phone.html", g.phoneView(r), http.StatusOK)
}

// Play mounts the Deck Library GET page and game CSS.
func (g *Game) Play() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /picker", g.getPicker)
	mux.HandleFunc("GET /howto", g.getHowto)
	mux.HandleFunc("GET /howto-sheet", g.getHowtoSheet)
	mux.HandleFunc("GET /partials/board", g.getBoardPartial)
	mux.HandleFunc("GET /partials/phone", g.getPhonePartial)
	mux.HandleFunc("POST /burn", g.postBurn)
	mux.HandleFunc("POST /reshuffle-yes", g.postReshuffleYes)
	mux.HandleFunc("POST /end-game", g.postEndGame)
	mux.HandleFunc("POST /draw", g.post(cmdDraw))
	mux.HandleFunc("POST /skip", g.post(cmdSkip))
	mux.HandleFunc("POST /keep-prompt", g.post(cmdKeepPrompt))
	mux.HandleFunc("POST /choose-prompt", g.post(cmdChoosePrompt))
	mux.HandleFunc("POST /slot", g.post(cmdSlot))
	mux.HandleFunc("POST /unslot", g.postUnslot)
	mux.HandleFunc("POST /discard", g.post(cmdDiscard))
	mux.HandleFunc("POST /lock", g.post(cmdLock))
	mux.HandleFunc("POST /wildcard-draft", g.post(cmdDraft))
	mux.HandleFunc("POST /reveal", g.post(cmdReveal))
	mux.HandleFunc("POST /vote", g.postVote)
	mux.HandleFunc("POST /confirm", g.post(cmdConfirm))
	mux.HandleFunc("POST /finish", g.postFinish)
	mux.Handle("GET /static/", runtimekit.Static(staticFiles))
	return mux
}

// Pause leaves picker toggles frozen. Pause counts as started.
func (g *Game) Pause() error { return g.run.Pause() }

// Resume clears the paused flag. Toggles stay frozen until Stop.
func (g *Game) Resume() error { return g.run.Resume() }

// Stop ends the match freeze. KV and library files stay.
func (g *Game) Stop() error { return g.run.Stop() }

// Shutdown unloads the helper.
func (g *Game) Shutdown() error { return g.run.Shutdown() }

// cleanupLocked drains the journals and drops the overlay on Stop. Shutdown
// also wipes the discard when the operator asked for that and closes the
// journal writers.
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
	g.burnSnap = nil
	if unload {
		g.stopWritersLocked()
		g.importErr = ""
	}
}

func (g *Game) matchFrozen() bool {
	g.run.Lock()
	defer g.run.Unlock()
	return g.run.Running()
}

func (g *Game) chromeView(title string) runtimekit.Page {
	return g.run.PageLocked(title)
}
