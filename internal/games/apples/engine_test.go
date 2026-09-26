package apples

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

type fakeSupply struct {
	piles   enabledPiles
	recycle bool
}

func (s *fakeSupply) unplayed(matchSettings) enabledPiles { return s.piles }

func (s *fakeSupply) recycleCovers(matchSettings, map[string]bool, int, int) bool {
	return s.recycle
}

func (s *fakeSupply) wildcardTexts() []string { return nil }

func testPiles(prompts, answers int) enabledPiles {
	var p enabledPiles
	for i := 0; i < prompts; i++ {
		p.Prompts = append(p.Prompts, playPrompt{LibraryID: "t", CardID: fmt.Sprintf("p%d", i), Text: fmt.Sprintf("Prompt %d _", i), Pick: 1})
	}
	for i := 0; i < answers; i++ {
		p.Answers = append(p.Answers, playCard{LibraryID: "t", CardID: fmt.Sprintf("a%d", i), Text: fmt.Sprintf("Answer %d", i)})
	}
	return p
}

var engineNow = time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)

func beginEngine(t *testing.T, supply *fakeSupply, roster ...string) *engine {
	t.Helper()
	settings := factorySettings()
	settings.HandSize = 3
	settings.PromptMode = modeSingle
	settings.Voting = voteOff
	settings.AutoDrawSec = 10
	var rows []rosterRow
	for _, id := range roster {
		rows = append(rows, rosterRow{ID: id, Name: id})
	}
	e := &engine{}
	out := e.Begin(settings, rows, supply.piles, supply, rand.New(rand.NewSource(1)), engineNow)
	if out.Shortage != "" {
		t.Fatalf("Begin shortage = %s", out.Shortage)
	}
	return e
}

func nonJudge(e *engine) string {
	for _, id := range e.SeatOrder {
		if a := e.Actors[id]; !a.Bot && id != e.JudgeID {
			return id
		}
	}
	return ""
}

func TestEngineBeginDealsHandsAndFillsBots(t *testing.T) {
	t.Parallel()
	e := beginEngine(t, &fakeSupply{piles: testPiles(5, 40)}, "p1", "p2")
	bots := 0
	for _, a := range e.Actors {
		if a.Bot {
			bots++
			continue
		}
		if len(a.Hand) != 3 {
			t.Fatalf("%s hand = %d", a.ID, len(a.Hand))
		}
	}
	if bots != 1 {
		t.Fatalf("bots = %d, want 1", bots)
	}
	if e.Phase != phaseDrawWait || e.TimerKind != timerAutoDraw {
		t.Fatalf("phase %s timer %s", e.Phase, e.TimerKind)
	}
}

func TestEngineRoundReportsPlayedCardsAndFlush(t *testing.T) {
	t.Parallel()
	supply := &fakeSupply{piles: testPiles(5, 40)}
	e := beginEngine(t, supply, "p1", "p2")
	judge := e.JudgeID
	player := nonJudge(e)

	if out := e.Do(command{Kind: cmdDraw, Actor: player}, engineNow); out.Err == nil {
		t.Fatal("non-judge drew")
	}
	out := e.Do(command{Kind: cmdDraw, Actor: judge}, engineNow)
	if out.Err != nil || e.Phase != phaseSubmit || len(out.Played) != 1 {
		t.Fatalf("draw: err=%v phase=%s played=%v", out.Err, e.Phase, out.Played)
	}
	card := e.Actors[player].Hand[0].CardID
	if out := e.Do(command{Kind: cmdSlot, Actor: player, Card: card}, engineNow); out.Err != nil {
		t.Fatal(out.Err)
	}
	out = e.Do(command{Kind: cmdLock, Actor: player}, engineNow)
	if out.Err != nil || len(out.Played) != 1 || out.Played[0].CardID != card {
		t.Fatalf("lock: err=%v played=%v", out.Err, out.Played)
	}
	if e.Phase != phaseReveal {
		t.Fatalf("phase after lock = %s", e.Phase)
	}
	for !e.allRevealed() {
		if out := e.Do(command{Kind: cmdReveal, Actor: judge}, engineNow); out.Err != nil {
			t.Fatal(out.Err)
		}
	}
	out = e.Do(command{Kind: cmdConfirm, Actor: judge, Card: player}, engineNow)
	if out.Err != nil || !out.FlushDiscard {
		t.Fatalf("confirm: err=%v flush=%v", out.Err, out.FlushDiscard)
	}
	if e.Actors[player].Score != e.Settings.WinnerPoints || e.Phase != phaseDrawWait {
		t.Fatalf("score %d phase %s", e.Actors[player].Score, e.Phase)
	}
	if e.JudgeID == judge {
		t.Fatal("judge did not rotate")
	}
}

func TestEngineDrawRaisesShortageWhenRecycleCovers(t *testing.T) {
	t.Parallel()
	supply := &fakeSupply{piles: testPiles(1, 40)}
	e := beginEngine(t, supply, "p1", "p2")
	supply.piles.Prompts = nil
	supply.recycle = true
	out := e.Do(command{Kind: cmdDraw, Actor: e.JudgeID}, engineNow)
	if out.Err != nil || out.Shortage != pendingDraw {
		t.Fatalf("err=%v shortage=%q", out.Err, out.Shortage)
	}
	if e.Phase != phaseDrawWait {
		t.Fatalf("phase = %s", e.Phase)
	}
	supply.piles = testPiles(3, 40)
	out = e.Reshuffled(pendingDraw, engineNow)
	if out.TooSmall || e.Phase != phaseSubmit {
		t.Fatalf("reshuffle: tooSmall=%v phase=%s", out.TooSmall, e.Phase)
	}
}

func TestEngineAdvanceWaitsWhilePaused(t *testing.T) {
	t.Parallel()
	e := beginEngine(t, &fakeSupply{piles: testPiles(5, 40)}, "p1", "p2")
	later := engineNow.Add(time.Duration(e.Settings.AutoDrawSec+1) * time.Second)
	e.SetPaused(true, engineNow)
	if out := e.Advance(later); out.Publish || e.Phase != phaseDrawWait {
		t.Fatalf("paused advance fired: phase %s", e.Phase)
	}
	e.SetPaused(false, later)
	if out := e.Advance(later.Add(time.Duration(e.Settings.AutoDrawSec+1) * time.Second)); !out.Publish || e.Phase != phaseSubmit {
		t.Fatalf("auto-draw did not fire: phase %s", e.Phase)
	}
}

func TestEngineSyncRosterPausesWithNoHumans(t *testing.T) {
	t.Parallel()
	e := beginEngine(t, &fakeSupply{piles: testPiles(5, 40)}, "p1")
	if out := e.SyncRoster(nil, engineNow); !out.Pause {
		t.Fatal("empty roster did not ask for pause")
	}
}
