package quips

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
	if len(seated) < g.MinPlayers() {
		return fmt.Errorf("Start needs at least %d seated players.", g.MinPlayers())
	}
	if g.burnCorrupt {
		return fmt.Errorf("Burn list is unreadable")
	}
	if g.discardCorrupt {
		return fmt.Errorf("Discard list is unreadable")
	}
	piles := buildPromptPiles(cat, settings)
	noBurn := filterBurns(piles, g.burns)
	if msg := startShortagePiles(noBurn, settings, len(seated)); msg != "" {
		return fmt.Errorf("%s", msg)
	}
	if allPromptsSpent(noBurn, g.played) {
		g.wipeDiscardLocked()
	}
	unplayed := filterPlayed(noBurn, g.played)
	pool := append([]playPrompt(nil), unplayed.Prompts...)
	g.rngLocked().Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })

	rows := make([]rosterRow, 0, len(seated))
	for _, p := range seated {
		rows = append(rows, rosterRow{ID: p.ID, Name: p.DisplayName})
	}
	eng := &engine{}
	if err := eng.Begin(settings, rows, promptDeal{Pool: pool}, g.engineHooks(), g.rngLocked(), g.clock()); err != nil {
		if err == errOutOfPrompts {
			need := promptsNeededForRound(openingRoundKind(settings), len(rows))
			eng.bootstrap(settings, rows, g.engineHooks(), g.rngLocked(), g.clock())
			eng.PromptPool = pool
			if g.discardWouldCover(need, eng) {
				g.engine = eng
				g.started = true
				g.paused = false
				g.showOverlayLocked(pendingStart)
				g.startTickerLocked()
				return nil
			}
		}
		return err
	}
	g.engine = eng
	g.started = true
	g.paused = false
	g.startTickerLocked()
	return nil
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

func (g *Game) rngLocked() *rand.Rand {
	if g.rng == nil {
		g.rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	return g.rng
}

func (g *Game) clock() time.Time {
	return g.timeNow()
}

func (g *Game) syncRosterLocked(h games.Helper) {
	if g.engine == nil || !g.started {
		return
	}
	rows := make([]rosterRow, 0, len(h.Seated()))
	for _, p := range h.Seated() {
		rows = append(rows, rosterRow{ID: p.ID, Name: p.DisplayName})
	}
	g.engine.syncRoster(rows)
}

func (g *Game) applyOutcomeLocked(out Outcome) {
	if out.DealShortage {
		g.handleDealShortageLocked()
	}
	if !out.Changed && !out.DealShortage {
		return
	}
	for _, ev := range out.Events {
		if g.helper != nil {
			g.helper.Publish(ev)
		}
	}
	if out.Finish && g.helper != nil {
		g.needFinish = true
	}
	if out.Pause && g.helper != nil {
		g.needPause = true
	}
}

func (g *Game) publishLocked() {
	if g.helper != nil {
		g.helper.Publish(eventQuips)
	}
}
