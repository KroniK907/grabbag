package quips

import (
	"net/http"
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
	if len(buttons) != 1 || buttons[0].Label != "Prompt Library" || buttons[0].Path != "/play/picker" || !buttons[0].HostOnly {
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

func TestStartRefusesWithoutEngine(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	h := newFakeHelper(dir)
	g := New()
	if err := g.Load(h); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = g.Shutdown() })
	if err := g.Start(h); err == nil {
		t.Fatal("Start should fail until engine is wired")
	}
}

type fakeHelper struct {
	dir    string
	kv     map[string][]byte
	admin  bool
	seated []games.Player
}

func newFakeHelper(dir string) *fakeHelper {
	return &fakeHelper{dir: dir, kv: map[string][]byte{}}
}

func (h *fakeHelper) Seated() []games.Player                  { return h.seated }
func (h *fakeHelper) Waiting() []games.Player                 { return nil }
func (h *fakeHelper) Audience() []games.Player                { return nil }
func (h *fakeHelper) Player(string) (games.Player, bool)      { return games.Player{}, false }
func (h *fakeHelper) PlayerFromRequest(*http.Request) (games.Player, bool, error) {
	return games.Player{}, false, nil
}
func (h *fakeHelper) DataDir() string                         { return h.dir }
func (h *fakeHelper) KVGet(key string) ([]byte, bool, error)  { v, ok := h.kv[key]; return v, ok, nil }
func (h *fakeHelper) KVSet(key string, value []byte) error    { h.kv[key] = value; return nil }
func (h *fakeHelper) Finish()                                 {}
func (h *fakeHelper) Pause()                                  {}
func (h *fakeHelper) Resume()                                 {}
func (h *fakeHelper) Publish(string)                          {}
func (h *fakeHelper) Notify(string, string, string, int)      {}
func (h *fakeHelper) Log(string)                              {}
func (h *fakeHelper) Theme() string                           { return "neon-light" }
func (h *fakeHelper) HasAdmin(*http.Request) bool             { return h.admin }
