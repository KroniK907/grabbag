package borrowedtruths

import "math/rand"

// tellsPerPlayer is the Auto rule: 2 at 4 to 7 seated, 1 at 8 and up.
func tellsPerPlayer(s matchSettings, seated int) int {
	if s.TellsPerPlayer > 0 {
		return s.TellsPerPlayer
	}
	if seated <= 7 {
		return 2
	}
	return 1
}

// tellOrder shuffles the seated players into passes. Nobody tells twice
// before everyone has told once, and a pass never starts with the player
// who ended the last one. Above 12 seated the match stops at MaxTellsLarge.
func tellOrder(s matchSettings, players []*player, rng *rand.Rand) []string {
	n := len(players)
	if n == 0 {
		return nil
	}
	total := n * tellsPerPlayer(s, n)
	if n > 12 {
		total = min(total, s.MaxTellsLarge)
	}
	var order []string
	for len(order) < total {
		pass := make([]string, n)
		for i, p := range players {
			pass[i] = p.ID
		}
		rng.Shuffle(n, func(i, j int) { pass[i], pass[j] = pass[j], pass[i] })
		if len(order) > 0 && n > 1 && pass[0] == order[len(order)-1] {
			pass[0], pass[n-1] = pass[n-1], pass[0]
		}
		order = append(order, pass...)
	}
	return order[:total]
}

// deal picks a card type by weight, then falls back through Lie, Borrowed,
// Yours. It returns nil when no type with a weight has a card left.
func (e *engine) deal(teller string) *card {
	s := e.Settings
	weights := map[cardKind]int{kindYours: s.MixYours, kindBorrowed: s.MixBorrowed, kindLie: s.MixLie}
	sum := s.MixYours + s.MixBorrowed + s.MixLie
	var first cardKind
	if sum > 0 {
		roll := e.rng.Intn(sum)
		for _, k := range []cardKind{kindYours, kindBorrowed, kindLie} {
			if roll < weights[k] {
				first = k
				break
			}
			roll -= weights[k]
		}
	}
	for _, k := range []cardKind{first, kindLie, kindBorrowed, kindYours} {
		if k == "" || weights[k] == 0 {
			continue
		}
		if i := e.pick(k, teller); i >= 0 {
			return &card{Fact: i, Kind: k}
		}
	}
	return nil
}

func (e *engine) open(i int) bool {
	return !e.Facts[i].Used && !e.Skipped[i]
}

func (e *engine) pick(k cardKind, teller string) int {
	switch k {
	case kindYours:
		return e.pickFrom(func(f fact) bool { return !f.Lie && f.Owner == teller })
	case kindBorrowed:
		return e.pickBorrowed(teller)
	case kindLie:
		if e.Settings.LieSource != lieSourceBank {
			if i := e.pickFrom(func(f fact) bool { return f.Lie && f.Owner != "" && f.Owner != teller }); i >= 0 {
				return i
			}
		}
		if e.Settings.LieSource != lieSourcePlayers {
			return e.pickFrom(func(f fact) bool { return f.Lie && f.Owner == "" })
		}
	}
	return -1
}

func (e *engine) pickFrom(ok func(fact) bool) int {
	var idx []int
	for i, f := range e.Facts {
		if e.open(i) && ok(f) {
			idx = append(idx, i)
		}
	}
	if len(idx) == 0 {
		return -1
	}
	return idx[e.rng.Intn(len(idx))]
}

// pickBorrowed takes a truth from the owners with the most unused truths, so
// everyone is borrowed from before anyone is borrowed from twice. The last
// borrowed owner is never picked again straight away.
func (e *engine) pickBorrowed(teller string) int {
	unused := map[string][]int{}
	for i, f := range e.Facts {
		if f.Lie || f.Owner == "" || f.Owner == teller || f.Owner == e.LastOwner || !e.open(i) {
			continue
		}
		unused[f.Owner] = append(unused[f.Owner], i)
	}
	best := 0
	var owners []string
	for _, p := range e.Players {
		switch n := len(unused[p.ID]); {
		case n == 0:
		case n > best:
			best = n
			owners = []string{p.ID}
		case n == best:
			owners = append(owners, p.ID)
		}
	}
	if len(owners) == 0 {
		return -1
	}
	idx := unused[owners[e.rng.Intn(len(owners))]]
	return idx[e.rng.Intn(len(idx))]
}
