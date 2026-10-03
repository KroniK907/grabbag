package apples

import (
	"fmt"
	"slices"
	"sort"
)

func (e *engine) allRevealed() bool {
	for _, p := range e.Packets {
		if !p.Revealed {
			return false
		}
	}
	return true
}

func (e *engine) revealNext() error {
	if e.Phase != phaseReveal {
		return fmt.Errorf("Nothing to reveal.")
	}
	for i := range e.Packets {
		if e.Packets[i].Revealed {
			continue
		}
		e.Packets[i].Revealed = true
		if !e.allRevealed() {
			e.armTimer(timerBetween, e.Settings.BetweenRevealSec)
			return nil
		}
		if e.Settings.Voting != voteOff && e.Settings.FavoriteVoteSec > 0 {
			e.armTimer(timerFavoriteVote, e.Settings.FavoriteVoteSec)
		} else {
			e.armTimer(timerJudgePick, e.Settings.JudgePickSec)
		}
		if e.LivePrompt != nil {
			e.out.BurnDrawer = true
		}
		return nil
	}
	return fmt.Errorf("Every answer is already up.")
}

// vote toggles voterID's favorite on a face-up answer. The Game checks
// whether the voter's role may vote at all.
func (e *engine) vote(voterID, targetID string) error {
	if e.Phase != phaseReveal || e.Settings.Voting == voteOff {
		return fmt.Errorf("Voting is closed.")
	}
	if !slices.ContainsFunc(e.Packets, func(p packet) bool { return p.Revealed }) {
		return fmt.Errorf("Voting is closed.")
	}
	if voterID == e.JudgeID {
		return fmt.Errorf("The judge does not vote.")
	}
	if voterID == targetID {
		return fmt.Errorf("You cannot vote for yourself.")
	}
	if !slices.ContainsFunc(e.Packets, func(p packet) bool { return p.ActorID == targetID && p.Revealed }) {
		return fmt.Errorf("That answer is still face down.")
	}
	if e.Votes[voterID] == targetID {
		delete(e.Votes, voterID)
		return nil
	}
	e.Votes[voterID] = targetID
	return nil
}

// confirm scores the round for winnerID and moves to the next round, sudden
// death, or the end.
func (e *engine) confirm(winnerID string) error {
	if e.Phase != phaseReveal || !e.allRevealed() {
		return fmt.Errorf("Confirm waits until every answer is up.")
	}
	if e.Timer.Kind == timerFavoriteVote {
		return fmt.Errorf("Favorites are still open.")
	}
	if !slices.ContainsFunc(e.Packets, func(p packet) bool { return p.ActorID == winnerID }) {
		return fmt.Errorf("That is not a revealed answer.")
	}
	mult := 1
	if e.InMultiplier {
		mult = e.Settings.LastRoundMultiplier
	}
	if a := e.Actors[winnerID]; a != nil {
		a.Score += e.Settings.WinnerPoints * mult
	}
	e.applyFavorites(mult)
	e.WinnerID = winnerID
	e.NamesShown = true
	e.PhoneErr = map[string]string{}
	e.out.Wildcards = e.keptWildcards(winnerID)
	e.clearTimer()
	e.Round++
	e.out.FlushDiscard = true
	e.afterConfirm()
	return nil
}

// applyFavorites awards Fav1..Fav3 to the top vote-getters. Ties share a
// place and push the next place down.
func (e *engine) applyFavorites(mult int) {
	if e.Settings.Voting == voteOff {
		return
	}
	counts := map[string]int{}
	for _, target := range e.Votes {
		counts[target]++
	}
	type row struct {
		id    string
		votes int
	}
	var rows []row
	for id, n := range counts {
		rows = append(rows, row{id, n})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].votes == rows[j].votes {
			return rows[i].id < rows[j].id
		}
		return rows[i].votes > rows[j].votes
	})
	places := []int{e.Settings.Fav1, e.Settings.Fav2, e.Settings.Fav3}
	i := 0
	place := 0
	for place < 3 && i < len(rows) {
		n := rows[i].votes
		start := i
		for i < len(rows) && rows[i].votes == n {
			i++
		}
		for _, r := range rows[start:i] {
			if a := e.Actors[r.id]; a != nil {
				a.Score += places[place] * mult
			}
		}
		place += i - start
	}
}

// autoPick confirms for the judge: the most-voted answer, else a random one.
func (e *engine) autoPick() {
	if len(e.Packets) == 0 {
		return
	}
	winner := e.Packets[e.intn(len(e.Packets))].ActorID
	if e.Settings.Voting != voteOff {
		if leaders := e.voteLeaders(); len(leaders) > 0 {
			winner = leaders[e.intn(len(leaders))]
		}
	}
	_ = e.confirm(winner)
}

// voteLeaders is every answer tied for the most favorite votes, in first
// vote order.
func (e *engine) voteLeaders() []string {
	counts := map[string]int{}
	best := 0
	var leaders []string
	for _, t := range e.Votes {
		counts[t]++
		if counts[t] > best {
			best = counts[t]
			leaders = []string{t}
		} else if counts[t] == best && !slices.Contains(leaders, t) {
			leaders = append(leaders, t)
		}
	}
	return leaders
}

// keptWildcards is the round's wildcard answers that the save policy keeps.
func (e *engine) keptWildcards(winnerID string) []playCard {
	if !e.WildcardAtStart {
		return nil
	}
	favFirst := e.voteLeaders()
	var out []playCard
	seen := map[string]bool{}
	for _, p := range e.Packets {
		for _, c := range p.Cards {
			if !c.Wildcard || c.Blank || seen[c.CardID] {
				continue
			}
			keep := e.Settings.WildcardSave == saveAll || p.ActorID == winnerID
			if !keep && e.Settings.WildcardSave == saveWinners {
				switch e.Settings.WildcardFavoritesKeep {
				case keepAll:
					keep = true
				case keepTop:
					keep = slices.Contains(favFirst, p.ActorID)
				}
			}
			if keep {
				seen[c.CardID] = true
				out = append(out, c)
			}
		}
	}
	return out
}
