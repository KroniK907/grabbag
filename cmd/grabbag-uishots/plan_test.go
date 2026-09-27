package main

import (
	"testing"

	"github.com/KroniK907/grabbag/internal/host"
)

func TestPlanSetsAndFilters(t *testing.T) {
	t.Parallel()
	index := host.PreviewIndex{
		Scenarios: []host.PreviewScenario{
			{Package: "apples", Surface: "phone", Name: "submit-seated", Frame: "phone", Sample: true,
				Players: []int{2, 7, 12, 64}, DefaultPlayers: 7, RealisticPlayers: 12, Path: "/dev/ui/s/apples/phone/submit-seated"},
			{Package: "lobby", Surface: "join", Name: "new", Frame: "phone", Path: "/dev/ui/s/lobby/join/new"},
		},
		Devices: []host.PreviewDevice{
			{ID: "phone", Width: 390, Height: 844, Scale: 3, Mobile: true, UA: "ios"},
			{ID: "tab", Width: 800, Height: 1280, Scale: 2, Mobile: true, Tablet: true, UA: "android"},
		},
		TextScales: []float64{1, 2},
		Frames:     host.PreviewFrames,
	}
	cfg := config{sets: map[string]bool{"scenarios": true, "sweep": true, "devices": true}, themes: []string{"neon-light", "neon-dark"}, maxDPR: 2, textMode: "device"}
	jobs := plan(cfg, index)
	count := map[string]int{}
	for _, j := range jobs {
		count[j.Set]++
		if j.Scale > 2 {
			t.Errorf("%s scale %v above cap", j.File, j.Scale)
		}
	}
	// scenarios: 2 pages x 2 themes; sweep: 4 counts; devices: phone 2 texts + tablet 2 texts x 2 orientations.
	if count["scenarios"] != 4 || count["sweep"] != 4 || count["devices"] != 6 {
		t.Fatalf("counts = %v", count)
	}
	stress := 0
	for _, j := range jobs {
		if j.Stress {
			stress++
		}
	}
	if stress != 1 {
		t.Fatalf("stress shots = %d, want the 64 sweep", stress)
	}
	cfg.only = []string{"lobby/*/*"}
	if got := plan(cfg, index); len(got) != 2 {
		t.Fatalf("filtered = %d, want lobby scenarios in 2 themes", len(got))
	}
}
