package apples

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStartRefusesWhenLibraryTooSmall(t *testing.T) {
	t.Parallel()
	h, g := tinyGame(t, 1, 2, 1)
	h.sit("p1", "Pat")
	err := g.Start(h)
	if err == nil || !strings.Contains(err.Error(), startRefuseMsg) {
		t.Fatalf("Start = %v", err)
	}
	if g.matchFrozen() {
		t.Fatal("refused Start still froze the match")
	}
}

func TestStartRefusesPickOverHandSize(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	h := newFakeHelper(dir, true)
	g := loadedGame(t, h)
	disableShipped(t, g)
	writeLib(t, dir, "tiny.json", tinyJSON(1, 20, 4))
	postSettings(g, "/pack", url.Values{"library": {"tiny"}, "pack": {"tiny"}, "enabled": {"1"}})
	postSettings(g, "/hand-size", url.Values{"hand_size": {"3"}})
	h.sit("p1", "Pat")
	err := g.Start(h)
	if err == nil || !strings.Contains(err.Error(), startRefuseMsg) {
		t.Fatalf("Start = %v", err)
	}
}

func TestStartDealsHumansAndFillsBots(t *testing.T) {
	t.Parallel()
	h, g := tinyGame(t, 4, 30, 1)
	postSettings(g, "/hand-size", url.Values{"hand_size": {"3"}})
	h.sit("p1", "Pat")
	if err := g.Start(h); err != nil {
		t.Fatal(err)
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.match == nil || len(g.match.Actors["p1"].Hand) != 3 {
		t.Fatalf("human hand = %#v", g.match.Actors["p1"])
	}
	bots := 0
	for _, a := range g.match.Actors {
		if a.Bot {
			bots++
			if len(a.Hand) != 0 {
				t.Fatalf("bot standing hand: %#v", a)
			}
		}
	}
	if bots != 2 {
		t.Fatalf("bots = %d", bots)
	}
}

func TestSettingsDefaultsAndRangeRejects(t *testing.T) {
	t.Parallel()
	h := newFakeHelper(t.TempDir(), true)
	g := loadedGame(t, h)
	page := settingsPage(t, g)
	if !strings.Contains(page, `value="7"`) || !strings.Contains(page, `value="500"`) {
		t.Fatalf("defaults missing: %s", page)
	}
	if !strings.Contains(page, "Deck Library") || !strings.Contains(page, ">Skip<") {
		t.Fatalf("settings groups missing: %s", page)
	}
	bad := postSettings(g, "/hand-size", url.Values{"hand_size": {"2"}})
	if bad.Code != http.StatusOK || !strings.Contains(bad.Body.String(), "Hand size is 3 to 12") {
		t.Fatalf("range = %d %s", bad.Code, bad.Body.String())
	}
	ok := postSettings(g, "/hand-size", url.Values{"hand_size": {"4"}})
	if ok.Code != http.StatusSeeOther {
		t.Fatalf("save = %d %s", ok.Code, ok.Body.String())
	}
}

func TestSettingsFreezeAfterStart(t *testing.T) {
	t.Parallel()
	h, g := tinyGame(t, 4, 30, 1)
	postSettings(g, "/hand-size", url.Values{"hand_size": {"3"}})
	h.sit("p1", "Pat")
	if err := g.Start(h); err != nil {
		t.Fatal(err)
	}
	frozen := postSettings(g, "/hand-size", url.Values{"hand_size": {"5"}})
	if frozen.Code != http.StatusOK || !strings.Contains(frozen.Body.String(), "This match is running. Setup is locked.") {
		t.Fatalf("freeze = %d %s", frozen.Code, frozen.Body.String())
	}
	if err := g.Pause(); err != nil {
		t.Fatal(err)
	}
	paused := postSettings(g, "/bot-count", url.Values{"bot_count": {"4"}})
	if paused.Code != http.StatusOK {
		t.Fatalf("pause writable: %d", paused.Code)
	}
}

func TestRoundLockRevealConfirm(t *testing.T) {
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
	draw := playPOST(g, "/draw", judge, nil)
	if draw.Code != http.StatusOK {
		t.Fatal(draw.Body.String())
	}
	if !strings.Contains(draw.Body.String(), "Skip") && settingsMode(g) == modeSkip {
		t.Fatalf("judge missing skip: %s", draw.Body.String())
	}
	card := firstHandCard(g, other)
	slot := playPOST(g, "/slot", other, url.Values{"card": {card}})
	if slot.Code != http.StatusOK {
		t.Fatal(slot.Body.String())
	}
	lock := playPOST(g, "/lock", other, nil)
	if lock.Code != http.StatusOK {
		t.Fatal(lock.Body.String())
	}
	// Bots already locked; should be in reveal.
	if phaseOf(g) != phaseReveal {
		t.Fatalf("phase = %s", phaseOf(g))
	}
	unsigned := httptest.NewRequest(http.MethodPost, "/reveal", nil)
	rec := httptest.NewRecorder()
	g.Play().ServeHTTP(rec, unsigned)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unsigned = %d", rec.Code)
	}
	stranger := playPOST(g, "/reveal", other, nil)
	if !strings.Contains(stranger.Body.String(), "Only the judge can reveal") {
		t.Fatalf("gate = %s", stranger.Body.String())
	}
	for phaseOf(g) == phaseReveal && !allRevealed(g) {
		playPOST(g, "/reveal", judge, nil)
	}
	winner := firstPacket(g)
	conf := playPOST(g, "/confirm", judge, url.Values{"winner": {winner}})
	if conf.Code != http.StatusOK {
		t.Fatal(conf.Body.String())
	}
	if g.match.Actors[winner].Score != 100 {
		t.Fatalf("score = %d", g.match.Actors[winner].Score)
	}
}

