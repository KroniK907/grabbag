package apples

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KroniK907/grabbag/internal/games"
	"github.com/KroniK907/grabbag/internal/ui"
)

func TestCatalogRegistersApples(t *testing.T) {
	t.Parallel()
	var got games.Factory
	for _, f := range games.Catalog() {
		if f.ID == "apples" {
			got = f
		}
	}
	if got.New == nil {
		t.Fatal("apples is not in the catalog")
	}
	g := got.New()
	if g.ID() != "apples" || g.Name() != "Apples for Humanity" || g.MinPlayers() != 1 || g.MaxPlayers() != 0 {
		t.Fatalf("id=%s name=%s min=%d max=%d", g.ID(), g.Name(), g.MinPlayers(), g.MaxPlayers())
	}
	buttons := g.BoardButtons()
	if len(buttons) != 2 || buttons[0].Label != "How to play" || buttons[0].Path != "/play/howto" || buttons[0].HostOnly {
		t.Fatalf("BoardButtons howto = %#v", buttons)
	}
	if buttons[1].Label != "Deck Library" || buttons[1].Path != "/play/picker" || !buttons[1].HostOnly {
		t.Fatalf("BoardButtons picker = %#v", buttons)
	}
}

func TestLoadCopiesShippedLibrariesOnce(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	h := newFakeHelper(dir, true)
	g := loadedGame(t, h)

	for _, name := range []string{"oranges.json", "white-black.json", "wildcard.json"} {
		path := filepath.Join(dir, name)
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
		if !strings.Contains(string(raw), `"formatVersion": 1`) {
			t.Fatalf("%s missing formatVersion", name)
		}
	}

	marker := filepath.Join(dir, "oranges.json")
	if err := os.WriteFile(marker, []byte(`{"keep":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := g.Load(h); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(marker)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"keep":true}` {
		t.Fatalf("Load overwrote oranges.json: %s", raw)
	}
}

func TestFirstLoadEnablesFamilyPacksOnly(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	h := newFakeHelper(dir, true)
	g := loadedGame(t, h)
	house := `{
  "formatVersion": 1,
  "id": "house",
  "name": "House",
  "packs": [{"id": "house", "name": "House", "prompts": [{"id": "h-p", "text": "Hi _"}], "answers": [{"id": "h-a", "text": "There"}]}]
}`
	if err := os.WriteFile(filepath.Join(dir, "house.json"), []byte(house), 0o600); err != nil {
		t.Fatal(err)
	}

	page := picker(t, g)
	if !strings.Contains(page, dir) {
		t.Fatal("picker missing DataDir")
	}
	assertCheckbox(t, page, "oranges", "oranges", true)
	assertCheckbox(t, page, "white-black", "white-black", true)
	assertCheckbox(t, page, "wildcard", "wildcard", false)
	assertCheckbox(t, page, "house", "house", false)
}

func TestFailedLibraryIsGrayRow(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	h := newFakeHelper(dir, true)
	g := loadedGame(t, h)
	if err := os.WriteFile(filepath.Join(dir, "broken.json"), []byte(`{"formatVersion": 2, "id": "x", "name": "X", "packs": []}`), 0o600); err != nil {
		t.Fatal(err)
	}
	page := picker(t, g)
	if !strings.Contains(page, "broken.json") || !strings.Contains(page, "formatVersion must be 1") || !strings.Contains(page, "apples-pack bad") {
		t.Fatalf("failed row missing: %s", page)
	}
	if strings.Contains(page, `name="library" value="x"`) {
		t.Fatal("failed library was enableable")
	}
}

func TestLoaderRejectsInvalidFiles(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		file string
		raw  string
		want string
	}{
		{name: "unreadable JSON", file: "trash.json", raw: `{not json`, want: "unreadable JSON"},
		{name: "missing library id", file: "noid.json", raw: `{"formatVersion":1,"id":"","name":"X","packs":[]}`, want: "missing library id"},
		{name: "pick less than 1", file: "pick.json", raw: `{"formatVersion":1,"id":"p","name":"P","packs":[{"id":"p","name":"P","prompts":[{"id":"p1","text":"Hi","pick":0}],"answers":[{"id":"a1","text":"A"}]}]}`, want: "pick must be at least 1"},
		{name: "duplicate pack ids", file: "dup-pack.json", raw: `{"formatVersion":1,"id":"d","name":"D","packs":[{"id":"same","name":"A","prompts":[{"id":"p1","text":"A"}],"answers":[{"id":"a1","text":"A"}]},{"id":"same","name":"B","prompts":[{"id":"p2","text":"B"}],"answers":[{"id":"a2","text":"B"}]}]}`, want: "duplicate pack id"},
		{name: "duplicate card ids", file: "dup-card.json", raw: `{"formatVersion":1,"id":"c","name":"C","packs":[{"id":"c","name":"C","prompts":[{"id":"same","text":"A"}],"answers":[{"id":"same","text":"B"}]}]}`, want: "duplicate card id"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			h := newFakeHelper(dir, true)
			g := loadedGame(t, h)
			if err := os.WriteFile(filepath.Join(dir, tt.file), []byte(tt.raw), 0o600); err != nil {
				t.Fatal(err)
			}
			page := picker(t, g)
			if !strings.Contains(page, tt.file) || !strings.Contains(page, tt.want) || !strings.Contains(page, "apples-pack bad") {
				t.Fatalf("want %q in gray row: %s", tt.want, page)
			}
		})
	}
}

