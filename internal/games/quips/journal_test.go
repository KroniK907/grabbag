package quips

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KroniK907/grabbag/internal/games"
)

func loadedGame(t *testing.T, h *fakeHelper) *Game {
	t.Helper()
	g := New()
	if err := g.Load(h); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = g.Shutdown() })
	return g
}

func TestBurnedFileMissingIsEmpty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	g := loadedGame(t, newFakeHelper(dir))
	path := filepath.Join(dir, "state", "burned.json")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("missing burned.json should stay missing, err=%v", err)
	}
	page := settingsPage(t, g)
	if !strings.Contains(page, "Nothing to unburn") {
		t.Fatalf("empty burns: %s", page)
	}
}

func TestCorruptBurnedStaysAndLogs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	h := newFakeHelper(dir)
	if err := os.MkdirAll(filepath.Join(dir, "state"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "state", "burned.json"), []byte(`{nope`), 0o600); err != nil {
		t.Fatal(err)
	}
	g := loadedGame(t, h)
	if !g.burnCorrupt {
		t.Fatal("corrupt burned.json was treated as empty")
	}
	if len(h.logs) == 0 {
		t.Fatal("corrupt burned.json was not logged")
	}
	page := settingsPage(t, g)
	if !strings.Contains(page, "Burn list is unreadable") {
		t.Fatalf("settings: %s", page)
	}
}

func TestBurnNormalizesPromptOnly(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	g := loadedGame(t, newFakeHelper(dir))
	g.mu.Lock()
	g.burnPairLocked(kindPrompt, "  Hello  ")
	g.drainJournalsLocked()
	g.mu.Unlock()

	raw, err := os.ReadFile(filepath.Join(dir, "state", "burned.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc burnedDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Burns) != 1 || doc.Burns[0].Text != "hello" || doc.Burns[0].Kind != kindPrompt {
		t.Fatalf("doc = %#v", doc)
	}
}

func TestDiscardOnAssignWritesFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	h := newFakeHelper(dir)
	h.seated = []games.Player{
		{ID: "p1", DisplayName: "One", Seated: true},
		{ID: "p2", DisplayName: "Two", Seated: true},
	}
	g := loadedGame(t, h)
	if err := g.Start(h); err != nil {
		t.Fatal(err)
	}
	g.drainJournalsLocked()
	raw, err := os.ReadFile(filepath.Join(dir, "state", "discard.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc discardDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.FormatVersion != 1 || len(doc.Played) == 0 {
		t.Fatalf("discard = %#v", doc)
	}
}

func TestReshuffleOnUnloadWipesOnlyOnShutdown(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	h := newFakeHelper(dir)
	g := loadedGame(t, h)
	h.admin = true
	postSettings(g, "/reshuffle-on-unload", url.Values{"enabled": {"1"}})
	g.mu.Lock()
	g.recordPlayedLocked("quips", "p0")
	g.drainJournalsLocked()
	g.mu.Unlock()
	if err := g.Stop(); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "state", "discard.json"))
	if !strings.Contains(string(raw), "p0") {
		t.Fatalf("Stop wiped discard: %s", raw)
	}
	if err := g.Shutdown(); err != nil {
		t.Fatal(err)
	}
	if err := g.Load(h); err != nil {
		t.Fatal(err)
	}
	g.mu.Lock()
	if len(g.played) != 0 {
		t.Fatalf("Shutdown left discard in memory: %#v", g.played)
	}
	g.mu.Unlock()
}

func TestRollingBurnDrawerKeepsThree(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	g := loadedGame(t, newFakeHelper(dir))
	g.mu.Lock()
	for i := 0; i < 5; i++ {
		g.pushBurnDrawerLocked(playPrompt{Text: "Prompt " + string(rune('A'+i))})
	}
	g.mu.Unlock()
	if len(g.burnDrawer) != 3 {
		t.Fatalf("drawer = %d", len(g.burnDrawer))
	}
	if g.burnDrawer[0].Text != "Prompt C" {
		t.Fatalf("oldest kept = %q", g.burnDrawer[0].Text)
	}
}

func TestPartialReshuffleKeepsActiveSegmentPrompt(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	g := loadedGame(t, newFakeHelper(dir))
	eng := &engine{
		Segments: []roundSegment{
			{Prompt: playPrompt{LibraryID: "quips", CardID: "active", Text: "Active"}},
		},
	}
	g.mu.Lock()
	g.recordPlayedLocked("quips", "active")
	g.recordPlayedLocked("quips", "old")
	g.partialReshuffleDiscardLocked(eng)
	g.mu.Unlock()
	if len(g.played) != 1 {
		t.Fatalf("played = %#v", g.played)
	}
	if _, ok := g.played[playedKey("quips", "active")]; !ok {
		t.Fatal("active prompt was recycled")
	}
}