func TestPickNHolesStayPut(t *testing.T) {
	t.Parallel()
	h, g := tinyGame(t, 4, 40, 2)
	postSettings(g, "/hand-size", url.Values{"hand_size": {"3"}})
	postSettings(g, "/prompt-mode", url.Values{"prompt_mode": {"single"}})
	postSettings(g, "/voting", url.Values{"voting": {"off"}})
	postSettings(g, "/timer-submit", url.Values{"submit": {"0"}})
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
	playPOST(g, "/draw", judge, nil)
	if promptPick(g.match.LivePrompt) != 2 {
		t.Fatalf("pick = %d", promptPick(g.match.LivePrompt))
	}
	c1 := g.match.Actors[other].Hand[0].CardID
	c2 := g.match.Actors[other].Hand[1].CardID
	playPOST(g, "/slot", other, url.Values{"card": {c1}})
	playPOST(g, "/slot", other, url.Values{"card": {c2}})
	if g.match.Actors[other].Holes[0] == nil || g.match.Actors[other].Holes[0].CardID != c1 {
		t.Fatalf("holes moved: %#v", g.match.Actors[other].Holes)
	}
	playPOST(g, "/unslot", other, url.Values{"hole": {"1"}})
	if g.match.Actors[other].Holes[0] == nil || g.match.Actors[other].Holes[0].CardID != c1 || g.match.Actors[other].Holes[1] != nil {
		t.Fatalf("unslot moved first card: %#v", g.match.Actors[other].Holes)
	}
}

func TestWildcardDuplicateAndEmpty(t *testing.T) {
	t.Parallel()
	if err := wildcardReject("   ", factorySettings(), nil); err == nil || err.Error() != "Type an answer" {
		t.Fatalf("empty = %v", err)
	}
	s := factorySettings()
	s.WildcardDuplicateBlock = true
	if err := wildcardReject("Three raccoons in a trench coat trying to buy a movie ticket", s, []string{"Three raccoons in a trench coat trying to buy a movie ticket"}); err == nil || err.Error() != "Wildcard already exists" {
		t.Fatalf("dup = %v", err)
	}
	s.WildcardBanned = "raccoon, trench coat"
	if err := wildcardReject("a trench coat", s, nil); err == nil {
		t.Fatal("expected banned phrase")
	}
}

