// Package quips is Quick Quips (catalog id quips).
package quips

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/KroniK907/grabbag/internal/games"
	"github.com/KroniK907/grabbag/internal/ui"
)

const (
	id           = "quips"
	assetVersion = "match-write-1"
)

//go:embed templates/*.html
var templateFiles embed.FS

//go:embed static/*
var staticFiles embed.FS

var pages = ui.MustParse(templateFiles, "templates/*.html")

// Game is one Quick Quips instance.
type Game struct {
	mu      sync.Mutex
	helper  games.Helper
	started bool
	paused  bool
	engine     *engine
	stopTick   chan struct{}
	needFinish bool
	needPause  bool

	burns          []burnEntry
	burnCorrupt    bool
	played         map[string]playedRow
	discardCorrupt bool

	rng *rand.Rand
	now func() time.Time
}

// New constructs an unloaded Quick Quips package.
func New() *Game {
	return &Game{
		played: map[string]playedRow{},
	}
}

// ID is the catalog id quips.
func (g *Game) ID() string { return id }

// Name is the player-facing label Quick Quips.
func (g *Game) Name() string { return "Quick Quips" }

// MinPlayers is 2. Quick Quips needs at least two seated writers.
func (g *Game) MinPlayers() int { return 2 }

// MaxPlayers is 0. The host seat cap is the ceiling.
func (g *Game) MaxPlayers() int { return 0 }

// Load stores the helper, copies missing shipped libraries, and opens journal paths.
func (g *Game) Load(h games.Helper) error {
	g.mu.Lock()
	g.helper = h
	g.started = false
	g.paused = false
	g.engine = nil
	g.openJournalsLocked(h)
	g.mu.Unlock()
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
	return mux
}

// Start deals prompts and opens the write phase.
func (g *Game) Start(h games.Helper) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.helper = h
	return g.beginMatchLocked(h)
}

// Board is the TV document after Start.
func (g *Game) Board(w http.ResponseWriter, r *http.Request) {
	g.render(w, "board.html", g.boardView(), http.StatusOK)
}

// BoardButtons publishes Prompt Library on the Lobby rail after Load.
func (g *Game) BoardButtons() []games.BoardButton {
	return []games.BoardButton{{
		Label:    "Prompt Library",
		Path:     "/play/picker",
		HostOnly: true,
	}}
}

// Phone is the seated player column after Start.
func (g *Game) Phone(w http.ResponseWriter, r *http.Request) {
	g.render(w, "phone.html", g.phoneView(r), http.StatusOK)
}

// Play mounts prompt library and static assets after Load.
func (g *Game) Play() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /picker", g.getPicker)
	mux.HandleFunc("GET /partials/board", g.getBoardPartial)
	mux.HandleFunc("GET /partials/phone", g.getPhonePartial)
	mux.HandleFunc("POST /draft", g.postDraft)
	mux.HandleFunc("POST /lock", g.postLock)
	files, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic("quips: embedded static directory is missing")
	}
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(files))))
	return mux
}

// Pause freezes the write timer and blocks compose POSTs.
func (g *Game) Pause() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.paused = true
	if g.engine != nil {
		g.engine.SetPaused(true, g.clock())
	}
	return nil
}

// Resume thaws the write timer and restarts the ticker.
func (g *Game) Resume() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.paused = false
	if g.engine != nil {
		g.engine.SetPaused(false, g.clock())
	}
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
	return nil
}

// Shutdown unloads the helper.
func (g *Game) Shutdown() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.stopTickerLocked()
	g.stopWritersLocked()
	g.drainJournalsLocked()
	g.helper = nil
	g.started = false
	g.paused = false
	g.engine = nil
	g.burns = nil
	g.played = map[string]playedRow{}
	g.burnCorrupt = false
	g.discardCorrupt = false
	return nil
}

func (g *Game) helperNow() games.Helper {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.helper
}

func (g *Game) timeNow() time.Time {
	if g.now != nil {
		return g.now()
	}
	return time.Now()
}

func (g *Game) randIntn(n int) int {
	if g.rng != nil {
		return g.rng.Intn(n)
	}
	return rand.Intn(n)
}

func (g *Game) getSettings(w http.ResponseWriter, r *http.Request) {
	g.render(w, "settings.html", g.settingsView(settingsErr{}), http.StatusOK)
}

func (g *Game) render(w http.ResponseWriter, name string, data any, status int) {
	var buf bytes.Buffer
	if err := pages.ExecuteTemplate(&buf, name, data); err != nil {
		http.Error(w, fmt.Sprintf("Could not render Quick Quips (%s).", name), http.StatusInternalServerError)
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

func (g *Game) pageView(title string) pageView {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.pageViewLocked(title)
}

func (g *Game) pageViewLocked(title string) pageView {
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
