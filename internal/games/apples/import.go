package apples

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/KroniK907/grabbag/internal/games"
)

// officialDumpURL is the JSON Against Humanity full dump. Tests replace Game.fetchURL.
const officialDumpURL = "https://raw.githubusercontent.com/crhallberg/json-against-humanity/latest/cah-all-full.json"

func (g *Game) postImportOfficial(w http.ResponseWriter, r *http.Request) {
	h := g.run.Helper()
	if h == nil {
		http.NotFound(w, r)
		return
	}
	if g.matchFrozen() {
		g.run.Render(w, "picker.html", g.pickerView(pickerErr{Msg: "Pack changes wait until the game ends."}), http.StatusOK)
		return
	}
	if err := g.importOfficial(h); err != nil {
		g.run.Lock()
		g.importErr = err.Error()
		g.run.Unlock()
		g.run.Render(w, "picker.html", g.pickerView(pickerErr{Msg: err.Error()}), http.StatusOK)
		return
	}
	g.run.Lock()
	g.importErr = ""
	g.run.Unlock()
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
