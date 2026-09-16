// Package testinggame is the Testing diagnostics package (game id testing).
package testinggame

import (
	"bytes"
	"embed"
	"io/fs"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/KroniK907/hackbox/internal/games"
	"github.com/KroniK907/hackbox/internal/ui"
)

const (
	id            = "testing"
	kvHideLatency = "hide-latency"
	eventTick     = "testing"
	eventTap      = "tap"
	flashFor      = 400 * time.Millisecond
)

// tickEvery is the room-wide SSE interval while Started. Tests may shorten it.
var tickEvery = time.Second

//go:embed templates/*.html
var templateFiles embed.FS

//go:embed static/*
var staticFiles embed.FS

var pages = ui.MustParse(templateFiles, "templates/*.html")

// Game is one Testing instance.
type Game struct {
	mu       sync.Mutex
	helper   games.Helper
	taps     map[string]int
	flashed  map[string]time.Time
	stopTick chan struct{}
}

// New constructs an unloaded Testing package.
func New() *Game {
	return &Game{
		taps:    make(map[string]int),
		flashed: make(map[string]time.Time),
	}
}

// ID is the catalog id testing.
func (g *Game) ID() string { return id }

// MinPlayers is 1. Start waits for at least one seated player.
func (g *Game) MinPlayers() int { return 1 }

// MaxPlayers is 0. The host seat cap is the ceiling.
func (g *Game) MaxPlayers() int { return 0 }

// Load stores the helper. Hide latency is read from KV on each render.
func (g *Game) Load(h games.Helper) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.helper = h
	return nil
}

// Settings is the Hide latency checkbox after Load.
func (g *Game) Settings() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", g.getSettings)
	mux.HandleFunc("POST /hide-latency", g.postHideLatency)
	return mux
}

// Start begins the 1s room-wide tick.
func (g *Game) Start(h games.Helper) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.helper = h
	g.stopTickerLocked()
	stop := make(chan struct{})
	g.stopTick = stop
	go g.runTick(stop)
	return nil
}

// Board is the Testing TV document.
func (g *Game) Board(w http.ResponseWriter, r *http.Request) {
	g.render(w, "board.html", g.boardData(r), http.StatusOK)
}

// Phone is the seated-phone body. Host wraps Lobby gear around it.
func (g *Game) Phone(w http.ResponseWriter, r *http.Request) {
	g.render(w, "phone.html", g.phoneData(r), http.StatusOK)
}

// Play mounts tap, end, and live partials after Start.
func (g *Game) Play() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /tap", g.postTap)
	mux.HandleFunc("POST /end", g.postEnd)
	mux.HandleFunc("GET /partials/board-list", g.getBoardList)
	mux.HandleFunc("GET /partials/phone-status", g.getPhoneStatus)
	files, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic("testinggame: embedded static directory is missing")
	}
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(files))))
	return mux
}

// Pause is a no-op.
func (g *Game) Pause() error { return nil }

// Resume is a no-op.
func (g *Game) Resume() error { return nil }

// Stop wipes tap counts and flash marks and ends the tick. KV stays.
func (g *Game) Stop() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.taps = make(map[string]int)
	g.flashed = make(map[string]time.Time)
	g.stopTickerLocked()
	return nil
}

// Shutdown unloads the helper and ends the tick if it is still running.
func (g *Game) Shutdown() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.stopTickerLocked()
	g.helper = nil
	g.taps = make(map[string]int)
	g.flashed = make(map[string]time.Time)
	return nil
}

func (g *Game) stopTickerLocked() {
	if g.stopTick != nil {
		close(g.stopTick)
		g.stopTick = nil
	}
}

func (g *Game) runTick(stop <-chan struct{}) {
	ticker := time.NewTicker(tickEvery)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			g.mu.Lock()
			h := g.helper
			g.mu.Unlock()
			if h != nil {
				h.Publish(eventTick)
			}
		}
	}
}

func (g *Game) getSettings(w http.ResponseWriter, r *http.Request) {
	g.render(w, "settings.html", settingsView{HideLatency: g.hideLatency()}, http.StatusOK)
}

