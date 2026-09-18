package quips

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KroniK907/grabbag/internal/games"
)

func TestPickerRequiresAdmin(t *testing.T) {
	t.Parallel()
	h := newFakeHelper(t.TempDir())
	g := loadedPickerGame(t, h)
	rec := httptest.NewRecorder()
	g.Play().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/picker", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestFirstLoadEnablesShippedPackNewPackOff(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	h := newAdminHelper(dir)
	g := loadedPickerGame(t, h)
	house := `{"formatVersion":1,"id":"house","name":"House","packs":[{"id":"house","name":"House","prompts":[{"id":"h-p","text":"Hi _"}]}]}`
	if err := os.WriteFile(filepath.Join(dir, "house.json"), []byte(house), 0o600); err != nil {
		t.Fatal(err)
	}
	page := pickerPage(t, g)
	assertPackCheckbox(t, page, "quips", "comedy", true)
	assertPackCheckbox(t, page, "house", "house", false)
}

func TestFailedLibraryIsGrayRow(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	h := newAdminHelper(dir)
	g := loadedPickerGame(t, h)
	if err := os.WriteFile(filepath.Join(dir, "broken.json"), []byte(`{"formatVersion": 2, "id": "x", "name": "X", "packs": []}`), 0o600); err != nil {
		t.Fatal(err)
	}
	page := pickerPage(t, g)
	if !strings.Contains(page, "broken.json") || !strings.Contains(page, "formatVersion must be 1") || !strings.Contains(page, "quips-pack bad") {
		t.Fatalf("failed row missing: %s", page)
	}
	if strings.Contains(page, `name="library" value="x"`) {
		t.Fatal("failed library was enableable")
	}
}

func TestTogglePackPersistsAndFreezesWhenStarted(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	h := newAdminHelper(dir)
	g := loadedPickerGame(t, h)
	rec := postPickerSettings(g, "/pack", url.Values{
		"library": {"quips"},
		"pack":    {"comedy"},
		"enabled": {"0"},
	})
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/play/picker" {
		t.Fatalf("toggle = %d %q", rec.Code, rec.Header().Get("Location"))
	}
	assertPackCheckbox(t, pickerPage(t, g), "quips", "comedy", false)

	g.mu.Lock()
	g.engine = &matchEngine{}
	g.mu.Unlock()
	if err := g.Start(h); err != nil {
		t.Fatal(err)
	}
	frozen := postPickerSettings(g, "/pack", url.Values{
		"library": {"quips"},
		"pack":    {"comedy"},
		"enabled": {"1"},
	})
	if frozen.Code != http.StatusOK || !strings.Contains(frozen.Body.String(), "Pack changes wait until the game ends") {
		t.Fatalf("freeze = %d %s", frozen.Code, frozen.Body.String())
	}
}

func TestPickerShortageOnSettingsAndPicker(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	h := newAdminHelper(dir)
	h.seated = []games.Player{{ID: "p1"}, {ID: "p2"}, {ID: "p3"}, {ID: "p4"}}
	g := loadedPickerGame(t, h)
	postPickerSettings(g, "/select-none", nil)
	for _, page := range []string{settingsPage(t, g), pickerPage(t, g)} {
		if !strings.Contains(page, startRefuseMsg) {
			t.Fatalf("missing shortage: %s", page)
		}
	}
}

func TestPickerTogglesUseHTMX(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	h := newAdminHelper(dir)
	g := loadedPickerGame(t, h)
	page := pickerPage(t, g)
	for _, want := range []string{
		`id="quips-picker"`,
		`hx-post="/settings/game/pack"`,
		`hx-post="/settings/game/select-all"`,
		`hx-target="#quips-picker"`,
		`hx-swap="outerHTML"`,
	} {
		if !strings.Contains(page, want) {
			t.Fatalf("picker missing %s", want)
		}
	}
}

func loadedPickerGame(t *testing.T, h *fakeHelper) *Game {
	t.Helper()
	g := New()
	if err := g.Load(h); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = g.Shutdown() })
	return g
}

func newAdminHelper(dir string) *fakeHelper {
	h := newFakeHelper(dir)
	h.admin = true
	return h
}

func pickerPage(t *testing.T, g *Game) string {
	t.Helper()
	rec := httptest.NewRecorder()
	g.Play().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/picker", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET picker = %d", rec.Code)
	}
	return rec.Body.String()
}

func postPickerSettings(g *Game, path string, vals url.Values) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(vals.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	g.Settings().ServeHTTP(rec, req)
	return rec
}

func assertPackCheckbox(t *testing.T, page, library, pack string, want bool) {
	t.Helper()
	row := packArticle(page, library, pack)
	if row == "" {
		t.Fatalf("pack %s/%s not found in page", library, pack)
	}
	checked := strings.Contains(row, "checked")
	if checked != want {
		t.Fatalf("pack %s/%s checked=%v want %v in %s", library, pack, checked, want, row)
	}
}

func packArticle(page, library, pack string) string {
	libNeedle := `name="library" value="` + library + `"`
	packNeedle := `name="pack" value="` + pack + `"`
	for _, chunk := range strings.Split(page, "<article") {
		if !strings.Contains(chunk, libNeedle) || !strings.Contains(chunk, packNeedle) {
			continue
		}
		return "<article" + chunk
	}
	return ""
}
