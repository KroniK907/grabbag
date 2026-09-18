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

func TestEngineVoteTimerArmsWhenVoteOpens(t *testing.T) {
	t.Parallel()
	begin := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	settings := factorySettings()
	settings.WriteSec = 0
	settings.VoteSec = 15
	settings.HostControlledReveals = false
	pool := make([]playPrompt, 4)
	for i := range pool {
		pool[i] = playPrompt{LibraryID: "quips", CardID: "p" + string(rune('a'+i)), Text: "prompt"}
	}
	rows := []rosterRow{{ID: "p1", Name: "One"}, {ID: "p2", Name: "Two"}}
	eng := &engine{}
	if err := eng.Begin(settings, rows, promptDeal{Pool: pool}, EngineHooks{}, rand.New(rand.NewSource(9)), begin); err != nil {
		t.Fatal(err)
	}
	voteOpen := begin.Add(20 * time.Second)
	lockPairAt(t, eng, "p1", []string{"a1", "a2"}, voteOpen)
	lockPairAt(t, eng, "p2", []string{"b1", "b2"}, voteOpen)
	if eng.Phase != phaseVote {
		t.Fatalf("phase=%s want vote", eng.Phase)
	}
	wantEnd := voteOpen.Add(15 * time.Second)
	if !eng.TimerEnd.Equal(wantEnd) {
		t.Fatalf("TimerEnd=%v want %v", eng.TimerEnd, wantEnd)
	}
	left := eng.TimerEnd.Sub(voteOpen)
	if left < 14*time.Second || left > 16*time.Second {
		t.Fatalf("remaining at vote open=%v", left)
	}
	out := eng.Advance(voteOpen.Add(14 * time.Second))
	if out.Changed {
		t.Fatal("vote should not close before duration")
	}
	out = eng.Advance(voteOpen.Add(16 * time.Second))
	if !out.Changed {
		t.Fatal("vote timer should close segment")
	}
	if eng.Phase == phaseVote && eng.TimerKind == timerVote {
		t.Fatal("vote timer should clear after expiry")
	}
}

func TestEngineComposeErrClearsOnSuccess(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC)
	settings := factorySettings()
	settings.WriteSec = 0
	settings.VoteSec = 5
	pool := make([]playPrompt, 4)
	for i := range pool {
		pool[i] = playPrompt{LibraryID: "quips", CardID: "x" + string(rune('a'+i)), Text: "prompt"}
	}
	rows := []rosterRow{{ID: "p1", Name: "One"}, {ID: "p2", Name: "Two"}}
	eng := &engine{}
	if err := eng.Begin(settings, rows, promptDeal{Pool: pool}, EngineHooks{}, rand.New(rand.NewSource(11)), now); err != nil {
		t.Fatal(err)
	}
	out := eng.Do(Command{Kind: CmdLock, Actor: "p1", Drafts: []string{"", ""}}, now)
	if out.PhoneErr["p1"] == "" {
		t.Fatal("empty lock should alert")
	}
	eng.PhoneErr["p1"] = out.PhoneErr["p1"]
	eng.Do(Command{Kind: CmdDraft, Actor: "p1", Slot: 0, Text: "ok"}, now)
	eng.Do(Command{Kind: CmdDraft, Actor: "p1", Slot: 1, Text: "ok2"}, now)
	out = eng.Do(Command{Kind: CmdLock, Actor: "p1", Drafts: []string{"ok", "ok2"}}, now)
	if out.PhoneErr["p1"] != "" {
		t.Fatalf("successful lock err=%q", out.PhoneErr["p1"])
	}
	if eng.PhoneErr["p1"] != "" {
		t.Fatalf("stored err=%q", eng.PhoneErr["p1"])
	}
	lockPairAt(t, eng, "p2", []string{"b1", "b2"}, now)
	if eng.PhoneErr["p1"] != "" {
		t.Fatalf("vote phase kept compose err=%q", eng.PhoneErr["p1"])
	}
}

func TestEngineBannedDraftKeptForLock(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)
	settings := factorySettings()
	settings.BannedWords = "banana"
	settings.ShowMatchedWord = true
	settings.WriteSec = 0
	pool := make([]playPrompt, 4)
	for i := range pool {
		pool[i] = playPrompt{LibraryID: "quips", CardID: "b" + string(rune('a'+i)), Text: "prompt"}
	}
	rows := []rosterRow{{ID: "p1", Name: "One"}, {ID: "p2", Name: "Two"}}
	eng := &engine{}
	if err := eng.Begin(settings, rows, promptDeal{Pool: pool}, EngineHooks{}, rand.New(rand.NewSource(12)), now); err != nil {
		t.Fatal(err)
	}
	out := eng.Do(Command{Kind: CmdDraft, Actor: "p1", Slot: 0, Text: "banana"}, now)
	if out.PhoneErr["p1"] != "Banned: banana" {
		t.Fatalf("draft err=%q", out.PhoneErr["p1"])
	}
	if eng.Writers["p1"].Slots[0].Draft != "banana" {
		t.Fatalf("draft=%q", eng.Writers["p1"].Slots[0].Draft)
	}
	eng.Do(Command{Kind: CmdDraft, Actor: "p1", Slot: 1, Text: "fine"}, now)
	out = eng.Do(Command{Kind: CmdLock, Actor: "p1", Drafts: []string{"banana", "fine"}}, now)
	if out.PhoneErr["p1"] != "Banned: banana" {
		t.Fatalf("lock err=%q", out.PhoneErr["p1"])
	}
}

func lockPairAt(t *testing.T, eng *engine, id string, texts []string, now time.Time) {
	t.Helper()
	for i, text := range texts {
		eng.Do(Command{Kind: CmdDraft, Actor: id, Slot: i, Text: text}, now)
	}
	out := eng.Do(Command{Kind: CmdLock, Actor: id, Drafts: texts}, now)
	if out.PhoneErr[id] != "" {
		t.Fatalf("lock %s: %s", id, out.PhoneErr[id])
	}
}
