package apples

import (
	"fmt"
	"slices"
)

const (
	pendingStart   = "start"
	pendingDraw    = "draw"
	pendingSkip    = "skip"
	pendingMulti   = "multi"
	pendingBotFill = "bot-fill"

	overlayHostCopy = "You've reached the bottom of the barrel! Reshuffle the discard pile?"
	overlayWaitCopy = "Waiting on host to reshuffle."
	overlayTVCopy   = overlayHostCopy
	overlayFailCopy = "Not enough cards left to deal."
)

func (g *Game) overlayActive() bool {
	return g.overlay != ""
}

func (g *Game) showOverlayLocked(kind string) {
	g.overlay = kind
	g.overlayTooSmall = false
	if g.match != nil {
		g.freezeTimerLocked(g.match)
	}
}

func (g *Game) clearOverlayLocked() {
	g.overlay = ""
	g.overlayTooSmall = false
	if g.match != nil && !g.paused {
		g.thawTimerLocked(g.match)
	}
}

func (g *Game) discardWouldCover(needP, needA int, m *matchState) bool {
	if g.discardCorrupt || len(g.played) == 0 || m == nil {
		return false
	}
	h := g.helper
	if h == nil {
		return false
	}
	cat := scanDataDir(h.DataDir())
	piles := filterBurns(buildPiles(cat, m.Settings), g.burns)
	hands := inHandKeys(m)
	haveP, haveA := 0, 0
	for _, p := range piles.Prompts {
		if !p.dealable(m.Settings.HandSize) {
			continue
		}
		if hands[p.key()] {
			continue
		}
		haveP++
	}
	for _, a := range piles.Answers {
		if hands[a.key()] {
			continue
		}
		haveA++
	}
	return haveP >= needP && haveA >= needA
}

func (g *Game) openingOverlayLocked(m *matchState, seated int) bool {
	needP, needA := openingNeeds(m.Settings, seated)
	if dealablePromptCount(enabledPiles{Prompts: m.Prompts, Answers: m.Answers}, m.Settings.HandSize) >= needP && len(m.Answers) >= needA {
		return false
	}
	if g.discardWouldCover(needP, needA, m) {
		g.showOverlayLocked(pendingStart)
		return true
	}
	return false
}

func (g *Game) drawNeedLocked(m *matchState) (needP, needA int) {
	needP = 1
	if m.Settings.PromptMode == modeMulti {
		needP = 2
	}
	maxPick := 1
	for _, p := range m.Prompts {
		if p.dealable(m.Settings.HandSize) && p.Pick > maxPick {
			maxPick = p.Pick
		}
	}
	needA = 0
	live := g.seatedLiveLocked(g.helper)
	for _, a := range m.Actors {
		if a.Bot {
			needA += maxPick
			continue
		}
		if _, ok := live[a.ID]; !ok {
			continue
		}
		if n := m.Settings.HandSize - len(a.Hand); n > 0 {
			needA += n
		}
	}
	return
}

func (g *Game) maybeOverlayDealLocked(m *matchState, kind string, needP, needA int) bool {
	haveP := dealablePromptCount(enabledPiles{Prompts: m.Prompts}, m.Settings.HandSize)
	haveA := len(m.Answers)
	if haveP >= needP && haveA >= needA {
		return false
	}
	if g.discardWouldCover(needP, needA, m) {
		g.showOverlayLocked(kind)
		return true
	}
	return false
}

func (g *Game) rebuildUnplayedLocked(m *matchState) {
	h := g.helper
	if h == nil || m == nil {
		return
	}
	cat := scanDataDir(h.DataDir())
	piles := filterPlayed(filterBurns(buildPiles(cat, m.Settings), g.burns), g.played)
	hands := inHandKeys(m)
	var prompts []playPrompt
	var answers []playCard
	for _, p := range piles.Prompts {
		if hands[p.key()] {
			continue
		}
		prompts = append(prompts, p)
	}
	for _, a := range piles.Answers {
		if hands[a.key()] {
			continue
		}
		answers = append(answers, a)
	}
	g.rngLocked().Shuffle(len(prompts), func(i, j int) { prompts[i], prompts[j] = prompts[j], prompts[i] })
	g.rngLocked().Shuffle(len(answers), func(i, j int) { answers[i], answers[j] = answers[j], answers[i] })
	m.Prompts = prompts
	m.Answers = answers
}

