package apples

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"time"

	"github.com/KroniK907/grabbag/internal/games"
)

func (g *Game) lockLocked(m *matchState, id string) error {
	a := m.Actors[id]
	if a == nil || !g.canSubmit(m, id) {
		return fmt.Errorf("You cannot lock now.")
	}
	pick := promptPick(m.LivePrompt)
	if filledCount(a.Holes) != pick {
		return fmt.Errorf("%d of %d", filledCount(a.Holes), pick)
	}
	a.Locked = true
	a.Discard = nil
	g.recordPacketLocked(m, a)
	g.maybeRevealLocked(m)
	return nil
}

func (g *Game) recordPacketLocked(m *matchState, a *actor) {
	for _, p := range m.Packets {
		if p.ActorID == a.ID {
			return
		}
	}
	m.Packets = append(m.Packets, packet{ActorID: a.ID, Cards: holeCards(a.Holes)})
}

func (g *Game) maybeRevealLocked(m *matchState) {
	if m.Phase != phaseSubmit {
		return
	}
	for _, a := range m.Actors {
		if a.ID == m.JudgeID && !(m.SuddenDeath && slices.Contains(m.TieIDs, a.ID)) {
			continue
		}
		if !a.Locked {
			return
		}
	}
	g.rngLocked().Shuffle(len(m.Packets), func(i, j int) { m.Packets[i], m.Packets[j] = m.Packets[j], m.Packets[i] })
	m.Phase = phaseReveal
	if m.SuddenDeath && m.JudgeID == "" {
		for i := range m.Packets {
			m.Packets[i].Revealed = true
		}
		g.autoPickLocked(m)
		return
	}
	g.armTimerLocked(m, "between-reveals", m.Settings.BetweenRevealSec)
}

func (g *Game) revealNextLocked(m *matchState) error {
	if m.Phase != phaseReveal {
		return fmt.Errorf("Nothing to reveal.")
	}
	for i := range m.Packets {
		if m.Packets[i].Revealed {
			continue
		}
		m.Packets[i].Revealed = true
		all := true
		for _, p := range m.Packets {
			if !p.Revealed {
				all = false
				break
			}
		}
		if all {
			g.armTimerLocked(m, "judge-pick", m.Settings.JudgePickSec)
		} else {
			g.armTimerLocked(m, "between-reveals", m.Settings.BetweenRevealSec)
		}
		return nil
	}
	return fmt.Errorf("Every answer is already up.")
}

func (g *Game) voteLocked(m *matchState, voterID, targetID string) error {
	if m.Phase != phaseReveal || m.Settings.Voting == voteOff {
		return fmt.Errorf("Voting is closed.")
	}
	anyUp := false
	for _, p := range m.Packets {
		if p.Revealed {
			anyUp = true
			break
		}
	}
	if !anyUp {
		return fmt.Errorf("Voting is closed.")
	}
	if voterID == m.JudgeID {
		return fmt.Errorf("The judge does not vote.")
	}
	if voterID == targetID {
		return fmt.Errorf("You cannot vote for yourself.")
	}
	found := false
	for _, p := range m.Packets {
		if p.ActorID == targetID && p.Revealed {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("That answer is still face down.")
	}
	m.Votes[voterID] = targetID
	return nil
}

func (g *Game) confirmLocked(h games.Helper, m *matchState, winnerID string) error {
	if m.Phase != phaseReveal {
		return fmt.Errorf("Confirm waits until every answer is up.")
	}
	for _, p := range m.Packets {
		if !p.Revealed {
			return fmt.Errorf("Confirm waits until every answer is up.")
		}
	}
	ok := false
	for _, p := range m.Packets {
		if p.ActorID == winnerID {
			ok = true
			break
		}
	}
	if !ok {
		return fmt.Errorf("That is not a revealed answer.")
	}
	mult := 1
	if m.InMultiplier {
		mult = m.Settings.LastRoundMultiplier
	}
	if a := m.Actors[winnerID]; a != nil {
		a.Score += m.Settings.WinnerPoints * mult
	}
	g.applyFavoritesLocked(m, mult)
	m.WinnerID = winnerID
	m.NamesShown = true
	g.saveWildcardsLocked(h, m, winnerID)
	g.clearTimerLocked(m)
	m.Round++
	return g.afterConfirmLocked(h, m)
}

func (g *Game) applyFavoritesLocked(m *matchState, mult int) {
	if m.Settings.Voting == voteOff {
		return
	}
	counts := map[string]int{}
	for _, target := range m.Votes {
		counts[target]++
	}
	if len(counts) == 0 {
		return
	}
	type row struct {
		id    string
		votes int
	}
	var rows []row
	for id, n := range counts {
		if n > 0 {
			rows = append(rows, row{id, n})
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].votes == rows[j].votes {
			return rows[i].id < rows[j].id
		}
		return rows[i].votes > rows[j].votes
	})
	places := []int{m.Settings.Fav1, m.Settings.Fav2, m.Settings.Fav3}
	i := 0
	place := 0
	for place < 3 && i < len(rows) {
		n := rows[i].votes
		var group []row
		for i < len(rows) && rows[i].votes == n {
			group = append(group, rows[i])
			i++
		}
		pts := places[place]
		for _, r := range group {
			if a := m.Actors[r.id]; a != nil {
				a.Score += pts * mult
			}
		}
		place += len(group)
	}
}

