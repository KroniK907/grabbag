package quips

import (
	"fmt"

	"github.com/KroniK907/grabbag/internal/games"
	"github.com/KroniK907/grabbag/internal/games/runtimekit"
)

// beginMatchLocked builds the match for Start. When the piles are short but
// the discard would cover them, the match starts behind the reshuffle
// overlay with its clock held.
func (g *Game) beginMatchLocked(h games.Helper) (*engine, error) {
	cat := scanDataDir(h.DataDir())
	settings, ok := g.loadSettings(h)
	settings = reconcileSettings(settings, ok, cat)
	seated := h.Seated()
	if len(seated) < g.MinPlayers() {
		return nil, fmt.Errorf("Start needs at least %d seated players.", g.MinPlayers())
	}
	if g.burnCorrupt {
		return nil, fmt.Errorf("Burn list is unreadable")
	}
	if g.discardCorrupt {
		return nil, fmt.Errorf("Discard list is unreadable")
	}
	piles := buildPromptPiles(cat, settings)
	noBurn := filterBurns(piles, g.burns)
	if msg := startShortagePiles(noBurn, settings, len(seated)); msg != "" {
		return nil, fmt.Errorf("%s", msg)
	}
	if allPromptsSpent(noBurn, g.played) {
		g.wipeDiscardLocked()
	}
	unplayed := filterPlayed(noBurn, g.played)
	pool := append([]playPrompt(nil), unplayed.Prompts...)
	g.run.Rand().Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })

	rows := make([]rosterRow, 0, len(seated))
	for _, p := range seated {
		rows = append(rows, rosterRow{ID: p.ID, Name: p.DisplayName})
	}
	eng := &engine{}
	if err := eng.Begin(settings, rows, promptDeal{Pool: pool}, g.engineHooks(), g.run.Rand(), g.run.Now()); err != nil {
		if err == errOutOfPrompts {
			need := promptsNeededForRound(openingRoundKind(settings), len(rows))
			eng.bootstrap(settings, rows, g.engineHooks(), g.run.Rand(), g.run.Now())
			eng.PromptPool = pool
			if g.discardWouldCover(need, eng) {
				g.showOverlayLocked(pendingStart)
				eng.SetPaused(true, g.run.Now())
				return eng, nil
			}
		}
		return nil, err
	}
	return eng, nil
}

func (g *Game) engineHooks() EngineHooks {
	return EngineHooks{
		OnPromptAssigned: func(libraryID, cardID string) {
			g.recordPlayedLocked(libraryID, cardID)
			g.enqueueDiscardLocked()
		},
		OnSegmentVoteOpen: func(prompt playPrompt) {
			g.pushBurnDrawerLocked(prompt)
		},
	}
}

func (g *Game) syncRosterLocked(h games.Helper) {
	if !g.run.Running() {
		return
	}
	rows := make([]rosterRow, 0, len(h.Seated()))
	for _, p := range h.Seated() {
		rows = append(rows, rosterRow{ID: p.ID, Name: p.DisplayName})
	}
	g.run.Engine().syncRoster(rows)
}

// resultLocked carries out a deal shortage and turns an engine Outcome into
// the kit Result. Phone errors are applied by the caller.
func (g *Game) resultLocked(out Outcome) runtimekit.Result {
	if out.DealShortage {
		g.handleDealShortageLocked()
	}
	if !out.Changed && !out.DealShortage {
		return runtimekit.Result{}
	}
	return runtimekit.Result{Events: out.Events, Finish: out.Finish, Pause: out.Pause}
}
