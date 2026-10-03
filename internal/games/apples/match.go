package apples

import (
	"fmt"

	"github.com/KroniK907/grabbag/internal/games"
	"github.com/KroniK907/grabbag/internal/games/runtimekit"
)

// beginMatchLocked builds the match for Start. A deal that raises the
// reshuffle overlay starts the match with its clock held.
func (g *Game) beginMatchLocked(h games.Helper) (*engine, error) {
	cat := scanDataDir(h.DataDir())
	settings, ok := g.loadSettings(h)
	settings = reconcileSettings(settings, ok, cat)
	seated := h.Seated()
	if len(seated) < 1 {
		return nil, fmt.Errorf("Start needs a seated player.")
	}
	if g.burnCorrupt {
		return nil, fmt.Errorf("Burn list is unreadable")
	}
	if g.discardCorrupt {
		return nil, fmt.Errorf("Discard list is unreadable")
	}
	noBurn := filterBurns(buildPiles(cat, settings), g.burns)
	if msg := startShortage(noBurn, settings, len(seated)); msg != "" {
		return nil, fmt.Errorf("%s", msg)
	}
	if allDealableSpent(noBurn, settings.HandSize, g.played) {
		g.wipeDiscardLocked()
	}
	e := &engine{}
	now := g.run.Now()
	g.resultLocked(e, e.Begin(settings, rosterRows(seated), filterPlayed(noBurn, g.played), deckSupply{g}, g.run.Rand(), now))
	if g.overlayActive() {
		e.SetPaused(true, now)
	}
	return e, nil
}

func rosterRows(players []games.Player) []rosterRow {
	rows := make([]rosterRow, 0, len(players))
	for _, p := range players {
		rows = append(rows, rosterRow{ID: p.ID, Name: p.DisplayName, ClaimedHost: p.ClaimedHost})
	}
	return rows
}

// seatLocked refreshes the engine's live roster before it acts.
func (g *Game) seatLocked() {
	if e, h := g.run.Engine(), g.run.HelperLocked(); e != nil && h != nil {
		e.SetSeated(rosterRows(h.Seated()))
	}
}

// syncRosterLocked brings late sitters and leavers into the match. Views call
// it too, so its host calls wait for the next Apply.
func (g *Game) syncRosterLocked(h games.Helper) {
	if !g.run.Running() {
		return
	}
	e := g.run.Engine()
	g.run.Defer(g.resultLocked(e, e.SyncRoster(rosterRows(h.Seated()), g.run.Now())))
}

// resultLocked carries out the engine's side effects on e: journals,
// wildcard saves, the burn drawer, and the reshuffle overlay. It returns the
// publish and host calls for the kit. Err stays with the caller.
func (g *Game) resultLocked(e *engine, out outcome) runtimekit.Result {
	for _, row := range out.Played {
		g.recordPlayedLocked(row.LibraryID, row.CardID)
	}
	if out.FlushDiscard {
		g.waitDiscardLocked(g.enqueueDiscardLocked())
	} else if len(out.Played) > 0 {
		g.enqueueDiscardLocked()
	}
	if h := g.run.HelperLocked(); len(out.Wildcards) > 0 && h != nil {
		added := saveWildcards(h.DataDir(), out.Wildcards)
		if e.Settings.WildcardDealPrevious {
			e.AddAnswers(added)
		}
	}
	if out.BurnDrawer {
		g.snapshotBurnDrawerLocked()
	}
	if out.Shortage != "" {
		g.showOverlayLocked(out.Shortage)
	}
	return runtimekit.Result{Changed: out.Publish, Pause: out.Pause, Finish: out.Finish}
}

// deckSupply reads the library, burns, and discard journal for the engine.
// Callers hold the game lock.
type deckSupply struct{ g *Game }

func (s deckSupply) unplayed(settings matchSettings) enabledPiles {
	h := s.g.run.HelperLocked()
	if h == nil {
		return enabledPiles{}
	}
	cat := scanDataDir(h.DataDir())
	return filterPlayed(filterBurns(buildPiles(cat, settings), s.g.burns), s.g.played)
}

func (s deckSupply) recycleCovers(settings matchSettings, inHand map[string]bool, needP, needA int) bool {
	g := s.g
	if g.discardCorrupt || len(g.played) == 0 || g.run.HelperLocked() == nil {
		return false
	}
	cat := scanDataDir(g.run.HelperLocked().DataDir())
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
	if s.g.run.HelperLocked() == nil {
		return nil
	}
	return wildcardTexts(s.g.run.HelperLocked().DataDir())
}
