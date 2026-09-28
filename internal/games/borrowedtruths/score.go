package borrowedtruths

import "sort"

// isLie reports whether calling Lie is right. A borrowed truth is a lie: the
// card is not the teller's.
func (c *card) isLie() bool { return c.Kind != kindYours }

// kindPick is the Lie pick that also names the right kind.
func (c *card) kindPick() string {
	if c.Kind == kindBorrowed {
		return pickBorrowed
	}
	return pickLie
}

func (e *engine) give(id string, points int, why string) {
	p := e.player(id)
	if p == nil || points == 0 {
		return
	}
	p.Score += points
	e.Awards = append(e.Awards, award{ID: id, Points: points, Why: why})
}

// scoreKnew pays each right I knew it pick 1 point. The insider's pick is
// recorded and ignored.
func (e *engine) scoreKnew() {
	in := e.insider()
	owner := e.Facts[e.Card.Fact].Owner
	for _, p := range e.voters() {
		k, ok := e.Knew[p.ID]
		if !ok || p.ID == in {
			continue
		}
		right := false
		switch e.Card.Kind {
		case kindYours:
			right = k.Pick == pickTrue
		case kindLie:
			right = k.Pick == pickLie
		case kindBorrowed:
			right = k.Pick == pickBorrowed && k.Owner == owner
		}
		if right {
			p.Calls++
			e.give(p.ID, 1, "Knew it")
		}
	}
}

// scoreVote pays the True or Lie vote, the teller's fooled points, and a
// player lie's author.
func (e *engine) scoreVote() {
	e.Awards = nil
	e.scoreKnew()
	teller := e.teller()
	in := e.insider()
	voted, lies := 0, 0
	for _, p := range e.countedVoters() {
		pick, ok := e.Votes[p.ID]
		if !ok {
			continue
		}
		voted++
		calledLie := pick != pickTrue
		if calledLie {
			lies++
		}
		if calledLie != e.Card.isLie() {
			teller.Fooled++
			e.give(teller.ID, 1, "Fooled "+p.Name)
			continue
		}
		p.Calls++
		points := 2
		if calledLie && pick == e.Card.kindPick() {
			points++
		}
		e.give(p.ID, points, "Right call")
	}
	if e.Card.Kind == kindLie && in != "" && voted > 0 && lies*2 < voted {
		e.player(in).Straight++
		e.give(in, 2, "Wrote the lie")
	}
}

// scoreOwnerVote pays voters who named the owner, and the owner if fewer than
// half of them did.
func (e *engine) scoreOwnerVote() {
	e.Awards = nil
	owner := e.insider()
	picked, named := 0, 0
	for _, p := range e.countedVoters() {
		target, ok := e.OwnerVotes[p.ID]
		if !ok {
			continue
		}
		picked++
		if target == owner {
			named++
			p.Calls++
			e.give(p.ID, 2, "Named the owner")
		}
	}
	if picked > 0 && named*2 < picked {
		e.player(owner).Straight++
		e.give(owner, 2, "Straight face")
	}
}

// standings is every seated player by score, then name.
func (e *engine) standings() []*player {
	out := append([]*player(nil), e.Players...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Name < out[j].Name
	})
	return out
}

type ribbon struct {
	Title   string
	Line    string
	Winners []*player
}

// ribbons are the end-of-match awards. A ribbon nobody earned is left out.
func (e *engine) ribbons() []ribbon {
	top := func(stat func(*player) int) []*player {
		best := 0
		var out []*player
		for _, p := range e.Players {
			switch v := stat(p); {
			case v == 0:
			case v > best:
				best, out = v, []*player{p}
			case v == best:
				out = append(out, p)
			}
		}
		return out
	}
	var out []ribbon
	for _, r := range []ribbon{
		{Title: "Straight Face", Line: "Most insider bonuses", Winners: top(func(p *player) int { return p.Straight })},
		{Title: "Silver Tongue", Line: "Most voters fooled", Winners: top(func(p *player) int { return p.Fooled })},
		{Title: "Lie Detector", Line: "Most correct calls", Winners: top(func(p *player) int { return p.Calls })},
	} {
		if len(r.Winners) > 0 {
			out = append(out, r)
		}
	}
	return out
}