func TestDuplicateLibraryIDsAreBothRejected(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	h := newFakeHelper(dir, true)
	g := loadedGame(t, h)
	dup := `{
  "formatVersion": 1,
  "id": "oranges",
  "name": "Clone",
  "packs": [{"id": "clone", "name": "Clone", "prompts": [{"id": "c-p", "text": "A"}], "answers": [{"id": "c-a", "text": "B"}]}]
}`
	if err := os.WriteFile(filepath.Join(dir, "clone.json"), []byte(dup), 0o600); err != nil {
		t.Fatal(err)
	}
	page := picker(t, g)
	if !strings.Contains(page, "oranges.json") || !strings.Contains(page, "clone.json") {
		t.Fatalf("both files should fail: %s", page)
	}
	if strings.Contains(page, `name="library" value="oranges"`) {
		t.Fatal("clashing oranges still enableable")
	}
}

func TestScanSkipsCahScratchDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	h := newFakeHelper(dir, true)
	g := loadedGame(t, h)
	if err := os.MkdirAll(filepath.Join(dir, "cah"), 0o700); err != nil {
		t.Fatal(err)
	}
	hidden := `{
  "formatVersion": 1,
  "id": "hidden",
  "name": "Hidden",
  "packs": [{"id": "hidden", "name": "Hidden", "prompts": [{"id": "x-p", "text": "A"}], "answers": [{"id": "x-a", "text": "B"}]}]
}`
	if err := os.WriteFile(filepath.Join(dir, "cah", "hidden.json"), []byte(hidden), 0o600); err != nil {
		t.Fatal(err)
	}
	page := picker(t, g)
	if strings.Contains(page, "Hidden") || strings.Contains(page, `value="hidden"`) {
		t.Fatalf("scanned cah/: %s", page)
	}
}

