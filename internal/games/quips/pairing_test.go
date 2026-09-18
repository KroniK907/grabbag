package quips

import (
	"math/rand"
	"testing"
)

func TestPairings2RegularDegrees(t *testing.T) {
	t.Parallel()
	ids := []string{"a", "b", "c", "d"}
	edges := pairings2Regular(ids, rand.New(rand.NewSource(1)))
	if len(edges) != len(ids) {
		t.Fatalf("edges=%d want %d", len(edges), len(ids))
	}
	deg := map[string]int{}
	opponents := map[string]map[string]int{}
	for _, e := range edges {
		deg[e[0]]++
		deg[e[1]]++
		if opponents[e[0]] == nil {
		 opponents[e[0]] = map[string]int{}
		}
		if opponents[e[1]] == nil {
		 opponents[e[1]] = map[string]int{}
		}
		opponents[e[0]][e[1]]++
		opponents[e[1]][e[0]]++
	}
	for _, id := range ids {
		if deg[id] != 2 {
			t.Fatalf("degree %s = %d want 2", id, deg[id])
		}
		for opp, n := range opponents[id] {
			if n > 1 {
				t.Fatalf("repeat opponent %s vs %s in same round", id, opp)
			}
		}
	}
}
