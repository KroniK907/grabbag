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
	"time"

	"github.com/KroniK907/grabbag/internal/games"
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
	if !strings.Contains(draw.Body.String(), "Skip") || !strings.Contains(draw.Body.String(), ">Lock in<") {
		t.Fatalf("judge missing hold controls: %s", draw.Body.String())
	}
	if keep := playPOST(g, "/keep-prompt", judge, nil); keep.Code != http.StatusOK {
		t.Fatal(keep.Body.String())
	}
	board := httptest.NewRecorder()
	g.Board(board, httptest.NewRequest(http.MethodGet, "/board", nil))
	if got := strings.Count(board.Body.String(), `class="apples-slot down"`); got != 2 {
		t.Fatalf("submit board has %d face-down packets, want 2: %s", got, board.Body.String())
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
	judgePhone := httptest.NewRecorder()
	judgeRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	judgeRequest.Header.Set("X-Player", judge)
	g.Phone(judgePhone, judgeRequest)
	if strings.Contains(judgePhone.Body.String(), ">Reveal next<") || !strings.Contains(judgePhone.Body.String(), "Choose the round winner.") {
		t.Fatalf("final reveal controls = %s", judgePhone.Body.String())
	}
	winner := firstPacket(g)
	conf := playPOST(g, "/confirm", judge, url.Values{"winner": {winner}})
	if conf.Code != http.StatusOK {
		t.Fatal(conf.Body.String())
	}
	if g.match.Actors[winner].Score != 100 {
		t.Fatalf("score = %d", g.match.Actors[winner].Score)
	}
	nextJudge := currentJudge(g)
	if nextJudge == judge {
		t.Fatalf("judge did not rotate from %s", judge)
	}
	nextPhone := httptest.NewRecorder()
	nextRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	nextRequest.Header.Set("X-Player", nextJudge)
	g.Phone(nextPhone, nextRequest)
	if !strings.Contains(nextPhone.Body.String(), ">Draw<") {
		t.Fatalf("incoming judge %s missing Draw: %s", nextJudge, nextPhone.Body.String())
	}
}