func (g *Game) afterConfirmLocked(h games.Helper, m *matchState) error {
	finishNow := false
	if m.Settings.WinByRounds {
		if m.Round >= m.Settings.RoundLimit {
			finishNow = true
		} else {
			m.InMultiplier = m.Round == m.Settings.RoundLimit-1 && m.Settings.LastRoundMultiplier > 1
		}
	} else if m.PendingFinish {
		finishNow = true
	} else {
		over := false
		for _, a := range m.Actors {
			if a.Score >= m.Settings.WinScore {
				over = true
				break
			}
		}
		if over {
			if m.Settings.LastRoundMultiplier <= 1 {
				finishNow = true
			} else {
				m.PendingFinish = true
				m.InMultiplier = true
			}
		}
	}
	if finishNow && g.leadTieLocked(m) {
		g.enterSuddenLocked(h, m)
		return nil
	}
	if finishNow {
		g.finishLocked(h, m)
		return nil
	}
	live := g.seatedLiveLocked(h)
	g.advanceJudgeLocked(m, live)
	m.Phase = phaseDrawWait
	g.armTimerLocked(m, "auto-draw", m.Settings.AutoDrawSec)
	return nil
}

func (g *Game) leadTieLocked(m *matchState) bool {
	top := -1
	n := 0
	for _, a := range m.Actors {
		if a.Score > top {
			top = a.Score
			n = 1
		} else if a.Score == top {
			n++
		}
	}
	return n > 1 && top >= 0
}

func (g *Game) enterSuddenLocked(h games.Helper, m *matchState) {
	top := -1
	for _, a := range m.Actors {
		if a.Score > top {
			top = a.Score
		}
	}
	var ties []string
	for _, a := range m.Actors {
		if a.Score == top {
			ties = append(ties, a.ID)
		}
	}
	m.TieIDs = ties
	m.SuddenDeath = true
	m.Phase = phaseSudden
	m.InMultiplier = true
	live := g.seatedLiveLocked(h)
	var pool []string
	for id := range live {
		if !slices.Contains(ties, id) {
			pool = append(pool, id)
		}
	}
	if len(pool) == 0 {
		m.JudgeID = ""
	} else {
		m.JudgeID = pool[g.rngLocked().Intn(len(pool))]
	}
}

func (g *Game) finishLocked(_ games.Helper, m *matchState) {
	m.Phase = phaseOver
	g.clearTimerLocked(m)
	g.needFinish = true
}

