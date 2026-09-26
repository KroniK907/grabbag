package apples

const (
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
	g.holdLocked()
}

func (g *Game) clearOverlayLocked() {
	g.overlay = ""
	g.overlayTooSmall = false
	g.holdLocked()
}

// fulfillOverlayLocked empties the discard journal and retries the deal
// that raised the overlay. The overlay stays up, marked too small, when the
// reshuffle still leaves the piles short.
func (g *Game) fulfillOverlayLocked() {
	g.wipeDiscardLocked()
	g.seatLocked()
	out := g.engine.Reshuffled(g.overlay, g.clock())
	g.applyOutcomeLocked(out)
	if out.TooSmall {
		g.overlayTooSmall = true
		return
	}
	g.clearOverlayLocked()
}

// snapshotBurnDrawerLocked fills the host's burn drawer with the round's
// prompt and answers once every answer is face up.
func (g *Game) snapshotBurnDrawerLocked() {
	e := g.engine
	if e == nil || e.LivePrompt == nil || len(e.Packets) == 0 || !e.allRevealed() {
		return
	}
	faces := []burnFace{{
		Kind: kindPrompt, Text: e.LivePrompt.Text, Norm: normalizeCardText(e.LivePrompt.Text),
		Burned: g.isBurned(kindPrompt, e.LivePrompt.Text),
	}}
	for _, p := range e.Packets {
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