func TestBoardRosterMarksLocked(t *testing.T) {
	t.Parallel()
	h, g := tinyGame(t, 6, 40, 1)
	postSettings(g, "/hand-size", url.Values{"hand_size": {"3"}})
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
	board := httptest.NewRecorder()
	g.Board(board, httptest.NewRequest(http.MethodGet, "/board", nil))
	if strings.Contains(board.Body.String(), `apples-locked-mark`) {
		t.Fatalf("lock mark before submit: %s", board.Body.String())
	}
	playPOST(g, "/keep-prompt", judge, nil)
	board = httptest.NewRecorder()
	g.Board(board, httptest.NewRequest(http.MethodGet, "/board", nil))
	body := board.Body.String()
	otherName := g.match.Actors[other].Name
	judgeName := g.match.Actors[judge].Name
	if !playerTileLocked(body, "Bot 1") {
		t.Fatalf("bot missing lock mark: %s", body)
	}
	if playerTileLocked(body, otherName) {
		t.Fatalf("picker already marked locked: %s", playerTile(body, otherName))
	}
	if playerTileLocked(body, judgeName) {
		t.Fatalf("judge marked locked: %s", playerTile(body, judgeName))
	}
	playPOST(g, "/slot", other, url.Values{"card": {firstHandCard(g, other)}})
	playPOST(g, "/lock", other, nil)
	board = httptest.NewRecorder()
	g.Board(board, httptest.NewRequest(http.MethodGet, "/board", nil))
	if strings.Contains(board.Body.String(), `apples-locked-mark`) {
		t.Fatalf("lock marks lingered after reveal: %s", board.Body.String())
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

func TestSlotSwapsLastHoleWhenFull(t *testing.T) {
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
	c3 := g.match.Actors[other].Hand[2].CardID
	playPOST(g, "/slot", other, url.Values{"card": {c1}})
	playPOST(g, "/slot", other, url.Values{"card": {c2}})
	swap := playPOST(g, "/slot", other, url.Values{"card": {c3}})
	if strings.Contains(swap.Body.String(), "Every hole is filled.") {
		t.Fatalf("full slot rejected: %s", swap.Body.String())
	}
	holes := g.match.Actors[other].Holes
	if holes[0] == nil || holes[0].CardID != c1 || holes[1] == nil || holes[1].CardID != c3 {
		t.Fatalf("swap last hole = %#v", holes)
	}
	hand := g.match.Actors[other].Hand
	foundOld := false
	for _, c := range hand {
		if c.CardID == c2 {
			foundOld = true
		}
		if c.CardID == c3 {
			t.Fatalf("new card still in hand: %#v", hand)
		}
	}
	if !foundOld {
		t.Fatalf("displaced card missing from hand: %#v", hand)
	}
}

func TestSlotReplacesSingleHoleWhenFull(t *testing.T) {
	t.Parallel()
	h, g := tinyGame(t, 6, 40, 1)
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
	c1 := g.match.Actors[other].Hand[0].CardID
	c2 := g.match.Actors[other].Hand[1].CardID
	playPOST(g, "/slot", other, url.Values{"card": {c1}})
	playPOST(g, "/slot", other, url.Values{"card": {c2}})
	hole := g.match.Actors[other].Holes
	if len(hole) != 1 || hole[0] == nil || hole[0].CardID != c2 {
		t.Fatalf("single swap = %#v", hole)
	}
	foundOld := false
	for _, c := range g.match.Actors[other].Hand {
		if c.CardID == c1 {
			foundOld = true
		}
	}
	if !foundOld {
		t.Fatalf("replaced card missing from hand: %#v", g.match.Actors[other].Hand)
	}
}

func TestWildcardDoneSlotsAndHidesBox(t *testing.T) {
	t.Parallel()
	g, other := startWildcardSubmit(t)
	rec := playPOST(g, "/wildcard-draft", other, url.Values{"text": {"A rubber chicken"}})
	if rec.Code != http.StatusOK {
		t.Fatalf("done = %d %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, `class="apples-wildcard"`) {
		t.Fatal("wildcard box still on the phone after Done")
	}
	if !strings.Contains(body, "A rubber chicken") {
		t.Fatalf("slotted text missing: %s", body)
	}
	g.mu.Lock()
	a := g.match.Actors[other]
	if a.Blank != nil {
		t.Fatal("blank still in the hand after Done")
	}
	if len(a.Holes) == 0 || a.Holes[0] == nil || a.Holes[0].Text != "A rubber chicken" {
		t.Fatalf("holes = %#v", a.Holes)
	}
	g.mu.Unlock()
}

func TestWildcardDiscardOnlyOneLeftover(t *testing.T) {
	t.Parallel()
	g, other := startWildcardSubmit(t)
	playPOST(g, "/wildcard-draft", other, url.Values{"text": {"A rubber chicken"}})
	g.mu.Lock()
	first := g.match.Actors[other].Hand[0].CardID
	second := g.match.Actors[other].Hand[1].CardID
	g.mu.Unlock()
	slotted := playPOST(g, "/discard", other, url.Values{"card": {first}})
	if strings.Count(slotted.Body.String(), ">Discard</button>") != 0 {
		t.Fatalf("leftover discard chips still on the hand: %s", slotted.Body.String())
	}
	if !strings.Contains(slotted.Body.String(), ">Undo</button>") {
		t.Fatalf("missing undo on discarded card: %s", slotted.Body.String())
	}
	again := playPOST(g, "/discard", other, url.Values{"card": {second}})
	if !strings.Contains(again.Body.String(), "already discarded") {
		t.Fatalf("second discard = %s", again.Body.String())
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	a := g.match.Actors[other]
	if a.Discard == nil || a.Discard.CardID != first {
		t.Fatalf("discard = %#v", a.Discard)
	}
	for _, c := range a.Hand {
		if c.CardID == first {
			t.Fatal("discarded card returned to hand")
		}
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
	if g.match.Phase != phaseHold {
		t.Fatalf("phase after draw = %s", g.match.Phase)
	}
	first := g.match.LivePrompt.CardID
	playPOST(g, "/skip", judge, nil)
	if g.match.LivePrompt.CardID == first {
		t.Fatal("skip kept the same prompt")
	}
	if g.match.Phase != phaseHold {
		t.Fatalf("phase after skip = %s", g.match.Phase)
	}
	if len(g.match.Prompts) != left-2 {
		t.Fatalf("skip returned prompt to pile: have %d started %d", len(g.match.Prompts), left)
	}
	playPOST(g, "/keep-prompt", judge, nil)
	if g.match.Phase != phaseSubmit {
		t.Fatalf("phase after lock in = %s", g.match.Phase)
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
	judge := currentJudge(g)
	playPOST(g, "/draw", judge, nil)
	for _, a := range g.match.Actors {
		if a.Bot && filledCount(a.Holes) != 0 {
			t.Fatalf("bot dealt before lock in: %#v", a)
		}
	}
	playPOST(g, "/keep-prompt", judge, nil)
	for _, a := range g.match.Actors {
		if !a.Bot {
			continue
		}
		if filledCount(a.Holes) != 1 || !a.Locked || len(a.Hand) != 0 {
			t.Fatalf("bot after lock in: %#v", a)
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

func TestScoringViewLabelsWinnerAndFavoriteVotes(t *testing.T) {
	t.Parallel()
	h := newFakeHelper(t.TempDir(), true)
	g := loadedGame(t, h)
	g.mu.Lock()
	g.started = true
	g.match = &matchState{
		Settings:   factorySettings(),
		Phase:      phaseReveal,
		Actors:     map[string]*actor{"p1": {ID: "p1", Name: "Pat"}, "p2": {ID: "p2", Name: "Sam"}},
		Packets:    []packet{{ActorID: "p1", Cards: []playCard{{Text: "One"}}, Revealed: true}, {ActorID: "p2", Cards: []playCard{{Text: "Two"}}, Revealed: true}},
		Votes:      map[string]string{"v1": "p1", "v2": "p1"},
		WinnerID:   "p1",
		NamesShown: true,
		PhoneErr:   map[string]string{},
	}
	g.mu.Unlock()

	rec := httptest.NewRecorder()
	g.Board(rec, httptest.NewRequest(http.MethodGet, "/board", nil))
	body := rec.Body.String()
	for _, want := range []string{"Round winner - Pat", "2 favorite votes", "Favorite 1st", "Judge's pick"} {
		if !strings.Contains(body, want) {
			t.Fatalf("board missing %q: %s", want, body)
		}
	}
}

func TestMatchWinnerStaysVisibleBeforeHostFinish(t *testing.T) {
	t.Parallel()
	h := newFakeHelper(t.TempDir(), true)
	g := loadedGame(t, h)
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	g.now = func() time.Time { return now }
	m := &matchState{
		Settings: matchSettings{FinishHoldSec: 8},
		Actors:   map[string]*actor{"p1": {ID: "p1", Name: "Pat"}},
		WinnerID: "p1",
		PhoneErr: map[string]string{},
	}
	g.mu.Lock()
	g.started = true
	g.match = m
	g.finishLocked(h, m)
	g.mu.Unlock()

	rec := httptest.NewRecorder()
	g.Board(rec, httptest.NewRequest(http.MethodGet, "/board", nil))
	if body := rec.Body.String(); !strings.Contains(body, ">Winner<") || !strings.Contains(body, "Back to lobby") || !strings.Contains(body, "0:08") {
		t.Fatalf("winner hold page = %s", body)
	}
	if h.finishN != 0 {
		t.Fatal("host finished before the winner hold elapsed")
	}

	now = now.Add(8*time.Second + time.Second)
	g.mu.Lock()
	g.fireTimerLocked()
	g.flushHostLocked()
	g.mu.Unlock()
	if h.finishN != 1 {
		t.Fatalf("Finish calls = %d, want 1", h.finishN)
	}
}

func TestSeatedPlayerCanVoteDuringReveal(t *testing.T) {
	t.Parallel()
	h := newFakeHelper(t.TempDir(), true)
	h.sit("p1", "Pat")
	h.sit("p2", "Sam")
	g := loadedGame(t, h)
	g.mu.Lock()
	g.started = true
	g.match = &matchState{
		Settings: factorySettings(),
		Phase:    phaseReveal,
		JudgeID:  "p2",
		Actors: map[string]*actor{
			"p1": {ID: "p1", Name: "Pat", Locked: true, Holes: []*playCard{{Text: "Mine"}}},
			"p2": {ID: "p2", Name: "Sam"},
		},
		LivePrompt: &playPrompt{Text: "Prompt _"},
		Packets: []packet{
			{ActorID: "p1", Cards: []playCard{{Text: "Mine"}}, Revealed: true},
			{ActorID: "bot:1", Cards: []playCard{{Text: "Bot"}}, Revealed: true},
		},
		Votes:    map[string]string{},
		PhoneErr: map[string]string{},
	}
	g.mu.Unlock()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Player", "p1")
	g.Phone(rec, req)
	body := rec.Body.String()
	if !strings.Contains(body, `hx-post="/play/vote"`) || !strings.Contains(body, "Tap one answer to choose your favorite.") {
		t.Fatalf("seated reveal phone missing vote: %s", body)
	}
	if strings.Contains(body, ">Lock<") || strings.Contains(body, ">Locked<") {
		t.Fatalf("seated reveal phone still on lock: %s", body)
	}

	vote := playPOST(g, "/vote", "p1", url.Values{"target": {"bot:1"}})
	if vote.Code != http.StatusOK {
		t.Fatal(vote.Body.String())
	}
	if g.match.Votes["p1"] != "bot:1" {
		t.Fatalf("vote = %#v", g.match.Votes)
	}
	again := playPOST(g, "/vote", "p1", url.Values{"target": {"bot:1"}})
	if again.Code != http.StatusOK {
		t.Fatal(again.Body.String())
	}
	if _, ok := g.match.Votes["p1"]; ok {
		t.Fatalf("retap kept vote = %#v", g.match.Votes)
	}
}

func TestAudienceVotePhoneMarksChoiceAndScoring(t *testing.T) {
	t.Parallel()
	h := newFakeHelper(t.TempDir(), true)
	viewer := games.Player{ID: "viewer", DisplayName: "Viewer", Audience: true}
	h.players[viewer.ID] = viewer
	h.audience = append(h.audience, viewer)
	g := loadedGame(t, h)
	g.mu.Lock()
	g.started = true
	g.match = &matchState{
		Settings: factorySettings(),
		Phase:    phaseReveal,
		JudgeID:  "judge",
		Actors: map[string]*actor{
			"judge": {ID: "judge", Name: "Judge"},
			"p1":    {ID: "p1", Name: "Pat"},
			"p2":    {ID: "p2", Name: "Sam"},
		},
		LivePrompt: &playPrompt{Text: "Prompt _"},
		Packets: []packet{
			{ActorID: "p1", Cards: []playCard{{Text: "One"}}, Revealed: true},
			{ActorID: "p2", Cards: []playCard{{Text: "Two"}}, Revealed: true},
		},
		Votes:    map[string]string{"viewer": "p1"},
		PhoneErr: map[string]string{},
	}
	g.mu.Unlock()

	renderPhone := func() string {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Player", viewer.ID)
		g.Phone(rec, req)
		return rec.Body.String()
	}
	voting := renderPhone()
	if !strings.Contains(voting, "apples-vote-card") || !strings.Contains(voting, "Your favorite") || !strings.Contains(voting, "is-voted") {
		t.Fatalf("audience voting phone = %s", voting)
	}

	g.mu.Lock()
	g.match.NamesShown = true
	g.match.WinnerID = "p2"
	g.mu.Unlock()
	scoring := renderPhone()
	for _, want := range []string{"Judge's pick", "Favorite 1st", "Your favorite", "Sam"} {
		if !strings.Contains(scoring, want) {
			t.Fatalf("scoring phone missing %q: %s", want, scoring)
		}
	}
}

func TestSkipHoldHidesPromptUntilKeep(t *testing.T) {
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
	judge := currentJudge(g)
	other := "p1"
	if other == judge {
		other = "p2"
	}
	playPOST(g, "/draw", judge, nil)
	prompt := g.match.LivePrompt.Text

	otherPhone := httptest.NewRecorder()
	otherReq := httptest.NewRequest(http.MethodGet, "/", nil)
	otherReq.Header.Set("X-Player", other)
	g.Phone(otherPhone, otherReq)
	if body := otherPhone.Body.String(); strings.Contains(body, prompt) || strings.Contains(body, `hx-post="/play/slot"`) {
		t.Fatalf("player saw the prompt or could slot before lock in: %s", body)
	} else if !strings.Contains(body, `class="apples-prompt down"`) {
		t.Fatalf("player wait copy: %s", body)
	}

	board := httptest.NewRecorder()
	g.Board(board, httptest.NewRequest(http.MethodGet, "/board", nil))
	if body := board.Body.String(); strings.Contains(body, prompt) || !strings.Contains(body, "Judge, pick a prompt") {
		t.Fatalf("hold board = %s", body)
	}

	card := firstHandCard(g, other)
	slot := playPOST(g, "/slot", other, url.Values{"card": {card}})
	if !strings.Contains(slot.Body.String(), "You cannot play a card now.") {
		t.Fatalf("slot during hold = %s", slot.Body.String())
	}

	playPOST(g, "/keep-prompt", judge, nil)
	ready := httptest.NewRecorder()
	readyReq := httptest.NewRequest(http.MethodGet, "/", nil)
	readyReq.Header.Set("X-Player", other)
	g.Phone(ready, readyReq)
	if body := ready.Body.String(); !strings.Contains(body, prompt) || !strings.Contains(body, `hx-post="/play/slot"`) {
		t.Fatalf("player after lock in = %s", body)
	}
}

func TestTimerViewKeepsEndAcrossTicks(t *testing.T) {
	t.Parallel()
	h := newFakeHelper(t.TempDir(), true)
	g := loadedGame(t, h)
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	g.now = func() time.Time { return now }
	m := &matchState{PhoneErr: map[string]string{}}
	g.mu.Lock()
	g.started = true
	g.match = m
	g.armTimerLocked(m, "submit", 30)
	label1, text1, sec1, total1, end1 := g.timerViewLocked(m)
	now = now.Add(5 * time.Second)
	label2, text2, sec2, total2, end2 := g.timerViewLocked(m)
	g.mu.Unlock()
	if label1 != "Submit" || total1 != 30 || end1 == 0 {
		t.Fatalf("first timer view = %s %s %d %d %d", label1, text1, sec1, total1, end1)
	}
	if end2 != end1 || total2 != total1 || sec2 != 25 || text2 != "0:25" {
		t.Fatalf("second timer view = %s %s %d %d %d", label2, text2, sec2, total2, end2)
	}
}

func TestClaimedHostPhoneUsesBurnDrawer(t *testing.T) {
	t.Parallel()
	h, g := tinyGame(t, 4, 30, 1)
	postSettings(g, "/hand-size", url.Values{"hand_size": {"3"}})
	h.sitHost("host", "Host")
	if err := g.Start(h); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Player", "host")
	g.Phone(rec, req)
	body := rec.Body.String()
	for _, want := range []string{"apples-burn-handle", "apples-burn-drawer", ">Back<", "Nothing to burn yet"} {
		if !strings.Contains(body, want) {
			t.Fatalf("burn drawer missing %q: %s", want, body)
		}
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

func startWildcardSubmit(t *testing.T) (*Game, string) {
	t.Helper()
	h, g := tinyGame(t, 6, 40, 1)
	if rec := postSettings(g, "/pack", url.Values{"library": {"wildcard"}, "pack": {"wildcard"}, "enabled": {"1"}}); rec.Code != http.StatusSeeOther {
		t.Fatal(rec.Body.String())
	}
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
	g.mu.Lock()
	blank := g.match.Actors[other].Blank
	g.mu.Unlock()
	if blank == nil {
		t.Fatal("submitter has no wildcard blank")
	}
	return g, other
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

func playerTile(body, name string) string {
	needle := "<strong>" + name + "</strong>"
	i := strings.Index(body, needle)
	if i < 0 {
		return ""
	}
	start := strings.LastIndex(body[:i], `<div class="apples-player`)
	if start < 0 {
		return ""
	}
	end := strings.Index(body[i:], "</div>")
	if end < 0 {
		return body[start:]
	}
	return body[start : i+end+len("</div>")]
}

func playerTileLocked(body, name string) bool {
	tile := playerTile(body, name)
	return strings.Contains(tile, " locked") && strings.Contains(tile, "apples-locked-mark")
}

func settingsMode(g *Game) string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.match.Settings.PromptMode
}