func (g *Game) autoPickLocked(m *matchState) {
	if len(m.Packets) == 0 {
		return
	}
	winner := m.Packets[g.rngLocked().Intn(len(m.Packets))].ActorID
	if m.Settings.Voting != voteOff {
		counts := map[string]int{}
		best := 0
		var leaders []string
		for _, t := range m.Votes {
			counts[t]++
			if counts[t] > best {
				best = counts[t]
				leaders = []string{t}
			} else if counts[t] == best && counts[t] > 0 && !slices.Contains(leaders, t) {
				leaders = append(leaders, t)
			}
		}
		if best > 0 && len(leaders) > 0 {
			winner = leaders[g.rngLocked().Intn(len(leaders))]
		}
	}
	_ = g.confirmLocked(g.helper, m, winner)
}

func (g *Game) wildcardCheckLocked(m *matchState, text string) error {
	return wildcardReject(text, m.Settings, g.wildcardTextsLocked())
}

func (g *Game) wildcardTextsLocked() []string {
	h := g.helper
	if h == nil {
		return nil
	}
	raw, err := os.ReadFile(filepath.Join(h.DataDir(), "wildcard.json"))
	if err != nil {
		return nil
	}
	lib, err := parseLibrary("wildcard.json", raw)
	if err != nil {
		return nil
	}
	var out []string
	for _, pack := range lib.Packs {
		for _, a := range pack.Answers {
			out = append(out, a.Text)
		}
	}
	return out
}

func (g *Game) saveWildcardsLocked(h games.Helper, m *matchState, winnerID string) {
	if !m.WildcardAtStart || h == nil {
		return
	}
	want := map[string]playCard{}
	favFirst := g.favoriteFirstPlaceLocked(m)
	for _, p := range m.Packets {
		for _, c := range p.Cards {
			if !c.Wildcard || c.Blank {
				continue
			}
			keep := m.Settings.WildcardSave == saveAll || p.ActorID == winnerID
			if !keep && m.Settings.WildcardSave == saveWinners {
				switch m.Settings.WildcardFavoritesKeep {
				case keepAll:
					keep = true
				case keepTop:
					keep = slices.Contains(favFirst, p.ActorID)
				}
			}
			if keep {
				want[c.CardID] = c
			}
		}
	}
	if len(want) == 0 {
		return
	}
	path := filepath.Join(h.DataDir(), "wildcard.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return
	}
	lib, err := parseLibrary("wildcard.json", raw)
	if err != nil {
		return
	}
	have := map[string]bool{}
	for _, pack := range lib.Packs {
		for _, a := range pack.Answers {
			have[normalizeCardText(a.Text)] = true
		}
	}
	if len(lib.Packs) == 0 {
		lib.Packs = []packFile{{ID: wildcardLibraryID, Name: "Wildcard"}}
	}
	for _, c := range want {
		norm := normalizeCardText(c.Text)
		if have[norm] {
			continue
		}
		have[norm] = true
		id := c.CardID
		if id == "" || id == blankCardID {
			id = wildcardID(c.Text)
		}
		lib.Packs[0].Answers = append(lib.Packs[0].Answers, answerCard{ID: id, Text: c.Text})
		if m.Settings.WildcardDealPrevious {
			m.Answers = append(m.Answers, playCard{LibraryID: wildcardLibraryID, CardID: id, Text: c.Text})
		}
	}
	encoded, err := json.MarshalIndent(lib, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, encoded, 0o600)
}

func (g *Game) favoriteFirstPlaceLocked(m *matchState) []string {
	counts := map[string]int{}
	best := 0
	for _, t := range m.Votes {
		counts[t]++
		if counts[t] > best {
			best = counts[t]
		}
	}
	if best == 0 {
		return nil
	}
	var ids []string
	for id, n := range counts {
		if n == best {
			ids = append(ids, id)
		}
	}
	return ids
}

func (g *Game) armTimerLocked(m *matchState, kind string, sec int) {
	if sec <= 0 {
		g.clearTimerLocked(m)
		return
	}
	if g.paused {
		m.TimerKind = kind
		m.FrozenLeft = time.Duration(sec) * time.Second
		m.TimerEnd = time.Time{}
		return
	}
	m.TimerKind = kind
	m.TimerEnd = g.clock().Add(time.Duration(sec) * time.Second)
	m.FrozenLeft = 0
}

