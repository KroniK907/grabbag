package quips

import "testing"

func TestPromptDemandGM089(t *testing.T) {
	t.Parallel()
	s := factorySettings()
	s.RoundCount = 3
	s.LastQuipEnabled = true
	if got := promptDemand(s, 4); got != 9 {
		t.Fatalf("last quip demand = %d want 9", got)
	}
	s.LastQuipEnabled = false
	if got := promptDemand(s, 4); got != 12 {
		t.Fatalf("standard demand = %d want 12", got)
	}
	s.RoundCount = 1
	s.LastQuipEnabled = true
	if got := promptDemand(s, 5); got != 1 {
		t.Fatalf("last-quip-only = %d want 1", got)
	}
}

func TestStartShortageMessage(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cat := scanDataDir(dir)
	s := factorySettings()
	s.RoundCount = 3
	if msg := startShortage(cat, s, 4); msg != startRefuseMsg {
		t.Fatalf("empty dir shortage = %q", msg)
	}
}
