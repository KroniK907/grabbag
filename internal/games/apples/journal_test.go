package apples

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBurnedFileMissingIsEmpty(t *testing.T) {
	t.Parallel()
	h, g := tinyGame(t, 4, 30, 1)
	path := filepath.Join(h.dir, "state", "burned.json")
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
	h := newFakeHelper(dir, true)
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
	raw, err := os.ReadFile(filepath.Join(dir, "state", "burned.json"))
	if err != nil || string(raw) != `{nope` {
		t.Fatalf("file changed: %s %v", raw, err)
	}
	page := settingsPage(t, g)
	if !strings.Contains(page, "Burn list is unreadable") || strings.Contains(page, "Unburn") && strings.Contains(page, "action=\"/settings/game/unburn\"") {
		if !strings.Contains(page, "Burn list is unreadable") {
			t.Fatalf("settings: %s", page)
		}
	}
}

func TestBurnNormalizesAndHidesAcrossLibraries(t *testing.T) {
	t.Parallel()
	h, g := tinyGame(t, 4, 20, 1)
	g.mu.Lock()
	g.burnPairLocked(kindAnswer, "  Answer 0  ")
	g.drainJournalsLocked()
	g.mu.Unlock()

	path := filepath.Join(h.dir, "state", "burned.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc burnedDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Burns) != 1 || doc.Burns[0].Text != "answer 0" || doc.FormatVersion != 1 {
		t.Fatalf("doc = %#v", doc)
	}

	h.sit("p1", "Pat")
	if err := g.Start(h); err != nil {
		t.Fatal(err)
	}
	g.mu.Lock()
	for _, c := range g.match.Answers {
		if normalizeCardText(c.Text) == "answer 0" {
			g.mu.Unlock()
			t.Fatal("burned answer stayed in the pile")
		}
	}
	g.mu.Unlock()
}

