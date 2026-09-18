// Package apples is Apples for Humanity (catalog id apples).
package apples

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/KroniK907/grabbag/internal/games"
	"github.com/KroniK907/grabbag/internal/ui"
)

const (
	id           = "apples"
	assetVersion = "match-10"
	// officialDumpURL is the JSON Against Humanity full dump. Tests replace Game.fetchURL.
	officialDumpURL = "https://raw.githubusercontent.com/crhallberg/json-against-humanity/latest/cah-all-full.json"
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
	match      *matchState
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

// MaxPlayers is 0. The host seat cap is the ceiling.
func (g *Game) MaxPlayers() int { return 0 }

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
	mux.HandleFunc("POST /timer-auto-draw", g.postAutoDraw)
	mux.HandleFunc("POST /timer-submit", g.postSubmitSec)
	mux.HandleFunc("POST /timer-between", g.postBetweenSec)
	mux.HandleFunc("POST /timer-judge-pick", g.postJudgePickSec)
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
	g.render(w, "board.html", g.boardView(), http.StatusOK)
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
	mux.HandleFunc("POST /draw", g.postDraw)
	mux.HandleFunc("POST /skip", g.postSkip)
	mux.HandleFunc("POST /keep-prompt", g.postKeepPrompt)
	mux.HandleFunc("POST /choose-prompt", g.postChoosePrompt)
	mux.HandleFunc("POST /slot", g.postSlot)
	mux.HandleFunc("POST /unslot", g.postUnslot)
	mux.HandleFunc("POST /discard", g.postDiscard)
	mux.HandleFunc("POST /lock", g.postLock)
	mux.HandleFunc("POST /wildcard-draft", g.postWildcardDraft)
	mux.HandleFunc("POST /reveal", g.postReveal)
	mux.HandleFunc("POST /vote", g.postVote)
	mux.HandleFunc("POST /confirm", g.postConfirm)
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
	if g.match != nil {
		g.freezeTimerLocked(g.match)
	}
	return nil
}

// Resume clears the paused flag. Toggles stay frozen until Stop.
func (g *Game) Resume() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.paused = false
	if g.match != nil && g.overlay == "" {
		g.thawTimerLocked(g.match)
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
	g.match = nil
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
	g.match = nil
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

func hxRequest(r *http.Request) bool {
	return r.Header.Get("HX-Request") != ""
}

func (g *Game) writePicker(w http.ResponseWriter, r *http.Request, rowErr pickerErr) {
	name := "picker.html"
	if hxRequest(r) {
		name = "picker-body"
	}
	g.render(w, name, g.pickerView(rowErr), http.StatusOK)
}

func (g *Game) writePickerOK(w http.ResponseWriter, r *http.Request) {
	if hxRequest(r) {
		g.writePicker(w, r, pickerErr{})
		return
	}
	http.Redirect(w, r, "/play/picker", http.StatusSeeOther)
}

func (g *Game) getPicker(w http.ResponseWriter, r *http.Request) {
	h := g.helperNow()
	if h == nil {
		http.NotFound(w, r)
		return
	}
	if !h.HasAdmin(r) {
		http.Error(w, "Admin session required.", http.StatusUnauthorized)
		return
	}
	g.render(w, "picker.html", g.pickerView(pickerErr{}), http.StatusOK)
}

func (g *Game) getSettings(w http.ResponseWriter, r *http.Request) {
	g.render(w, "settings.html", g.settingsView(settingsErr{}), http.StatusOK)
}

func (g *Game) postPack(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Could not read the form.", http.StatusBadRequest)
		return
	}
	libraryID := r.FormValue("library")
	packID := r.FormValue("pack")
	g.togglePack(w, r, libraryID, packID, formEnabled(r))
}

func formEnabled(r *http.Request) bool {
	for _, v := range r.Form["enabled"] {
		if v == "1" {
			return true
		}
	}
	return false
}

func (g *Game) postTag(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Could not read the form.", http.StatusBadRequest)
		return
	}
	g.toggleTag(w, r, r.FormValue("tag"), formEnabled(r))
}

func (g *Game) toggleTag(w http.ResponseWriter, r *http.Request, tag string, on bool) {
	h := g.helperNow()
	if h == nil {
		http.NotFound(w, r)
		return
	}
	if g.matchFrozen() {
		g.writePicker(w, r, pickerErr{
			Msg: "Tag changes wait until the game ends.", Tag: tag,
		})
		return
	}
	cat := scanDataDir(h.DataDir())
	if !catalogHasTag(cat, tag) {
		g.writePicker(w, r, pickerErr{
			Msg: "That tag is not in the library.", Tag: tag,
		})
		return
	}
	settings, ok := g.loadSettings(h)
	settings = reconcileSettings(settings, ok, cat)
	settings.setTag(tag, on)
	if err := g.saveSettings(h, settings); err != nil {
		g.writePicker(w, r, pickerErr{
			Msg: "Could not save tag enablement.", Tag: tag,
		})
		return
	}
	g.writePickerOK(w, r)
}

func (g *Game) postSelectAll(w http.ResponseWriter, r *http.Request) {
	g.selectAll(w, r, true)
}

func (g *Game) postSelectNone(w http.ResponseWriter, r *http.Request) {
	g.selectAll(w, r, false)
}

