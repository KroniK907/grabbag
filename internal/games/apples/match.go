package apples

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/KroniK907/grabbag/internal/games"
)

func (g *Game) beginMatchLocked(h games.Helper) error {
	cat := scanDataDir(h.DataDir())
	settings, ok := g.loadSettings(h)
	settings = reconcileSettings(settings, ok, cat)
	seated := h.Seated()
	if len(seated) < 1 {
		return fmt.Errorf("Start needs a seated player.")
	}
	if g.burnCorrupt {
		return fmt.Errorf("Burn list is unreadable")
	}
	if g.discardCorrupt {
		return fmt.Errorf("Discard list is unreadable")
	}
	noBurn := filterBurns(buildPiles(cat, settings), g.burns)
	if msg := startShortage(noBurn, settings, len(seated)); msg != "" {
		return fmt.Errorf("%s", msg)
	}
	if allDealableSpent(noBurn, settings.HandSize, g.played) {
		g.wipeDiscardLocked()
	}
	g.engine = &engine{}
	g.started = true
	g.paused = false
	out := g.engine.Begin(settings, rosterRows(seated), filterPlayed(noBurn, g.played), deckSupply{g}, g.rngLocked(), g.clock())
	g.applyOutcomeLocked(out)
	g.startTickerLocked()
	return nil
}

func rosterRows(players []games.Player) []rosterRow {
	rows := make([]rosterRow, 0, len(players))
	for _, p := range players {
		rows = append(rows, rosterRow{ID: p.ID, Name: p.DisplayName})
	}
	return rows
}

// seatLocked refreshes the engine's live roster before it acts.
func (g *Game) seatLocked() {
	if g.engine != nil && g.helper != nil {
		g.engine.SetSeated(rosterRows(g.helper.Seated()))
	}
}

func (g *Game) syncRosterLocked(h games.Helper) {
	if g.engine == nil || !g.started {
		return
	}
	g.applyOutcomeLocked(g.engine.SyncRoster(rosterRows(h.Seated()), g.clock()))
}

// applyOutcomeLocked carries out the engine's side effects: journals,
// wildcard saves, the burn drawer, the reshuffle overlay, and host calls.
func (g *Game) applyOutcomeLocked(out outcome) {
	for _, row := range out.Played {
		g.recordPlayedLocked(row.LibraryID, row.CardID)
	}
	if out.FlushDiscard {
		g.waitDiscardLocked(g.enqueueDiscardLocked())
	} else if len(out.Played) > 0 {
		g.enqueueDiscardLocked()
	}
	if len(out.Wildcards) > 0 && g.helper != nil {
		added := saveWildcards(g.helper.DataDir(), out.Wildcards)
		if g.engine.Settings.WildcardDealPrevious {
			g.engine.AddAnswers(added)
		}
	}
	if out.BurnDrawer {
		g.snapshotBurnDrawerLocked()
	}
	if out.Shortage != "" {
		g.showOverlayLocked(out.Shortage)
	}
	if out.Publish {
		g.publishLocked()
	}
	if out.Pause {
		g.needPause = true
	}
	if out.Finish {
		g.needFinish = true
	}
}

func (g *Game) rngLocked() *rand.Rand {
	if g.rng == nil {
		g.rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	return g.rng
}

func (g *Game) clock() time.Time {
	if g.now != nil {
		return g.now()
	}
	return time.Now()
}

func (g *Game) publishLocked() {
	if g.helper != nil {
		g.helper.Publish(eventMatch)
	}
}

// deckSupply reads the library, burns, and discard journal for the engine.
// Callers hold g.mu.
type deckSupply struct{ g *Game }

func (s deckSupply) unplayed(settings matchSettings) enabledPiles {
	h := s.g.helper
	if h == nil {
		return enabledPiles{}
	}
	cat := scanDataDir(h.DataDir())
	return filterPlayed(filterBurns(buildPiles(cat, settings), s.g.burns), s.g.played)
}

func (s deckSupply) recycleCovers(settings matchSettings, inHand map[string]bool, needP, needA int) bool {
	g := s.g
	if g.discardCorrupt || len(g.played) == 0 || g.helper == nil {
		return false
	}
	cat := scanDataDir(g.helper.DataDir())
	piles := filterBurns(buildPiles(cat, settings), g.burns)
	haveP, haveA := 0, 0
	for _, p := range piles.Prompts {
		if p.dealable(settings.HandSize) && !inHand[p.key()] {
			haveP++
		}
	}
	for _, a := range piles.Answers {
		if !inHand[a.key()] {
			haveA++
		}
	}
	return haveP >= needP && haveA >= needA
}

func (s deckSupply) wildcardTexts() []string {
	if s.g.helper == nil {
		return nil
	}
	return wildcardTexts(s.g.helper.DataDir())
}
