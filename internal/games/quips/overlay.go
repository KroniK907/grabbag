package quips

const (
	pendingStart = "start"
	pendingRound = "round"

	overlayHostCopy = "You've reached the bottom of the barrel! Reshuffle the discard pile?"
	overlayWaitCopy = "Waiting on host to reshuffle."
	overlayTVCopy   = overlayHostCopy
	overlayFailCopy = "Not enough prompts left to deal."
)

func (g *Game) overlayActive() bool {
	return g.overlay != ""
}

func (g *Game) showOverlayLocked(kind string) {
	g.overlay = kind
	g.overlayTooSmall = false
	g.run.Hold()
}

func (g *Game) clearOverlayLocked() {
	g.overlay = ""
	g.overlayTooSmall = false
	g.run.Hold()
}

func promptsNeededForRound(kind roundKind, seated int) int {
	if seated < 1 {
		return 0
	}
	if kind == roundLastQuip {
		return 1
	}
	return seated
}

func openingRoundKind(settings matchSettings) roundKind {
	if settings.LastQuipEnabled && settings.RoundCount == 1 {
		return roundLastQuip
	}
	return roundStandard
}

func (g *Game) activeSegmentPromptKeys(eng *engine) map[string]bool {
	out := map[string]bool{}
	if eng == nil {
		return out
	}
	for _, seg := range eng.Segments {
		if seg.Prompt.LibraryID != "" && seg.Prompt.CardID != "" {
			out[seg.Prompt.key()] = true
		}
	}
	return out
}

func (g *Game) recyclableDiscardCount(eng *engine) int {
	if g.discardCorrupt || len(g.played) == 0 {
		return 0
	}
	active := g.activeSegmentPromptKeys(eng)
	n := 0
	for k := range g.played {
		if active[k] {
			continue
		}
		n++
	}
	return n
}

func (g *Game) discardWouldCover(need int, eng *engine) bool {
	if need <= 0 || g.discardCorrupt || len(g.played) == 0 || eng == nil {
		return false
	}
	have := len(eng.PromptPool) + g.recyclableDiscardCount(eng)
	return have >= need
}

func (g *Game) handleDealShortageLocked() {
	if g.run.Engine() == nil {
		return
	}
	kind := openingRoundKind(g.run.Engine().Settings)
	if g.run.Engine().Round > 1 || len(g.run.Engine().Segments) > 0 {
		kind = g.run.Engine().Kind
	}
	need := promptsNeededForRound(kind, len(g.run.Engine().seatedIDs()))
	if g.discardWouldCover(need, g.run.Engine()) {
		overlayKind := pendingRound
		if g.run.Engine().Phase == "" || len(g.run.Engine().Segments) == 0 {
			overlayKind = pendingStart
		}
		g.showOverlayLocked(overlayKind)
		return
	}
	g.overlayTooSmall = true
	g.showOverlayLocked(pendingRound)
}

func (g *Game) partialReshuffleDiscardLocked(eng *engine) {
	active := g.activeSegmentPromptKeys(eng)
	next := map[string]playedRow{}
	for k, row := range g.played {
		if active[k] {
			next[k] = row
		}
	}
	g.played = next
	g.enqueueDiscardLocked()
}

func (g *Game) rebuildPromptPoolLocked(eng *engine) {
	h := g.run.HelperLocked()
	if h == nil || eng == nil {
		return
	}
	cat := scanDataDir(h.DataDir())
	piles := filterPlayed(filterBurns(buildPromptPiles(cat, eng.Settings), g.burns), g.played)
	pool := append([]playPrompt(nil), piles.Prompts...)
	g.run.Rand().Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	eng.PromptPool = pool
}

func (g *Game) fulfillOverlayLocked() error {
	if g.run.Engine() == nil {
		return nil
	}
	kind := g.overlay
	g.partialReshuffleDiscardLocked(g.run.Engine())
	g.rebuildPromptPoolLocked(g.run.Engine())
	if err := g.run.Engine().openWriteRoundForIDs(g.run.Now()); err != nil {
		if err == errOutOfPrompts {
			g.overlayTooSmall = true
			g.overlay = kind
			return nil
		}
		return err
	}
	g.clearOverlayLocked()
	g.run.Publish(eventQuips)
	return nil
}

func overlayCopy(claimedHost bool, tooSmall bool) (title string, showYes bool) {
	if tooSmall {
		return overlayFailCopy, false
	}
	if claimedHost {
		return overlayHostCopy, true
	}
	return overlayWaitCopy, false
}
