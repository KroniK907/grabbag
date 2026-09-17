// Package apples is Apples for Humanity (catalog id apples).
package apples

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/KroniK907/grabbag/internal/games"
	"github.com/KroniK907/grabbag/internal/ui"
)

const (
	id         = "apples"
	cssVersion = "picker-1"
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
	mux.HandleFunc("POST /select-all", g.postSelectAll)
	mux.HandleFunc("POST /select-none", g.postSelectNone)
	mux.HandleFunc("POST /import-official", g.postImportOfficial)
	return mux
}

// Start marks the match running so picker toggles freeze.
func (g *Game) Start(h games.Helper) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.helper = h
	g.started = true
	g.paused = false
	return nil
}

// Board is a stub TV document until the round-loop task.
func (g *Game) Board(w http.ResponseWriter, r *http.Request) {
	g.render(w, "board.html", g.chromeView("Apples for Humanity"), http.StatusOK)
}

// BoardButtons publishes Deck Library on the Lobby rail after Load.
func (g *Game) BoardButtons() []games.BoardButton {
	return []games.BoardButton{{
		Label:    "Deck Library",
		Path:     "/play/picker",
		HostOnly: true,
	}}
}

// Phone is a stub seated-phone body until the round-loop task.
func (g *Game) Phone(w http.ResponseWriter, r *http.Request) {
	g.render(w, "phone.html", g.chromeView("Apples for Humanity"), http.StatusOK)
}

// Play mounts the Deck Library GET page and game CSS.
func (g *Game) Play() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /picker", g.getPicker)
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
	return nil
}

// Resume clears the paused flag. Toggles stay frozen until Stop.
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
	g.started = false
	g.paused = false
	return nil
}

// Shutdown unloads the helper.
func (g *Game) Shutdown() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.helper = nil
	g.started = false
	g.paused = false
	g.importErr = ""
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
		GameCSS: "/play/static/game.css?v=" + cssVersion,
	}
	if h := g.helperNow(); h != nil {
		view.Chrome.Theme = ui.NormalizeTheme(h.Theme())
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
	g.render(w, "settings.html", g.pickerView(pickerErr{}), http.StatusOK)
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
		g.render(w, "picker.html", g.pickerView(pickerErr{
			Msg: "Pack changes wait until the game ends.", LibraryID: libraryID, PackID: packID,
		}), http.StatusOK)
		return
	}
	cat := scanDataDir(h.DataDir())
	if !packExists(cat, libraryID, packID) {
		g.render(w, "picker.html", g.pickerView(pickerErr{
			Msg: "That pack is not in the library.", LibraryID: libraryID, PackID: packID,
		}), http.StatusOK)
		return
	}
	settings, ok := g.loadSettings(h)
	settings = reconcileSettings(settings, ok, cat)
	settings.setPack(libraryID, packID, on)
	if err := g.saveSettings(h, settings); err != nil {
		g.render(w, "picker.html", g.pickerView(pickerErr{
			Msg: "Could not save pack enablement.", LibraryID: libraryID, PackID: packID,
		}), http.StatusOK)
		return
	}
	http.Redirect(w, r, "/play/picker", http.StatusSeeOther)
}

func (g *Game) selectAll(w http.ResponseWriter, r *http.Request, on bool) {
	h := g.helperNow()
	if h == nil {
		http.NotFound(w, r)
		return
	}
	if g.matchFrozen() {
		g.render(w, "picker.html", g.pickerView(pickerErr{Msg: "Pack changes wait until the game ends."}), http.StatusOK)
		return
	}
	cat := scanDataDir(h.DataDir())
	settings := matchSettings{}
	settings.setAll(cat, on)
	if err := g.saveSettings(h, settings); err != nil {
		g.render(w, "picker.html", g.pickerView(pickerErr{Msg: "Could not save pack enablement."}), http.StatusOK)
		return
	}
	http.Redirect(w, r, "/play/picker", http.StatusSeeOther)
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

func (g *Game) pickerView(rowErr pickerErr) pickerView {
	view := pickerView{
		pageView:     g.chromeView("Deck Library"),
		RowError:     rowErr.Msg,
		ErrorLibrary: rowErr.LibraryID,
		ErrorPack:    rowErr.PackID,
	}
	h := g.helperNow()
	if h == nil {
		return view
	}
	view.DataDir = h.DataDir()
	view.Frozen = g.matchFrozen()
	g.mu.Lock()
	if view.RowError == "" {
		view.RowError = g.importErr
	}
	g.mu.Unlock()
	cat := scanDataDir(h.DataDir())
	settings, ok := g.loadSettings(h)
	settings = reconcileSettings(settings, ok, cat)
	for _, lib := range cat.Libraries {
		item := libraryView{
			ID:          lib.ID,
			Name:        lib.Name,
			Description: lib.Description,
			License:     lib.License,
			Filename:    lib.Source,
		}
		for _, pack := range lib.Packs {
			item.Packs = append(item.Packs, packView{
				LibraryID:   lib.ID,
				ID:          pack.ID,
				Name:        pack.Name,
				Description: pack.Description,
				Enabled:     settings.packOn(lib.ID, pack.ID),
				PromptCount: len(pack.Prompts),
				AnswerCount: len(pack.Answers),
			})
		}
		view.Libraries = append(view.Libraries, item)
	}
	view.Failed = cat.Failed
	return view
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
}

type pageView struct {
	ui.Chrome
	GameCSS string
}

type pickerView struct {
	pageView
	DataDir      string
	Frozen       bool
	RowError     string
	ErrorLibrary string
	ErrorPack    string
	Libraries    []libraryView
	Failed       []failedFile
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
}
