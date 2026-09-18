package quips

import "testing"

func TestScoringMultiplierRampAndClamp(t *testing.T) {
	t.Parallel()
	if scoringMultiplier(1, 1) != 1 {
		t.Fatal("round 1")
	}
	if scoringMultiplier(2, 1) != 2 {
		t.Fatal("round 2")
	}
	if scoringMultiplier(3, 1) != 3 {
		t.Fatal("round 3")
	}
	if scoringMultiplier(4, 1) != 3 {
		t.Fatal("round 4 clamp")
	}
	if scoringMultiplier(3, 0) != 1 {
		t.Fatal("increase 0")
	}
	if scoringMultiplier(3, 2) != 3 {
		t.Fatalf("inc 2 round 3 = %d", scoringMultiplier(3, 2))
	}
}