func (g *Game) postHideLatency(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Could not read the form.", http.StatusBadRequest)
		return
	}
	h := g.helperNow()
	if h == nil {
		http.NotFound(w, r)
		return
	}
	value := []byte("0")
	if r.FormValue("hide") == "1" {
		value = []byte("1")
	}
	if err := h.KVSet(kvHideLatency, value); err != nil {
		http.Error(w, "Could not save Hide latency.", http.StatusInternalServerError)
		return
	}
	h.Publish("roster")
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

func (g *Game) postTap(w http.ResponseWriter, r *http.Request) {
	h := g.helperNow()
	if h == nil {
		http.NotFound(w, r)
		return
	}
	player, ok, err := h.PlayerFromRequest(r)
	if err != nil || !ok || !player.Seated {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	g.mu.Lock()
	g.taps[player.ID]++
	g.flashed[player.ID] = time.Now().Add(flashFor)
	g.mu.Unlock()
	h.Publish(eventTap)
	w.WriteHeader(http.StatusNoContent)
}

func (g *Game) postEnd(w http.ResponseWriter, r *http.Request) {
	h := g.helperNow()
	if h == nil {
		http.NotFound(w, r)
		return
	}
	if !g.mayEnd(h, r) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	h.Finish()
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (g *Game) mayEnd(h games.Helper, r *http.Request) bool {
	if h.HasAdmin(r) {
		return true
	}
	player, ok, err := h.PlayerFromRequest(r)
	return err == nil && ok && player.ClaimedHost
}

func (g *Game) getBoardList(w http.ResponseWriter, r *http.Request) {
	g.render(w, "board-list", g.boardData(r), http.StatusOK)
}

func (g *Game) getPhoneStatus(w http.ResponseWriter, r *http.Request) {
	g.render(w, "phone-status", g.phoneData(r), http.StatusOK)
}

func (g *Game) boardData(r *http.Request) boardView {
	h := g.helperNow()
	view := boardView{
		Chrome:      ui.Page("Testing"),
		HideLatency: g.hideLatency(),
	}
	if h != nil {
		view.ShowEnd = h.HasAdmin(r)
		now := time.Now()
		g.mu.Lock()
		for _, p := range h.Seated() {
			row := playerRow{
				ID:          p.ID,
				DisplayName: p.DisplayName,
				AvatarSeed:  p.AvatarSeed,
				Connected:   p.Connected,
				Taps:        g.taps[p.ID],
				Flash:       now.Before(g.flashed[p.ID]),
			}
			if p.LastHeartbeatRTT > 0 {
				row.Latency = formatLatency(p.LastHeartbeatRTT)
			}
			view.Players = append(view.Players, row)
		}
		g.mu.Unlock()
	}
	return view
}

func (g *Game) phoneData(r *http.Request) phoneView {
	h := g.helperNow()
	view := phoneView{}
	if h == nil {
		return view
	}
	player, ok, err := h.PlayerFromRequest(r)
	if err != nil || !ok {
		return view
	}
	view.DisplayName = player.DisplayName
	if player.LastHeartbeatRTT > 0 {
		view.Latency = formatLatency(player.LastHeartbeatRTT)
	}
	view.ShowEnd = player.ClaimedHost
	return view
}

func (g *Game) hideLatency() bool {
	h := g.helperNow()
	if h == nil {
		return false
	}
	raw, ok, err := h.KVGet(kvHideLatency)
	return err == nil && ok && string(raw) == "1"
}

func (g *Game) helperNow() games.Helper {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.helper
}

func (g *Game) render(w http.ResponseWriter, name string, data any, status int) {
	var buf bytes.Buffer
	if err := pages.ExecuteTemplate(&buf, name, data); err != nil {
		http.Error(w, "Could not render Testing.", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

func formatLatency(d time.Duration) string {
	return strconv.FormatInt(d.Milliseconds(), 10) + " ms"
}

type settingsView struct {
	HideLatency bool
}

type boardView struct {
	ui.Chrome
	Players     []playerRow
	HideLatency bool
	ShowEnd     bool
}

type phoneView struct {
	DisplayName string
	Latency     string
	ShowEnd     bool
}

type playerRow struct {
	ID          string
	DisplayName string
	AvatarSeed  string
	Connected   bool
	Latency     string
	Taps        int
	Flash       bool
}