func (g *Game) fulfillOverlayLocked(m *matchState) error {
	kind := g.overlay
	g.wipeDiscardLocked()
	g.rebuildUnplayedLocked(m)
	switch kind {
	case pendingStart:
		for _, a := range m.Actors {
			if a.Bot {
				continue
			}
			g.dealHumanLocked(m, a, m.Settings.HandSize)
		}
		if humanHandsShort(m, m.Settings.HandSize) {
			g.overlayTooSmall = true
			g.overlay = kind
			return nil
		}
	case pendingDraw, pendingMulti:
		if err := g.drawLocked(m); err != nil {
			g.overlayTooSmall = true
			g.overlay = kind
			return nil
		}
	case pendingSkip:
		if err := g.skipAfterReshuffleLocked(m); err != nil {
			g.overlayTooSmall = true
			g.overlay = kind
			return nil
		}
	case pendingBotFill:
		g.fillWaitingBotsLocked(m)
	}
	if g.overlayTooSmall {
		return nil
	}
	g.clearOverlayLocked()
	return nil
}

func seatedHumanCount(m *matchState) int {
	n := 0
	for _, a := range m.Actors {
		if !a.Bot {
			n++
		}
	}
	return n
}

func humanHandsShort(m *matchState, handSize int) bool {
	for _, a := range m.Actors {
		if !a.Bot && len(a.Hand) < handSize {
			return true
		}
	}
	return false
}

func (g *Game) skipAfterReshuffleLocked(m *matchState) error {
	if m.LivePrompt == nil {
		return fmt.Errorf("Skip is not available.")
	}
	old := *m.LivePrompt
	p, ok := g.popDealablePromptLocked(m)
	if !ok {
		return fmt.Errorf("%s", startRefuseMsg)
	}
	g.recordPlayedLocked(old.LibraryID, old.CardID)
	g.enqueueDiscardLocked()
	m.LivePrompt = &p
	g.recordPlayedLocked(p.LibraryID, p.CardID)
	g.enqueueDiscardLocked()
	if m.Phase == phaseSubmit {
		g.resetSubmitLocked(m)
		g.enterSubmitLocked(m)
		return nil
	}
	m.Phase = phaseHold
	g.clearTimerLocked(m)
	return nil
}

func (g *Game) fillWaitingBotsLocked(m *matchState) {
	if m.Phase != phaseSubmit || m.LivePrompt == nil {
		return
	}
	for _, a := range m.Actors {
		if a.Bot && !a.Locked && filledCount(a.Holes) == 0 {
			g.dealBotSubmitLocked(m, a)
		}
	}
	g.maybeRevealLocked(m)
}

func (g *Game) stripBurnedFromPilesLocked(m *matchState) {
	if m == nil {
		return
	}
	m.Prompts = slices.DeleteFunc(m.Prompts, func(p playPrompt) bool {
		return g.isBurned(kindPrompt, p.Text)
	})
	m.Answers = slices.DeleteFunc(m.Answers, func(c playCard) bool {
		return g.isBurned(kindAnswer, c.Text)
	})
}

func (g *Game) snapshotBurnDrawerLocked(m *matchState) {
	if m == nil || m.LivePrompt == nil {
		return
	}
	for _, p := range m.Packets {
		if !p.Revealed {
			return
		}
	}
	if len(m.Packets) == 0 {
		return
	}
	var faces []burnFace
	faces = append(faces, burnFace{
		Kind: kindPrompt, Text: m.LivePrompt.Text, Norm: normalizeCardText(m.LivePrompt.Text),
		Burned: g.isBurned(kindPrompt, m.LivePrompt.Text),
	})
	for _, p := range m.Packets {
		for _, c := range p.Cards {
			if c.Blank {
				continue
			}
			faces = append(faces, burnFace{
				Kind: kindAnswer, Text: c.Text, Norm: normalizeCardText(c.Text),
				Burned: g.isBurned(kindAnswer, c.Text),
			})
		}
	}
	g.burnSnap = faces
	g.burnChecks = map[string]bool{}
	g.burnErr = ""
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
