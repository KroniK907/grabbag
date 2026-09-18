package quips

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KroniK907/grabbag/internal/games"
)

func TestCatalogRegistersQuips(t *testing.T) {
	t.Parallel()
	var got games.Factory
	for _, f := range games.Catalog() {
		if f.ID == id {
			got = f
		}
	}
	if got.New == nil {
		t.Fatal("quips is not in the catalog")
	}
	g := got.New()
	if g.ID() != "quips" || g.Name() != "Quick Quips" || g.MinPlayers() != 2 || g.MaxPlayers() != 0 {
		t.Fatalf("id=%s name=%s min=%d max=%d", g.ID(), g.Name(), g.MinPlayers(), g.MaxPlayers())
	}
	buttons := g.BoardButtons()
	if len(buttons) != 2 ||
		buttons[0].Label != "How to play" || buttons[0].Path != "/play/howto" || buttons[0].HostOnly ||
		buttons[1].Label != "Prompt Library" || buttons[1].Path != "/play/picker" || !buttons[1].HostOnly {
		t.Fatalf("BoardButtons = %#v", buttons)
	}
}

func TestLoadCopiesShippedQuipsOnce(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	h := newFakeHelper(dir)
	g := New()
	if err := g.Load(h); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = g.Shutdown() })

	path := filepath.Join(dir, "quips.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("missing quips.json: %v", err)
	}
	if !strings.Contains(string(raw), `"id": "quips"`) {
		t.Fatalf("unexpected quips.json: %s", raw)
	}

	if err := os.WriteFile(path, []byte(`{"keep":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := g.Load(h); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"keep":true}` {
		t.Fatalf("Load overwrote quips.json: %s", raw)
	}
}

func TestHowtoAndCompactViewportAssets(t *testing.T) {
	t.Parallel()
	g := New()
	if err := g.Load(newFakeHelper(t.TempDir())); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = g.Shutdown() })

	for _, tt := range []struct {
		path string
		want []string
	}{
		{path: "/howto", want: []string{"How to play", "Last Quip", "Back to game"}},
		{path: "/static/game.js", want: []string{"visualViewport", "--quips-visual-height", "quips-compact-height", "height < 760"}},
		{path: "/static/game.css", want: []string{".quips-lock-bar .ui-btn", "min-height: 44px", "var(--quips-visual-height, 100dvh)", "height: 100dvh"}},
	} {
		rec := httptest.NewRecorder()
		g.Play().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s = %d", tt.path, rec.Code)
		}
		for _, want := range tt.want {
			if !strings.Contains(rec.Body.String(), want) {
				t.Fatalf("%s missing %q", tt.path, want)
			}
		}
	}

	rec := httptest.NewRecorder()
	g.render(rec, "phone.html", phoneView{pageView: g.pageView("Quick Quips")}, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `class="quips-help"`) {
		t.Fatalf("phone missing help: %s", rec.Body.String())
	}
}

func TestStartOpensWritePhase(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	h := newFakeHelper(dir)
	h.seated = []games.Player{
		{ID: "p1", DisplayName: "One", Seated: true},
		{ID: "p2", DisplayName: "Two", Seated: true},
	}
	g := New()
	if err := g.Load(h); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = g.Shutdown() })
	if err := g.Start(h); err != nil {
		t.Fatalf("Start: %v", err)
	}
	g.mu.Lock()
	phase := g.engine.Phase
	g.mu.Unlock()
	if phase != phaseWrite {
		t.Fatalf("phase=%s want write", phase)
	}
}

type fakeHelper struct {
	dir    string
	kv     map[string][]byte
	admin  bool
	seated []games.Player
	logs   []string
}

func newFakeHelper(dir string) *fakeHelper {
	return &fakeHelper{dir: dir, kv: map[string][]byte{}}
}

func (h *fakeHelper) Seated() []games.Player             { return h.seated }
func (h *fakeHelper) Waiting() []games.Player            { return nil }
func (h *fakeHelper) Audience() []games.Player           { return nil }
func (h *fakeHelper) Player(string) (games.Player, bool) { return games.Player{}, false }
func (h *fakeHelper) PlayerFromRequest(*http.Request) (games.Player, bool, error) {
	return games.Player{}, false, nil
}
func (h *fakeHelper) DataDir() string                        { return h.dir }
func (h *fakeHelper) KVGet(key string) ([]byte, bool, error) { v, ok := h.kv[key]; return v, ok, nil }
func (h *fakeHelper) KVSet(key string, value []byte) error   { h.kv[key] = value; return nil }
func (h *fakeHelper) Finish()                                {}
func (h *fakeHelper) Pause()                                 {}
func (h *fakeHelper) Resume()                                {}
func (h *fakeHelper) Publish(string)                         {}
func (h *fakeHelper) Notify(string, string, string, int)     {}
func (h *fakeHelper) Log(line string)                        { h.logs = append(h.logs, line) }
func (h *fakeHelper) Theme() string                          { return "neon-light" }
func (h *fakeHelper) HasAdmin(*http.Request) bool            { return h.admin }