func TestSkipSpendsPromptMultiReturnsOther(t *testing.T) {
	t.Parallel()
	h, g := tinyGame(t, 6, 40, 1)
	postSettings(g, "/hand-size", url.Values{"hand_size": {"3"}})
	postSettings(g, "/prompt-mode", url.Values{"prompt_mode": {modeSkip}})
	postSettings(g, "/voting", url.Values{"voting": {"off"}})
	postSettings(g, "/timer-submit", url.Values{"submit": {"0"}})
	h.sit("p1", "Pat")
	h.sit("p2", "Sam")
	seedRNG(g)
	if err := g.Start(h); err != nil {
		t.Fatal(err)
	}
	left := len(g.match.Prompts)
	judge := currentJudge(g)
	playPOST(g, "/draw", judge, nil)
	first := g.match.LivePrompt.CardID
	playPOST(g, "/skip", judge, nil)
	if g.match.LivePrompt.CardID == first {
		t.Fatal("skip kept the same prompt")
	}
	if len(g.match.Prompts) != left-2 {
		t.Fatalf("skip returned prompt to pile: have %d started %d", len(g.match.Prompts), left)
	}

	g2h, g2 := tinyGame(t, 6, 40, 1)
	postSettings(g2, "/hand-size", url.Values{"hand_size": {"3"}})
	postSettings(g2, "/prompt-mode", url.Values{"prompt_mode": {modeMulti}})
	postSettings(g2, "/voting", url.Values{"voting": {"off"}})
	g2h.sit("p1", "Pat")
	g2h.sit("p2", "Sam")
	seedRNG(g2)
	if err := g2.Start(g2h); err != nil {
		t.Fatal(err)
	}
	left = len(g2.match.Prompts)
	judge = currentJudge(g2)
	playPOST(g2, "/draw", judge, nil)
	if g2.match.Phase != phaseChoose {
		t.Fatalf("phase = %s", g2.match.Phase)
	}
	keep := g2.match.Choice[0].CardID
	drop := g2.match.Choice[1].CardID
	playPOST(g2, "/choose-prompt", judge, url.Values{"prompt": {keep}})
	found := false
	for _, p := range g2.match.Prompts {
		if p.CardID == drop {
			found = true
		}
	}
	if !found {
		t.Fatal("unpicked multi prompt was not returned")
	}
	if g2.match.LivePrompt.CardID != keep {
		t.Fatal("chose the wrong prompt")
	}
}

func TestBotsGetPickOnlyOnDraw(t *testing.T) {
	t.Parallel()
	h, g := tinyGame(t, 4, 30, 1)
	postSettings(g, "/hand-size", url.Values{"hand_size": {"3"}})
	postSettings(g, "/voting", url.Values{"voting": {"off"}})
	postSettings(g, "/timer-submit", url.Values{"submit": {"0"}})
	h.sit("p1", "Pat")
	seedRNG(g)
	if err := g.Start(h); err != nil {
		t.Fatal(err)
	}
	for _, a := range g.match.Actors {
		if a.Bot && filledCount(a.Holes) != 0 {
			t.Fatal("bot dealt before draw")
		}
	}
	playPOST(g, "/draw", currentJudge(g), nil)
	for _, a := range g.match.Actors {
		if !a.Bot {
			continue
		}
		if filledCount(a.Holes) != 1 || !a.Locked || len(a.Hand) != 0 {
			t.Fatalf("bot after draw: %#v", a)
		}
	}
}

