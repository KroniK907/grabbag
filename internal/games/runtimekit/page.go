package runtimekit

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"

	"github.com/KroniK907/grabbag/internal/games"
	"github.com/KroniK907/grabbag/internal/ui"
)

// Page is the chrome and asset links every game document starts from. Embed
// it in board, phone, and settings views.
type Page struct {
	ui.Chrome
	GameCSS string
	GameJS  string
}

// NewPage stamps the room theme from h, or the default theme when h is nil,
// and links /play/static/game.css and game.js at version.
func NewPage(title string, h games.Helper, version string) Page {
	page := Page{
		Chrome:  ui.Chrome{Title: title, Theme: ui.DefaultTheme},
		GameCSS: "/play/static/game.css?v=" + version,
		GameJS:  "/play/static/game.js?v=" + version,
	}
	if h != nil {
		page.Chrome.Theme = ui.NormalizeTheme(h.Theme())
	}
	return page
}

// Render executes the named template into a buffer, then writes it with
// status. A template error is a 500 that names the game and template.
func Render(w http.ResponseWriter, pages *template.Template, game, name string, data any, status int) {
	var buf bytes.Buffer
	if err := pages.ExecuteTemplate(&buf, name, data); err != nil {
		http.Error(w, fmt.Sprintf("Could not render %s (%s).", game, name), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

// Static serves the static/ folder of an embedded game FS. Mount it on Play
// at "GET /static/". The game embeds static/* itself.
func Static(files fs.FS) http.Handler {
	sub, err := fs.Sub(files, "static")
	if err != nil {
		panic("runtimekit: " + err.Error())
	}
	return http.StripPrefix("/static/", http.FileServer(http.FS(sub)))
}

// Assets is the shell asset list for a game that ships static/game.css and
// static/game.js, served by Static under Play at /play/static/. The query
// carries the game id so two games never share a cached URL.
func Assets(id, version string) ui.Assets {
	q := "?v=" + id + "-" + version
	return ui.Assets{
		CSS: []string{"/play/static/game.css" + q},
		JS:  []string{"/play/static/game.js" + q},
	}
}