func TestPickerRequiresAdmin(t *testing.T) {
	t.Parallel()
	h := newFakeHelper(t.TempDir(), false)
	g := loadedGame(t, h)
	rec := httptest.NewRecorder()
	g.Play().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/picker", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestTogglePackPersistsUntilStarted(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	h := newFakeHelper(dir, true)
	g := loadedGame(t, h)

	rec := postSettings(g, "/pack", url.Values{
		"library": {"wildcard"},
		"pack":    {"wildcard"},
		"enabled": {"0", "1"},
	})
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/play/picker" {
		t.Fatalf("toggle = %d %q", rec.Code, rec.Header().Get("Location"))
	}
	page := picker(t, g)
	assertCheckbox(t, page, "wildcard", "wildcard", true)

	h.sit("p1", "Pat")
	if err := g.Start(h); err != nil {
		t.Fatal(err)
	}
	frozen := postSettings(g, "/pack", url.Values{
		"library": {"wildcard"},
		"pack":    {"wildcard"},
		"enabled": {"0"},
	})
	if frozen.Code != http.StatusOK {
		t.Fatalf("started toggle status = %d", frozen.Code)
	}
	body := frozen.Body.String()
	if !strings.Contains(body, "Pack changes wait until the game ends") {
		t.Fatalf("missing freeze note: %s", body)
	}
	row := packArticle(body, "wildcard", "wildcard")
	if !strings.Contains(row, "Pack changes wait until the game ends") {
		t.Fatalf("freeze error not on wildcard row: %s", row)
	}
	assertCheckbox(t, picker(t, g), "wildcard", "wildcard", true)

	if err := g.Pause(); err != nil {
		t.Fatal(err)
	}
	paused := postSettings(g, "/select-none", nil)
	if paused.Code != http.StatusOK {
		t.Fatalf("pause still writable: %d", paused.Code)
	}

	if err := g.Stop(); err != nil {
		t.Fatal(err)
	}
	off := postSettings(g, "/pack", url.Values{
		"library": {"wildcard"},
		"pack":    {"wildcard"},
		"enabled": {"0"},
	})
	if off.Code != http.StatusSeeOther {
		t.Fatalf("after stop = %d", off.Code)
	}
	assertCheckbox(t, picker(t, g), "wildcard", "wildcard", false)
}

func TestSelectAllAndNone(t *testing.T) {
	t.Parallel()
	h := newFakeHelper(t.TempDir(), true)
	g := loadedGame(t, h)
	all := postSettings(g, "/select-all", nil)
	if all.Code != http.StatusSeeOther {
		t.Fatalf("select-all = %d", all.Code)
	}
	page := picker(t, g)
	assertCheckbox(t, page, "wildcard", "wildcard", true)
	assertCheckbox(t, page, "oranges", "oranges", true)

	none := postSettings(g, "/select-none", nil)
	if none.Code != http.StatusSeeOther {
		t.Fatalf("select-none = %d", none.Code)
	}
	page = picker(t, g)
	assertCheckbox(t, page, "wildcard", "wildcard", false)
	assertCheckbox(t, page, "oranges", "oranges", false)
}

func TestCorruptMatchSettingsReloadsFactory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	h := newFakeHelper(dir, true)
	g := loadedGame(t, h)
	h.kv[kvMatchSettings] = []byte("{nope")
	page := picker(t, g)
	assertCheckbox(t, page, "oranges", "oranges", true)
	assertCheckbox(t, page, "wildcard", "wildcard", false)
}

func TestOfficialImportDownloadAndConvertErrors(t *testing.T) {
	t.Parallel()
	t.Run("download", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "nope", http.StatusBadGateway)
		}))
		t.Cleanup(server.Close)
		dir := t.TempDir()
		h := newFakeHelper(dir, true)
		g := newGameWithURL(t, h, server.URL)
		rec := postSettings(g, "/import-official", nil)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Could not download official packs") {
			t.Fatalf("download fail = %d %s", rec.Code, rec.Body.String())
		}
		if _, err := os.Stat(filepath.Join(dir, "cah", "json-against-humanity.json")); !os.IsNotExist(err) {
			t.Fatal("wrote dump after download failure")
		}
		if _, err := os.Stat(filepath.Join(dir, "official.json")); !os.IsNotExist(err) {
			t.Fatal("wrote official.json after download failure")
		}
	})
	t.Run("convert", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, "not-json")
		}))
		t.Cleanup(server.Close)
		dir := t.TempDir()
		h := newFakeHelper(dir, true)
		g := newGameWithURL(t, h, server.URL)
		rec := postSettings(g, "/import-official", nil)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Could not convert official packs") {
			t.Fatalf("convert fail = %d %s", rec.Code, rec.Body.String())
		}
		if _, err := os.Stat(filepath.Join(dir, "cah", "json-against-humanity.json")); err != nil {
			t.Fatal("expected raw dump after convert failure")
		}
		if _, err := os.Stat(filepath.Join(dir, "official.json")); !os.IsNotExist(err) {
			t.Fatal("wrote official.json after convert failure")
		}
	})
}

