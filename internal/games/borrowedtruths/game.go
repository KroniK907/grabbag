// Package borrowedtruths is Borrowed Truths (catalog id borrowedtruths): Would
// I Lie to You? for the couch, where some true stories belong to someone else.
package borrowedtruths

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/KroniK907/grabbag/internal/games"
	"github.com/KroniK907/grabbag/internal/ui"
)

const (
	id           = "borrowedtruths"
	assetVersion = "first-pass-1"
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

// Game is one Borrowed Truths instance.
type Game struct {
	mu         sync.Mutex
	helper     games.Helper
	started    bool
	paused     bool
	engine     *engine
	stopTick   chan struct{}
	needFinish bool
	needPause  bool
	needResume bool
	// runDir holds this match's This Is My photos. Stop and Shutdown delete it.
	runDir string
	// photoSrc replaces photo URLs in previews, which have no files.
	photoSrc func(id string) template.URL

	rng *rand.Rand
	now func() time.Time
}

// New constructs an unloaded Borrowed Truths package.
func New() *Game { return &Game{} }

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

// Load stores the helper.
func (g *Game) Load(h games.Helper) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.helper = h
	g.started = false
	g.paused = false
	g.engine = nil
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
	g.mu.Lock()
	defer g.mu.Unlock()
	g.helper = h
	seated := h.Seated()
	if len(seated) < g.MinPlayers() {
		return fmt.Errorf("Borrowed Truths needs at least %d seated players.", g.MinPlayers())
	}
	rows := make([]rosterRow, 0, len(seated))
	for _, p := range seated {
		rows = append(rows, rosterRow{ID: p.ID, Name: p.DisplayName, Seed: p.AvatarSeed})
	}
	g.dropRunLocked()
	g.engine = newEngine(g.loadSettings(h), rows, bank, g.rngLocked(), g.clock())
	g.started = true
	g.paused = false
	g.startTickerLocked()
	return nil
}

// Board is the TV document after Start.
func (g *Game) Board(w http.ResponseWriter, r *http.Request) {
	g.render(w, "board.html", g.boardView(), http.StatusOK)
}

// BoardButtons publishes How to play on the Lobby rail.
func (g *Game) BoardButtons() []games.BoardButton {
	return []games.BoardButton{{Label: "How to play", Path: "/play/howto"}}
}

// Phone is the player column after Start.
func (g *Game) Phone(w http.ResponseWriter, r *http.Request) {
	g.render(w, "phone.html", g.phoneView(r, ""), http.StatusOK)
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
	mux.HandleFunc("POST /host/pause", g.postHostPause)
	mux.HandleFunc("POST /host/resume", g.postHostResume)
	files, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic("borrowedtruths: embedded static directory is missing")
	}
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(files))))
	return mux
}

// Pause freezes the phase timer and blocks phone posts.
func (g *Game) Pause() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.paused = true
	if g.engine != nil {
		g.engine.SetPaused(true, g.clock())
	}
	return nil
}

// Resume thaws the phase timer.
func (g *Game) Resume() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.paused = false
	if g.engine != nil {
		g.engine.SetPaused(false, g.clock())
	}
	return nil
}

// Stop drops the match. Facts and photos are run state, so they go with it.
// KV stays.
func (g *Game) Stop() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.stopTickerLocked()
	g.dropRunLocked()
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
	g.dropRunLocked()
	g.helper = nil
	g.started = false
	g.paused = false
	g.engine = nil
	return nil
}

func (g *Game) helperNow() games.Helper {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.helper
}

func (g *Game) clock() time.Time {
	if g.now != nil {
		return g.now()
	}
	return time.Now()
}

func (g *Game) rngLocked() *rand.Rand {
	if g.rng == nil {
		g.rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	return g.rng
}

func (g *Game) publishLocked() {
	if g.helper != nil {
		g.helper.Publish(event)
	}
}

// publishFactsLocked is a facts tick. Only the board listens, so a phone
// mid-write is not swapped.
func (g *Game) publishFactsLocked() {
	if g.helper != nil {
		g.helper.Publish(eventFacts)
	}
}

func (g *Game) startTickerLocked() {
	g.stopTickerLocked()
	stop := make(chan struct{})
	g.stopTick = stop
	go func() {
		tick := time.NewTicker(250 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-stop:
				return
			case <-tick.C:
				g.mu.Lock()
				if g.engine != nil && g.started && !g.paused {
					finish, changed := g.engine.Advance(g.clock())
					g.afterLocked(finish, changed)
				}
				g.mu.Unlock()
			}
		}
	}()
}

func (g *Game) stopTickerLocked() {
	if g.stopTick != nil {
		close(g.stopTick)
		g.stopTick = nil
	}
}

// afterLocked publishes a change and runs host calls that must happen with
// the lock released.
func (g *Game) afterLocked(finish, changed bool) {
	if changed {
		g.publishLocked()
	}
	if finish {
		g.needFinish = true
	}
	h := g.helper
	pause, resume, fin := g.needPause, g.needResume, g.needFinish
	g.needPause, g.needResume, g.needFinish = false, false, false
	if h == nil || !(pause || resume || fin) {
		return
	}
	g.mu.Unlock()
	if pause {
		h.Pause()
	}
	if resume {
		h.Resume()
	}
	if fin {
		h.Finish()
	}
	g.mu.Lock()
}

func (g *Game) render(w http.ResponseWriter, name string, data any, status int) {
	var buf bytes.Buffer
	if err := pages.ExecuteTemplate(&buf, name, data); err != nil {
		http.Error(w, fmt.Sprintf("Could not render Borrowed Truths (%s).", name), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

func (g *Game) getHowto(w http.ResponseWriter, r *http.Request) {
	if g.helperNow() == nil {
		http.NotFound(w, r)
		return
	}
	g.render(w, "howto.html", g.pageView("How to play Borrowed Truths"), http.StatusOK)
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
