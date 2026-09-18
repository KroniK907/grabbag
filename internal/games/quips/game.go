// Package quips is Quick Quips (catalog id quips).
package quips

import (
	"bytes"
	"embed"
	"errors"
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
	assetVersion = "scaffold-1"
)

var errEngineNotReady = errors.New("quick quips match engine is not ready yet")

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
	engine  *matchEngine

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
	return nil
}

// Settings is a stub game-settings page after Load.
func (g *Game) Settings() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", g.getSettings)
	return mux
}

// Start refuses until the match engine task wires g.engine.
func (g *Game) Start(h games.Helper) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.helper = h
	if g.engine == nil {
		return errEngineNotReady
	}
	g.started = true
	g.paused = false
	return nil
}

// Board is the TV document after Start.
func (g *Game) Board(w http.ResponseWriter, r *http.Request) {
	g.render(w, "board.html", g.pageView("Quick Quips"), http.StatusOK)
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
	g.render(w, "phone.html", g.pageView("Quick Quips"), http.StatusOK)
}

// Play mounts prompt library and static assets after Load.
func (g *Game) Play() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /picker", g.getPicker)
	files, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic("quips: embedded static directory is missing")
	}
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(files))))
	return mux
}

// Pause is a no-op until the engine exists.
func (g *Game) Pause() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.paused = true
	return nil
}

// Resume clears the paused flag.
func (g *Game) Resume() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.paused = false
	return nil
}

// Stop ends the match freeze. KV and library files stay.
func (g *Game) Stop() error {
	g.mu.Lock()
	defer g.mu.Unlock()
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

func (g *Game) pageView(title string) pageView {
	view := pageView{
		Chrome:  ui.Chrome{Title: title, Theme: ui.DefaultTheme},
		GameCSS: "/play/static/game.css?v=" + assetVersion,
	}
	if h := g.helperNow(); h != nil {
		view.Chrome.Theme = ui.NormalizeTheme(h.Theme())
	}
	return view
}

func (g *Game) getSettings(w http.ResponseWriter, r *http.Request) {
	g.render(w, "settings.html", settingsView{
		Chrome:  g.pageView("Quick Quips settings").Chrome,
		Message: "Match settings will appear here in a follow-up task.",
	}, http.StatusOK)
}

func (g *Game) getPicker(w http.ResponseWriter, r *http.Request) {
	h := g.helperNow()
	if h == nil {
		http.NotFound(w, r)
		return
	}
	g.render(w, "picker.html", pickerView{
		pageView: g.pageView("Prompt Library"),
		DataDir:  h.DataDir(),
	}, http.StatusOK)
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
}

type settingsView struct {
	ui.Chrome
	Message string
}

type pickerView struct {
	pageView
	DataDir string
}