func TestOfficialImportConvertsFixture(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("testdata/cah-full.json")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(raw)
	}))
	t.Cleanup(server.Close)
	dir := t.TempDir()
	h := newFakeHelper(dir, true)
	g := newGameWithURL(t, h, server.URL)
	rec := postSettings(g, "/import-official", nil)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("import = %d %s", rec.Code, rec.Body.String())
	}

	out, err := os.ReadFile(filepath.Join(dir, "official.json"))
	if err != nil {
		t.Fatal(err)
	}
	var lib struct {
		ID    string `json:"id"`
		Packs []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"packs"`
	}
	if err := json.Unmarshal(out, &lib); err != nil {
		t.Fatal(err)
	}
	if lib.ID != "official" {
		t.Fatalf("id = %s", lib.ID)
	}
	names := map[string]bool{}
	for _, p := range lib.Packs {
		names[p.Name] = true
	}
	if !names["The Base Set"] || !names["Family Edition"] {
		t.Fatalf("missing kept packs: %#v", names)
	}
	for _, banned := range []string{"A.I. Pack", "Procedurally-Generated Cards", "PAX East 2014", "Retail Exclusive", "Conversion Kit", "House Fan Pack"} {
		if names[banned] {
			t.Fatalf("kept omitted pack %s", banned)
		}
	}

	page := picker(t, g)
	assertCheckbox(t, page, "official", "the-base-set", false)
	assertCheckbox(t, page, "official", "family-edition", false)
	if !strings.Contains(page, "creativecommons.org/licenses/by-nc-sa/4.0/legalcode") {
		t.Fatal("missing license strip")
	}

	on := postSettings(g, "/pack", url.Values{
		"library": {"official"},
		"pack":    {"the-base-set"},
		"enabled": {"1"},
	})
	if on.Code != http.StatusSeeOther {
		t.Fatal(on.Body.String())
	}

	again := postSettings(g, "/import-official", nil)
	if again.Code != http.StatusSeeOther {
		t.Fatalf("reimport = %d %s", again.Code, again.Body.String())
	}
	assertCheckbox(t, picker(t, g), "official", "the-base-set", true)
	assertCheckbox(t, picker(t, g), "official", "family-edition", false)
}

func loadedGame(t *testing.T, h *fakeHelper) *Game {
	t.Helper()
	g := New()
	if err := g.Load(h); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = g.Shutdown() })
	return g
}

func newGameWithURL(t *testing.T, h *fakeHelper, fetchURL string) *Game {
	t.Helper()
	g := New()
	g.fetchURL = fetchURL
	if err := g.Load(h); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = g.Shutdown() })
	return g
}

func picker(t *testing.T, g *Game) string {
	t.Helper()
	rec := httptest.NewRecorder()
	g.Play().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/picker", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("picker status = %d %s", rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

func postSettings(g *Game, path string, vals url.Values) *httptest.ResponseRecorder {
	var body io.Reader
	if vals != nil {
		body = strings.NewReader(vals.Encode())
	}
	req := httptest.NewRequest(http.MethodPost, path, body)
	if vals != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	rec := httptest.NewRecorder()
	g.Settings().ServeHTTP(rec, req)
	return rec
}

func assertCheckbox(t *testing.T, page, libraryID, packID string, on bool) {
	t.Helper()
	needle := `name="library" value="` + libraryID + `"`
	if !strings.Contains(page, needle) {
		t.Fatalf("missing library %s in %s", libraryID, page)
	}
	block := packForm(page, libraryID, packID)
	if block == "" {
		t.Fatalf("missing pack form %s/%s in %s", libraryID, packID, page)
	}
	checked := strings.Contains(block, "checked")
	if checked != on {
		t.Fatalf("%s/%s checked=%v want %v in %s", libraryID, packID, checked, on, block)
	}
}

func packForm(page, libraryID, packID string) string {
	return packSlice(page, libraryID, packID, "<form", "</form>")
}

func packArticle(page, libraryID, packID string) string {
	return packSlice(page, libraryID, packID, "<article", "</article>")
}

func packSlice(page, libraryID, packID, open, close string) string {
	start := strings.Index(page, `name="library" value="`+libraryID+`"`)
	for start >= 0 {
		formStart := strings.LastIndex(page[:start], open)
		formEnd := strings.Index(page[start:], close)
		if formStart < 0 || formEnd < 0 {
			return ""
		}
		block := page[formStart : start+formEnd]
		if strings.Contains(block, `name="pack" value="`+packID+`"`) {
			return block
		}
		next := strings.Index(page[start+1:], `name="library" value="`+libraryID+`"`)
		if next < 0 {
			return ""
		}
		start = start + 1 + next
	}
	return ""
}

type fakeHelper struct {
	dir       string
	admin     bool
	kv        map[string][]byte
	seated    []games.Player
	waiting   []games.Player
	audience  []games.Player
	players   map[string]games.Player
	published []string
	finishN   int
	pauseN    int
	logs      []string
}

func newFakeHelper(dir string, admin bool) *fakeHelper {
	return &fakeHelper{dir: dir, admin: admin, kv: map[string][]byte{}, players: map[string]games.Player{}}
}

func (h *fakeHelper) sit(id, name string) {
	p := games.Player{ID: id, DisplayName: name, Seated: true}
	h.seated = append(h.seated, p)
	h.players[id] = p
}

func (h *fakeHelper) sitHost(id, name string) {
	p := games.Player{ID: id, DisplayName: name, Seated: true, ClaimedHost: true}
	h.seated = append(h.seated, p)
	h.players[id] = p
}

func (h *fakeHelper) Seated() []games.Player  { return append([]games.Player(nil), h.seated...) }
func (h *fakeHelper) Waiting() []games.Player { return append([]games.Player(nil), h.waiting...) }
func (h *fakeHelper) Audience() []games.Player {
	return append([]games.Player(nil), h.audience...)
}
func (h *fakeHelper) Player(id string) (games.Player, bool) {
	p, ok := h.players[id]
	return p, ok
}
func (h *fakeHelper) PlayerFromRequest(r *http.Request) (games.Player, bool, error) {
	id := r.Header.Get("X-Player")
	if id == "" {
		return games.Player{}, false, nil
	}
	p, ok := h.players[id]
	return p, ok, nil
}
func (h *fakeHelper) DataDir() string { return h.dir }
func (h *fakeHelper) KVGet(key string) ([]byte, bool, error) {
	v, ok := h.kv[key]
	return v, ok, nil
}
func (h *fakeHelper) KVSet(key string, value []byte) error {
	h.kv[key] = append([]byte(nil), value...)
	return nil
}
func (h *fakeHelper) Finish() { h.finishN++ }
func (h *fakeHelper) Pause()  { h.pauseN++ }
func (h *fakeHelper) Resume() {}
func (h *fakeHelper) Publish(name string) {
	h.published = append(h.published, name)
}
func (h *fakeHelper) Log(line string) {
	h.logs = append(h.logs, line)
}
func (h *fakeHelper) Theme() string               { return ui.DefaultTheme }
func (h *fakeHelper) HasAdmin(*http.Request) bool { return h.admin }

var _ games.Helper = (*fakeHelper)(nil)