func (g *Game) togglePack(w http.ResponseWriter, r *http.Request, libraryID, packID string, on bool) {
	h := g.helperNow()
	if h == nil {
		http.NotFound(w, r)
		return
	}
	if g.matchFrozen() {
		g.writePicker(w, r, pickerErr{
			Msg: "Pack changes wait until the game ends.", LibraryID: libraryID, PackID: packID,
		})
		return
	}
	cat := scanDataDir(h.DataDir())
	if !packExists(cat, libraryID, packID) {
		g.writePicker(w, r, pickerErr{
			Msg: "That pack is not in the library.", LibraryID: libraryID, PackID: packID,
		})
		return
	}
	settings, ok := g.loadSettings(h)
	settings = reconcileSettings(settings, ok, cat)
	settings.setPack(libraryID, packID, on)
	if err := g.saveSettings(h, settings); err != nil {
		g.writePicker(w, r, pickerErr{
			Msg: "Could not save pack enablement.", LibraryID: libraryID, PackID: packID,
		})
		return
	}
	g.writePickerOK(w, r)
}

func (g *Game) selectAll(w http.ResponseWriter, r *http.Request, on bool) {
	h := g.helperNow()
	if h == nil {
		http.NotFound(w, r)
		return
	}
	if g.matchFrozen() {
		g.writePicker(w, r, pickerErr{Msg: "Pack changes wait until the game ends."})
		return
	}
	cat := scanDataDir(h.DataDir())
	settings, ok := g.loadSettings(h)
	settings = reconcileSettings(settings, ok, cat)
	settings.setAllEnabled(cat, on)
	if err := g.saveSettings(h, settings); err != nil {
		g.writePicker(w, r, pickerErr{Msg: "Could not save pack enablement."})
		return
	}
	g.writePickerOK(w, r)
}

func (g *Game) postImportOfficial(w http.ResponseWriter, r *http.Request) {
	h := g.helperNow()
	if h == nil {
		http.NotFound(w, r)
		return
	}
	if g.matchFrozen() {
		g.render(w, "picker.html", g.pickerView(pickerErr{Msg: "Pack changes wait until the game ends."}), http.StatusOK)
		return
	}
	if err := g.importOfficial(h); err != nil {
		g.mu.Lock()
		g.importErr = err.Error()
		g.mu.Unlock()
		g.render(w, "picker.html", g.pickerView(pickerErr{Msg: err.Error()}), http.StatusOK)
		return
	}
	g.mu.Lock()
	g.importErr = ""
	g.mu.Unlock()
	http.Redirect(w, r, "/play/picker", http.StatusSeeOther)
}

func (g *Game) importOfficial(h games.Helper) error {
	client := g.httpClient
	url := g.fetchURL
	if client == nil {
		client = http.DefaultClient
	}
	if url == "" {
		url = officialDumpURL
	}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("Could not download official packs.")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Could not download official packs.")
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return fmt.Errorf("Could not download official packs.")
	}
	cahDir := filepath.Join(h.DataDir(), "cah")
	if err := os.MkdirAll(cahDir, 0o700); err != nil {
		return fmt.Errorf("Could not download official packs.")
	}
	if err := os.WriteFile(filepath.Join(cahDir, "json-against-humanity.json"), raw, 0o600); err != nil {
		return fmt.Errorf("Could not download official packs.")
	}
	lib, err := convertOfficial(raw)
	if err != nil {
		return fmt.Errorf("Could not convert official packs.")
	}
	encoded, err := json.MarshalIndent(lib, "", "  ")
	if err != nil {
		return fmt.Errorf("Could not convert official packs.")
	}
	tmp := filepath.Join(h.DataDir(), officialFileName+".tmp")
	dest := filepath.Join(h.DataDir(), officialFileName)
	if err := os.WriteFile(tmp, encoded, 0o600); err != nil {
		return fmt.Errorf("Could not convert official packs.")
	}
	if err := os.Rename(tmp, dest); err != nil {
		return fmt.Errorf("Could not convert official packs.")
	}
	return g.persistSettings()
}

func (g *Game) persistSettings() error {
	h := g.helperNow()
	if h == nil {
		return fmt.Errorf("game is not loaded")
	}
	cat := scanDataDir(h.DataDir())
	existing, ok := g.loadSettings(h)
	next := reconcileSettings(existing, ok, cat)
	return g.saveSettings(h, next)
}

func (g *Game) loadSettings(h games.Helper) (matchSettings, bool) {
	raw, found, err := h.KVGet(kvMatchSettings)
	if err != nil || !found {
		return matchSettings{}, false
	}
	return parseMatchSettings(raw)
}

func (g *Game) saveSettings(h games.Helper, s matchSettings) error {
	raw, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return h.KVSet(kvMatchSettings, raw)
}

func packExists(cat catalog, libraryID, packID string) bool {
	for _, lib := range cat.Libraries {
		if lib.ID != libraryID {
			continue
		}
		for _, pack := range lib.Packs {
			if pack.ID == packID {
				return true
			}
		}
	}
	return false
}

type pickerErr struct {
	Msg       string
	LibraryID string
	PackID    string
	Tag       string
}

type pageView struct {
	ui.Chrome
	GameCSS string
	GameJS  string
}

type pickerView struct {
	pageView
	DataDir      string
	Frozen       bool
	RowError     string
	ErrorLibrary string
	ErrorPack    string
	ErrorTag     string
	Shortage     []string
	Tags         []tagView
	Libraries    []libraryView
	Failed       []failedFile
}

type tagView struct {
	ID      string
	Label   string
	Color   string
	Enabled bool
}

type libraryView struct {
	ID          string
	Name        string
	Description string
	License     string
	Filename    string
	Packs       []packView
}

type packView struct {
	LibraryID   string
	ID          string
	Name        string
	Description string
	Enabled     bool
	PromptCount int
	AnswerCount int
	Dots        []tagView
}