func (g *Game) clearTimerLocked(m *matchState) {
	m.TimerKind = ""
	m.TimerEnd = time.Time{}
	m.FrozenLeft = 0
}

func (g *Game) freezeTimerLocked(m *matchState) {
	if m.TimerKind == "" || m.TimerEnd.IsZero() {
		return
	}
	left := m.TimerEnd.Sub(g.clock())
	if left < 0 {
		left = 0
	}
	m.FrozenLeft = left
	m.TimerEnd = time.Time{}
}

func (g *Game) thawTimerLocked(m *matchState) {
	if m.TimerKind == "" || m.FrozenLeft <= 0 {
		return
	}
	m.TimerEnd = g.clock().Add(m.FrozenLeft)
	m.FrozenLeft = 0
}

func (g *Game) fireTimerLocked() {
	if g.match == nil || g.paused || g.match.TimerKind == "" || g.match.TimerEnd.IsZero() {
		return
	}
	if g.clock().Before(g.match.TimerEnd) {
		return
	}
	m := g.match
	kind := m.TimerKind
	g.clearTimerLocked(m)
	switch kind {
	case "auto-draw":
		_ = g.drawLocked(m)
	case "submit":
		g.timerSubmitLocked(m)
	case "between-reveals":
		_ = g.revealNextLocked(m)
	case "judge-pick":
		g.autoPickLocked(m)
	}
	g.publishLocked()
	g.flushHostLocked()
}

func (g *Game) timerSubmitLocked(m *matchState) {
	pick := promptPick(m.LivePrompt)
	for _, a := range m.Actors {
		if !g.canSubmit(m, a.ID) {
			continue
		}
		for filledCount(a.Holes) < pick {
			idx := -1
			for i, c := range a.Hand {
				if !c.Wildcard && !c.Blank {
					idx = i
					break
				}
			}
			if idx < 0 {
				break
			}
			card := a.Hand[idx]
			a.Hand = slices.Delete(a.Hand, idx, idx+1)
			slot := firstEmptyHole(a.Holes)
			if slot < 0 {
				break
			}
			c := card
			a.Holes[slot] = &c
		}
		if filledCount(a.Holes) == pick {
			_ = g.lockLocked(m, a.ID)
		}
	}
}

func (g *Game) startTickerLocked() {
	g.stopTickerLocked()
	stop := make(chan struct{})
	g.stopTick = stop
	go func() {
		tick := time.NewTicker(200 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-stop:
				return
			case <-tick.C:
				g.mu.Lock()
				g.fireTimerLocked()
				g.mu.Unlock()
			}
		}
	}()
}

func (g *Game) stopTickerLocked() {
	if g.stopTick != nil {
		close(g.stopTick)
		g.stopTick = nil
	}
}

func (g *Game) setDraftLocked(m *matchState, id, text string) error {
	a := m.Actors[id]
	if a == nil || !g.canSubmit(m, id) {
		return fmt.Errorf("You cannot type a wildcard now.")
	}
	if len([]rune(text)) > m.Settings.WildcardCap && m.Settings.WildcardCap > 0 {
		text = string([]rune(text)[:m.Settings.WildcardCap])
	}
	a.Draft = text
	return nil
}

func remainingCap(draft string, cap int) int {
	if cap <= 0 {
		return 0
	}
	n := cap - len([]rune(draft))
	if n < 0 {
		return 0
	}
	return n
}

func (g *Game) playerMayVote(m *matchState, p games.Player) bool {
	if m == nil || m.Settings.Voting == voteOff || m.Phase != phaseReveal {
		return false
	}
	if p.ID == m.JudgeID {
		return false
	}
	switch m.Settings.Voting {
	case voteAudience:
		return p.Audience || p.Waiting
	case voteSeated:
		return p.Seated
	case voteBoth:
		return p.Seated || p.Audience || p.Waiting
	default:
		return false
	}
}
