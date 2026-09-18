package quips

import "math/rand"

// pairings2Regular returns N edges for N seated players (GM-005). Each player
// appears in exactly two segments. For three or more players, opponents differ.
func pairings2Regular(ids []string, rng *rand.Rand) [][2]string {
	n := len(ids)
	if n < 2 {
		return nil
	}
	if n == 2 {
		return [][2]string{{ids[0], ids[1]}, {ids[0], ids[1]}}
	}
	order := append([]string(nil), ids...)
	rng.Shuffle(len(order), func(i, j int) { order[i], order[j] = order[j], order[i] })
	out := make([][2]string, n)
	for i := 0; i < n; i++ {
		out[i] = [2]string{order[i], order[(i+1)%n]}
	}
	return out
}
