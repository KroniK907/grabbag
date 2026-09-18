package quips

import (
	"math/rand"
	"testing"
	"time"
)

func TestEngineWriteGateAndTimer(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)
	clock := now
	eng := &engine{}
	settings := factorySettings()
	settings.WriteSec = 5
	pool := make([]playPrompt, 8)
	for i := range pool {
		pool[i] = playPrompt{LibraryID: "quips", CardID: "p" + string(rune('a'+i)), Text: "prompt"}
	}
	rows := []rosterRow{{ID: "p1", Name: "One"}, {ID: "p2", Name: "Two"}}
	if err := eng.Begin(settings, rows, promptDeal{Pool: pool}, EngineHooks{}, rand.New(rand.NewSource(2)), clock); err != nil {
		t.Fatal(err)
	}
	if eng.Phase != phaseWrite {
		t.Fatalf("phase=%s", eng.Phase)
	}
	out := eng.Do(Command{Kind: CmdDraft, Actor: "p1", Slot: 0, Text: "first"}, clock)
	if !out.Changed {
		t.Fatal("draft should change")
	}
	out = eng.Do(Command{Kind: CmdLock, Actor: "p1", Drafts: []string{"first", "second"}}, clock)
	if out.PhoneErr["p1"] != "" {
		t.Fatalf("lock p1: %s", out.PhoneErr["p1"])
	}
	for _, ev := range out.Events {
		if ev == eventQuipsLock {
			goto sawLock
		}
	}
	t.Fatal("missing quips-lock event")
sawLock:
	eng.Do(Command{Kind: CmdDraft, Actor: "p2", Slot: 0, Text: "alpha"}, clock)
	eng.Do(Command{Kind: CmdDraft, Actor: "p2", Slot: 1, Text: "beta"}, clock)
	out = eng.Do(Command{Kind: CmdLock, Actor: "p2", Drafts: []string{"alpha", "beta"}}, clock)
	if eng.Phase != phaseVote {
		t.Fatalf("phase after all locks=%s", eng.Phase)
	}
	if len(out.Events) == 0 {
		t.Fatal("expected write-end publish event")
	}

	eng2 := &engine{}
	settings.WriteSec = 1
	if err := eng2.Begin(settings, rows, promptDeal{Pool: pool}, EngineHooks{}, rand.New(rand.NewSource(3)), clock); err != nil {
		t.Fatal(err)
	}
	eng2.Do(Command{Kind: CmdDraft, Actor: "p1", Slot: 0, Text: "x"}, clock)
	eng2.Do(Command{Kind: CmdDraft, Actor: "p1", Slot: 1, Text: "y"}, clock)
	future := clock.Add(2 * time.Second)
	eng2.now = func() time.Time { return future }
	out = eng2.Advance(future)
	if eng2.Phase != phaseVote {
		t.Fatalf("timer phase=%s", eng2.Phase)
	}
	if !out.Changed {
		t.Fatal("timer advance should change")
	}
}
