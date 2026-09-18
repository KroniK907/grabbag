package quips

import (
	"math/rand"
	"testing"
	"time"
)

func TestEngineMiniMatchAutoReveal(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 1, 18, 0, 0, 0, time.UTC)
	clock := now
	settings := factorySettings()
	settings.RoundCount = 1
	settings.LastQuipEnabled = false
	settings.WriteSec = 0
	settings.VoteSec = 1
	settings.WinnerScreenSec = 0
	settings.FinalScoresSec = 0
	settings.HostControlledReveals = false
	settings.SeatedVotePoints = 10
	settings.RoundMultiplierIncreaseBy = 1

	pool := make([]playPrompt, 4)
	for i := range pool {
		pool[i] = playPrompt{LibraryID: "quips", CardID: "c" + string(rune('a'+i)), Text: "prompt"}
	}
	rows := []rosterRow{{ID: "p1", Name: "One"}, {ID: "p2", Name: "Two"}}
	eng := &engine{}
	if err := eng.Begin(settings, rows, promptDeal{Pool: pool}, EngineHooks{}, rand.New(rand.NewSource(4)), clock); err != nil {
		t.Fatal(err)
	}
	lockPair(t, eng, "p1", []string{"a1", "a2"}, clock)
	lockPair(t, eng, "p2", []string{"b1", "b2"}, clock)
	if eng.Phase != phaseVote {
		t.Fatalf("phase=%s want vote", eng.Phase)
	}
	tick := clock.Add(5 * time.Second)
	for eng.Phase != phaseFinalScores && eng.Phase != phaseOver {
		eng.now = func() time.Time { return tick }
		switch eng.Phase {
		case phaseVote:
			eng.Do(Command{Kind: CmdVote, Actor: "p1", VoteTarget: "p2", VoterSeat: true}, tick)
			eng.Do(Command{Kind: CmdVote, Actor: "p2", VoteTarget: "p1", VoterSeat: true}, tick)
			tick = tick.Add(3 * time.Second)
			eng.now = func() time.Time { return tick }
			eng.Advance(tick)
		case phaseHold:
			tick = tick.Add(2 * time.Second)
			eng.now = func() time.Time { return tick }
			eng.Advance(tick)
		default:
			t.Fatalf("unexpected phase %s", eng.Phase)
		}
		if tick.Sub(clock) > 2*time.Minute {
			t.Fatal("match loop did not finish")
		}
	}
	if eng.Phase != phaseFinalScores && eng.Phase != phaseOver {
		t.Fatalf("after parade phase=%s scores=%v", eng.Phase, eng.Scores)
	}
	if eng.Scores["p1"] == 0 && eng.Scores["p2"] == 0 {
		t.Fatalf("expected points: %v", eng.Scores)
	}
	if eng.Phase == phaseFinalScores {
		tick = tick.Add(2 * time.Second)
		eng.now = func() time.Time { return tick }
		eng.Advance(tick)
	}
	if !eng.MatchOver {
		t.Fatal("match should finish after auto final scores")
	}
}

func TestEngineHostRevealGatesVote(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 1, 19, 0, 0, 0, time.UTC)
	settings := factorySettings()
	settings.WriteSec = 0
	settings.VoteSec = 30
	settings.WinnerScreenSec = 5
	settings.HostControlledReveals = true
	settings.RoundCount = 1
	settings.LastQuipEnabled = false

	pool := []playPrompt{{LibraryID: "quips", CardID: "x", Text: "p"}, {LibraryID: "quips", CardID: "y", Text: "p2"}}
	rows := []rosterRow{{ID: "p1", Name: "One"}, {ID: "p2", Name: "Two"}}
	eng := &engine{}
	if err := eng.Begin(settings, rows, promptDeal{Pool: pool}, EngineHooks{}, rand.New(rand.NewSource(1)), now); err != nil {
		t.Fatal(err)
	}
	lockPair(t, eng, "p1", []string{"q1", "q2"}, now)
	lockPair(t, eng, "p2", []string{"r1", "r2"}, now)
	if eng.Phase != phaseParadeWait {
		t.Fatalf("phase=%s", eng.Phase)
	}
	eng.Do(Command{Kind: CmdHostNextSegment, Actor: "host"}, now)
	if eng.Phase != phaseReveal {
		t.Fatalf("phase=%s", eng.Phase)
	}
	out := eng.Do(Command{Kind: CmdVote, Actor: "p1", VoteTarget: "p2", VoterSeat: true}, now)
	if out.PhoneErr["p1"] == "" {
		t.Fatal("vote should be blocked before reveals")
	}
	eng.Do(Command{Kind: CmdHostReveal, Actor: "host"}, now)
	eng.Do(Command{Kind: CmdHostReveal, Actor: "host"}, now)
	if eng.Phase != phaseVote {
		t.Fatalf("phase=%s want vote", eng.Phase)
	}
}

func lockPair(t *testing.T, eng *engine, id string, texts []string, now time.Time) {
	t.Helper()
	for i, text := range texts {
		eng.Do(Command{Kind: CmdDraft, Actor: id, Slot: i, Text: text}, now)
	}
	out := eng.Do(Command{Kind: CmdLock, Actor: id, Drafts: texts}, now)
	if out.PhoneErr[id] != "" {
		t.Fatalf("lock %s: %s", id, out.PhoneErr[id])
	}
}
