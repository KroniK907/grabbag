// Package apples is Apples for Humanity (catalog id apples).
package apples

import (
	"bytes"
	"embed"
	"io/fs"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/KroniK907/grabbag/internal/games"
	"github.com/KroniK907/grabbag/internal/ui"
)

const (
	id           = "apples"
	assetVersion = "match-15"
)

//go:embed templates/*.html
var templateFiles embed.FS

//go:embed static/*
var staticFiles embed.FS

var pages = ui.MustParse(templateFiles, "templates/*.html")

// Game is one Apples for Humanity instance.
type Game struct {
	mu         sync.Mutex
	helper     games.Helper
	started    bool
	paused     bool
	importErr  string
	fetchURL   string
	httpClient *http.Client
	engine     *engine
	stopTick   chan struct{}
	rng        *rand.Rand
	now        func() time.Time
	needFinish bool
	needPause  bool

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
	return &Game{
		fetchURL: officialDumpURL,
		httpClient: &http.Client{
			Timeout: 45 * time.Second,
		},
	}
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
	g.mu.Lock()
	g.helper = h
	g.started = false
	g.paused = false
	g.importErr = ""
	g.openJournalsLocked(h)
	g.mu.Unlock()
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
	g.mu.Lock()
	defer g.mu.Unlock()
	g.helper = h
	return g.beginMatchLocked(h)
}

// Board is the full-bleed TV document after Start.
func (g *Game) Board(w http.ResponseWriter, r *http.Request) {
	g.render(w, "board.html", g.boardView(r), http.StatusOK)
}

// BoardButtons publishes How to play and Deck Library on the Lobby rail after Load.
func (g *Game) BoardButtons() []games.BoardButton {
	return []games.BoardButton{
		{Label: "How to play", Path: "/play/howto"},
		{Label: "Deck Library", Path: "/play/picker", HostOnly: true},
	}
}

// Phone is the seated, judge, audience, or wait-list column. Host currently
// only mounts this for seated players (CoreHost #59).
func (g *Game) Phone(w http.ResponseWriter, r *http.Request) {
	g.render(w, "phone.html", g.phoneView(r), http.StatusOK)
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
	files, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic("apples: embedded static directory is missing")
	}
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(files))))
	return mux
}

// Pause leaves picker toggles frozen. Pause counts as started.
func (g *Game) Pause() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.paused = true
	g.holdLocked()
	return nil
}

// Resume clears the paused flag. Toggles stay frozen until Stop.
func (g *Game) Resume() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.paused = false
	g.holdLocked()
	if g.started {
		g.startTickerLocked()
	}
	return nil
}

// Stop ends the match freeze. KV and library files stay.
func (g *Game) Stop() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.stopTickerLocked()
	g.drainJournalsLocked()
	g.started = false
	g.paused = false
	g.engine = nil
	g.overlay = ""
	g.overlayTooSmall = false
	g.burnSnap = nil
	return nil
}

// Shutdown unloads the helper.
func (g *Game) Shutdown() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.stopTickerLocked()
	if g.helper != nil {
		settings, ok := g.loadSettings(g.helper)
		if ok && settings.ReshuffleDiscardOnUnload && !g.discardCorrupt {
			g.wipeDiscardLocked()
		}
	}
	g.drainJournalsLocked()
	g.stopWritersLocked()
	g.helper = nil
	g.started = false
	g.paused = false
	g.importErr = ""
	g.engine = nil
	g.overlay = ""
	return nil
}

func (g *Game) helperNow() games.Helper {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.helper
}

func (g *Game) matchFrozen() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.started
}

func (g *Game) chromeView(title string) pageView {
	view := pageView{
		Chrome:  ui.Chrome{Title: title, Theme: ui.DefaultTheme},
		GameCSS: "/play/static/game.css?v=" + assetVersion,
		GameJS:  "/play/static/game.js?v=" + assetVersion,
	}
	if g.helper != nil {
		view.Chrome.Theme = ui.NormalizeTheme(g.helper.Theme())
	}
	return view
}

func (g *Game) render(w http.ResponseWriter, name string, data any, status int) {
	var buf bytes.Buffer
	if err := pages.ExecuteTemplate(&buf, name, data); err != nil {
		http.Error(w, "Could not render Apples for Humanity.", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

type pageView struct {
	ui.Chrome
	GameCSS string
	GameJS  string
}