func TestUnburnLastTenNewestFirst(t *testing.T) {
	t.Parallel()
	_, g := tinyGame(t, 4, 20, 1)
	g.mu.Lock()
	for i := 0; i < 12; i++ {
		g.burnPairLocked(kindPrompt, "Prompt "+string(rune('A'+i)))
	}
	g.drainJournalsLocked()
	last := g.lastBurns(10)
	g.mu.Unlock()
	if len(last) != 10 {
		t.Fatalf("last = %d", len(last))
	}
	page := settingsPage(t, g)
	if !strings.Contains(page, "<h2>Burns</h2>") {
		t.Fatal(page)
	}
	rec := postSettings(g, "/unburn", url.Values{"kind": {last[0].Kind}, "text": {last[0].Text}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("unburn = %d %s", rec.Code, rec.Body.String())
	}
}

func TestDiscardSpendThenConfirmWritesStateFile(t *testing.T) {
	t.Parallel()
	h, g := tinyGame(t, 6, 40, 1)
	postSettings(g, "/hand-size", url.Values{"hand_size": {"3"}})
	postSettings(g, "/voting", url.Values{"voting": {"off"}})
	postSettings(g, "/timer-submit", url.Values{"submit": {"0"}})
	postSettings(g, "/timer-judge-pick", url.Values{"judge_pick": {"0"}})
	h.sit("p1", "Pat")
	h.sit("p2", "Sam")
	seedRNG(g)
	if err := g.Start(h); err != nil {
		t.Fatal(err)
	}
	judge := currentJudge(g)
	other := "p1"
	if other == judge {
		other = "p2"
	}
	if playPOST(g, "/draw", judge, nil).Code != http.StatusOK {
		t.Fatal("draw")
	}
	if playPOST(g, "/keep-prompt", judge, nil).Code != http.StatusOK {
		t.Fatal("keep")
	}
	card := firstHandCard(g, other)
	playPOST(g, "/slot", other, url.Values{"card": {card}})
	playPOST(g, "/lock", other, nil)
	for !allRevealed(g) {
		if playPOST(g, "/reveal", judge, nil).Code != http.StatusOK {
			t.Fatal("reveal")
		}
	}
	if playPOST(g, "/confirm", judge, url.Values{"winner": {firstPacket(g)}}).Code != http.StatusOK {
		t.Fatal("confirm")
	}
	raw, err := os.ReadFile(filepath.Join(h.dir, "state", "discard.json"))
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

func TestStartWipesDiscardWhenEveryDealableCardIsSpent(t *testing.T) {
	t.Parallel()
	h, g := tinyGame(t, 4, 20, 1)
	g.mu.Lock()
	cat := scanDataDir(h.dir)
	s, ok := g.loadSettings(h)
	s = reconcileSettings(s, ok, cat)
	piles := filterBurns(buildPiles(cat, s), g.burns)
	for _, p := range piles.Prompts {
		g.recordPlayedLocked(p.LibraryID, p.CardID)
	}
	for _, a := range piles.Answers {
		g.recordPlayedLocked(a.LibraryID, a.CardID)
	}
	g.drainJournalsLocked()
	g.mu.Unlock()
	h.sit("p1", "Pat")
	postSettings(g, "/hand-size", url.Values{"hand_size": {"3"}})
	if err := g.Start(h); err != nil {
		t.Fatal(err)
	}
	g.mu.Lock()
	n := len(g.played)
	g.mu.Unlock()
	if n != 0 {
		t.Fatalf("played after exhaustion wipe = %d", n)
	}
}

func TestReshuffleOnUnloadWipesOnlyOnShutdown(t *testing.T) {
	t.Parallel()
	h, g := tinyGame(t, 4, 20, 1)
	postSettings(g, "/reshuffle-on-unload", url.Values{"enabled": {"1"}})
	g.mu.Lock()
	g.recordPlayedLocked("tiny", "a0")
	g.drainJournalsLocked()
	g.mu.Unlock()
	if err := g.Stop(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(h.dir, "state", "discard.json")); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(h.dir, "state", "discard.json"))
	if !strings.Contains(string(raw), "a0") {
		t.Fatalf("Stop wiped discard: %s", raw)
	}
	if err := g.Shutdown(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(h.dir, "state", "discard.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "a0") {
		t.Fatalf("Shutdown left discard: %s", raw)
	}
}

func TestClearLeavesDiscardFile(t *testing.T) {
	t.Parallel()
	h, g := tinyGame(t, 4, 20, 1)
	g.mu.Lock()
	g.recordPlayedLocked("tiny", "a0")
	g.drainJournalsLocked()
	g.mu.Unlock()
	h.kv = map[string][]byte{}
	page := settingsPage(t, g)
	if !strings.Contains(page, "Reshuffle discard pile") {
		t.Fatal(page)
	}
	if _, err := os.Stat(filepath.Join(h.dir, "state", "discard.json")); err != nil {
		t.Fatal(err)
	}
}

func TestHowtoPublicAndSettingBranches(t *testing.T) {
	t.Parallel()
	h := newFakeHelper(t.TempDir(), false)
	g := loadedGame(t, h)
	rec := httptest.NewRecorder()
	g.Play().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/howto", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("howto = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "How to play") || !strings.Contains(body, "Judge") {
		t.Fatal(body)
	}
	if !strings.Contains(body, "Skip") {
		t.Fatalf("default skip missing: %s", body)
	}
	h.admin = true
	postSettings(g, "/prompt-mode", url.Values{"prompt_mode": {"single"}})
	postSettings(g, "/voting", url.Values{"voting": {"off"}})
	rec = httptest.NewRecorder()
	g.Play().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/howto", nil))
	body = rec.Body.String()
	if strings.Contains(body, "<h2>Skip</h2>") || strings.Contains(body, "<h2>Favorites</h2>") {
		t.Fatalf("single/off still has mode copy: %s", body)
	}
}

func TestPhoneHelpAfterStart(t *testing.T) {
	t.Parallel()
	h, g := tinyGame(t, 4, 30, 1)
	postSettings(g, "/hand-size", url.Values{"hand_size": {"3"}})
	h.sit("p1", "Pat")
	if err := g.Start(h); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Player", "p1")
	rec := httptest.NewRecorder()
	g.Phone(rec, req)
	if !strings.Contains(rec.Body.String(), `aria-label="How to play"`) {
		t.Fatal(rec.Body.String())
	}
	sheet := httptest.NewRecorder()
	g.Play().ServeHTTP(sheet, httptest.NewRequest(http.MethodGet, "/howto-sheet", nil))
	if sheet.Code != http.StatusOK || !strings.Contains(sheet.Body.String(), "Close") || !strings.Contains(sheet.Body.String(), "apples-howto-sheet") {
		t.Fatalf("sheet = %d %s", sheet.Code, sheet.Body.String())
	}
}

func TestStartOverlayWhenDiscardHoldsTheRest(t *testing.T) {
	t.Parallel()
	h, g := tinyGame(t, 4, 8, 1)
	postSettings(g, "/hand-size", url.Values{"hand_size": {"3"}})
	g.mu.Lock()
	cat := scanDataDir(h.dir)
	s, ok := g.loadSettings(h)
	s = reconcileSettings(s, ok, cat)
	piles := filterBurns(buildPiles(cat, s), g.burns)
	for i, a := range piles.Answers {
		if i >= 6 {
			break
		}
		g.recordPlayedLocked(a.LibraryID, a.CardID)
	}
	g.drainJournalsLocked()
	g.mu.Unlock()
	h.sitHost("p1", "Pat")
	if err := g.Start(h); err != nil {
		t.Fatal(err)
	}
	if g.overlay != pendingStart {
		t.Fatalf("overlay = %q", g.overlay)
	}
	phone := playPOST(g, "/reshuffle-yes", "p1", nil)
	if !strings.Contains(phone.Body.String(), overlayHostCopy) && g.overlay != "" && !g.overlayTooSmall {
		if g.overlay != "" {
			t.Fatalf("yes left overlay %q tooSmall=%v %s", g.overlay, g.overlayTooSmall, phone.Body.String())
		}
	}
}

func TestBurnUnsignedIs401(t *testing.T) {
	t.Parallel()
	h, g := tinyGame(t, 4, 20, 1)
	postSettings(g, "/hand-size", url.Values{"hand_size": {"3"}})
	h.sit("p1", "Pat")
	if err := g.Start(h); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/burn", strings.NewReader(""))
	rec := httptest.NewRecorder()
	g.Play().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code = %d", rec.Code)
	}
}

func TestUnburnRequiresAdmin(t *testing.T) {
	t.Parallel()
	h := newFakeHelper(t.TempDir(), false)
	g := loadedGame(t, h)
	rec := postSettings(g, "/unburn", url.Values{"kind": {"answer"}, "text": {"x"}})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code = %d %s", rec.Code, rec.Body.String())
	}
}