func TestSuddenDeathWhenLeadTiedAtFinish(t *testing.T) {
	t.Parallel()
	h, g := tinyGame(t, 6, 40, 1)
	postSettings(g, "/hand-size", url.Values{"hand_size": {"3"}})
	postSettings(g, "/win-score", url.Values{"win_score": {"100"}})
	postSettings(g, "/winner-points", url.Values{"winner_points": {"0"}})
	postSettings(g, "/voting", url.Values{"voting": {"off"}})
	postSettings(g, "/timer-submit", url.Values{"submit": {"0"}})
	postSettings(g, "/timer-judge-pick", url.Values{"judge_pick": {"0"}})
	h.sit("p1", "Pat")
	h.sit("p2", "Sam")
	seedRNG(g)
	if err := g.Start(h); err != nil {
		t.Fatal(err)
	}
	g.match.Actors["p1"].Score = 100
	g.match.Actors["p2"].Score = 100
	g.match.PendingFinish = true
	g.match.Phase = phaseReveal
	g.match.Packets = []packet{
		{ActorID: "p1", Cards: []playCard{{Text: "a"}}, Revealed: true},
		{ActorID: "p2", Cards: []playCard{{Text: "b"}}, Revealed: true},
	}
	g.match.LivePrompt = &playPrompt{Text: "Hi", Pick: 1}
	_ = g.confirmLocked(h, g.match, "p1")
	if g.match.Phase != phaseSudden {
		t.Fatalf("phase = %s", g.match.Phase)
	}
}

func tinyGame(t *testing.T, prompts, answers, pick int) (*fakeHelper, *Game) {
	t.Helper()
	dir := t.TempDir()
	h := newFakeHelper(dir, true)
	g := loadedGame(t, h)
	disableShipped(t, g)
	writeLib(t, dir, "tiny.json", tinyJSON(prompts, answers, pick))
	rec := postSettings(g, "/pack", url.Values{"library": {"tiny"}, "pack": {"tiny"}, "enabled": {"1"}})
	if rec.Code != http.StatusSeeOther {
		t.Fatal(rec.Body.String())
	}
	return h, g
}

func disableShipped(t *testing.T, g *Game) {
	t.Helper()
	if rec := postSettings(g, "/select-none", nil); rec.Code != http.StatusSeeOther {
		t.Fatal(rec.Body.String())
	}
}

func writeLib(t *testing.T, dir, name, raw string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
}

func tinyJSON(prompts, answers, pick int) string {
	type pcard struct {
		ID   string `json:"id"`
		Text string `json:"text"`
		Pick int    `json:"pick,omitempty"`
	}
	type acard struct {
		ID   string `json:"id"`
		Text string `json:"text"`
	}
	var ps []pcard
	for i := 0; i < prompts; i++ {
		ps = append(ps, pcard{ID: fmt.Sprintf("p%d", i), Text: fmt.Sprintf("Prompt %d _", i), Pick: pick})
	}
	var as []acard
	for i := 0; i < answers; i++ {
		as = append(as, acard{ID: fmt.Sprintf("a%d", i), Text: fmt.Sprintf("Answer %d", i)})
	}
	body, _ := json.Marshal(map[string]any{
		"formatVersion": 1,
		"id":            "tiny",
		"name":          "Tiny",
		"packs": []map[string]any{{
			"id": "tiny", "name": "Tiny", "prompts": ps, "answers": as,
		}},
	})
	return string(body)
}

func settingsPage(t *testing.T, g *Game) string {
	t.Helper()
	rec := httptest.NewRecorder()
	g.Settings().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("settings = %d %s", rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

func playPOST(g *Game, path, player string, vals url.Values) *httptest.ResponseRecorder {
	var body *strings.Reader
	if vals != nil {
		body = strings.NewReader(vals.Encode())
	} else {
		body = strings.NewReader("")
	}
	req := httptest.NewRequest(http.MethodPost, path, body)
	req.Header.Set("X-Player", player)
	if vals != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	rec := httptest.NewRecorder()
	g.Play().ServeHTTP(rec, req)
	return rec
}

func seedRNG(g *Game) {
	g.mu.Lock()
	g.rng = rand.New(rand.NewSource(1))
	g.mu.Unlock()
}

func currentJudge(g *Game) string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.match.JudgeID
}

func phaseOf(g *Game) phase {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.match.Phase
}

func firstHandCard(g *Game, id string) string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.match.Actors[id].Hand[0].CardID
}

func allRevealed(g *Game) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, p := range g.match.Packets {
		if !p.Revealed {
			return false
		}
	}
	return len(g.match.Packets) > 0
}

func firstPacket(g *Game) string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.match.Packets[0].ActorID
}

func settingsMode(g *Game) string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.match.Settings.PromptMode
}
