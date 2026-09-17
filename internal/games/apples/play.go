package apples

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/KroniK907/grabbag/internal/games"
)

func (g *Game) playerOr401(w http.ResponseWriter, r *http.Request) (games.Player, bool) {
	h := g.helperNow()
	if h == nil {
		http.NotFound(w, r)
		return games.Player{}, false
	}
	p, ok, err := h.PlayerFromRequest(r)
	if err != nil || !ok {
		http.Error(w, "Sign in to play.", http.StatusUnauthorized)
		return games.Player{}, false
	}
	return p, true
}

func (g *Game) withMatch(w http.ResponseWriter, r *http.Request, fn func(*matchState, games.Player) error) {
	p, ok := g.playerOr401(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Could not read the form.", http.StatusBadRequest)
		return
	}
	g.mu.Lock()
	if g.match == nil || !g.started {
		g.mu.Unlock()
		http.NotFound(w, r)
		return
	}
	if g.overlayActive() {
		g.match.PhoneErr[p.ID] = overlayWaitCopy
		view := g.phoneViewLocked(p)
		g.mu.Unlock()
		g.render(w, "phone.html", view, http.StatusOK)
		return
	}
	if g.helper != nil {
		g.syncRosterLocked(g.helper)
	}
	err := fn(g.match, p)
	if err != nil {
		g.match.PhoneErr[p.ID] = err.Error()
	} else {
		delete(g.match.PhoneErr, p.ID)
		g.publishLocked()
	}
	view := g.phoneViewLocked(p)
	g.flushHostLocked()
	g.mu.Unlock()
	g.render(w, "phone.html", view, http.StatusOK)
}

func (g *Game) flushHostLocked() {
	h := g.helper
	finish := g.needFinish
	pause := g.needPause
	g.needFinish = false
	g.needPause = false
	if h == nil {
		return
	}
	if pause {
		g.mu.Unlock()
		h.Pause()
		g.mu.Lock()
	}
	if finish {
		g.mu.Unlock()
		h.Finish()
		g.mu.Lock()
	}
}

func (g *Game) postDraw(w http.ResponseWriter, r *http.Request) {
	g.withMatch(w, r, func(m *matchState, p games.Player) error {
		if p.ID != m.JudgeID {
			return fmt.Errorf("Only the judge can draw.")
		}
		return g.drawLocked(m)
	})
}

func (g *Game) postSkip(w http.ResponseWriter, r *http.Request) {
	g.withMatch(w, r, func(m *matchState, p games.Player) error {
		if p.ID != m.JudgeID {
			return fmt.Errorf("Only the judge can skip.")
		}
		return g.skipLocked(m)
	})
}

func (g *Game) postKeepPrompt(w http.ResponseWriter, r *http.Request) {
	g.withMatch(w, r, func(m *matchState, p games.Player) error {
		if p.ID != m.JudgeID {
			return fmt.Errorf("Only the judge can lock in the prompt.")
		}
		return g.keepPromptLocked(m)
	})
}

func (g *Game) postChoosePrompt(w http.ResponseWriter, r *http.Request) {
	g.withMatch(w, r, func(m *matchState, p games.Player) error {
		if p.ID != m.JudgeID {
			return fmt.Errorf("Only the judge can choose.")
		}
		return g.choosePromptLocked(m, r.FormValue("prompt"))
	})
}

func (g *Game) postSlot(w http.ResponseWriter, r *http.Request) {
	g.withMatch(w, r, func(m *matchState, p games.Player) error {
		return g.slotLocked(m, p.ID, r.FormValue("card"))
	})
}

func (g *Game) postUnslot(w http.ResponseWriter, r *http.Request) {
	g.withMatch(w, r, func(m *matchState, p games.Player) error {
		n, err := strconv.Atoi(r.FormValue("hole"))
		if err != nil {
			return fmt.Errorf("That hole is empty.")
		}
		return g.unslotLocked(m, p.ID, n)
	})
}

func (g *Game) postDiscard(w http.ResponseWriter, r *http.Request) {
	g.withMatch(w, r, func(m *matchState, p games.Player) error {
		return g.markDiscardLocked(m, p.ID, r.FormValue("card"))
	})
}

func (g *Game) postLock(w http.ResponseWriter, r *http.Request) {
	g.withMatch(w, r, func(m *matchState, p games.Player) error {
		return g.lockLocked(m, p.ID)
	})
}

func (g *Game) postWildcardDraft(w http.ResponseWriter, r *http.Request) {
	g.withMatch(w, r, func(m *matchState, p games.Player) error {
		return g.setDraftLocked(m, p.ID, r.FormValue("text"))
	})
}

func (g *Game) postReveal(w http.ResponseWriter, r *http.Request) {
	g.withMatch(w, r, func(m *matchState, p games.Player) error {
		if p.ID != m.JudgeID {
			return fmt.Errorf("Only the judge can reveal.")
		}
		return g.revealNextLocked(m)
	})
}

func (g *Game) postVote(w http.ResponseWriter, r *http.Request) {
	g.withMatch(w, r, func(m *matchState, p games.Player) error {
		if !g.playerMayVote(m, p) {
			return fmt.Errorf("You cannot vote now.")
		}
		return g.voteLocked(m, p.ID, r.FormValue("target"))
	})
}

func (g *Game) postConfirm(w http.ResponseWriter, r *http.Request) {
	g.withMatch(w, r, func(m *matchState, p games.Player) error {
		if p.ID != m.JudgeID {
			return fmt.Errorf("Only the judge can confirm.")
		}
		return g.confirmLocked(g.helper, m, r.FormValue("winner"))
	})
}

func (g *Game) getBoardPartial(w http.ResponseWriter, r *http.Request) {
	g.render(w, "board-frame", g.boardView(), http.StatusOK)
}

func (g *Game) getPhonePartial(w http.ResponseWriter, r *http.Request) {
	g.render(w, "phone-frame", g.phoneView(r), http.StatusOK)
}

func (g *Game) getHowto(w http.ResponseWriter, r *http.Request) {
	if g.helperNow() == nil {
		http.NotFound(w, r)
		return
	}
	g.render(w, "howto.html", g.howtoView(false), http.StatusOK)
}

func (g *Game) getHowtoSheet(w http.ResponseWriter, r *http.Request) {
	if g.helperNow() == nil {
		http.NotFound(w, r)
		return
	}
	g.render(w, "howto-sheet", g.howtoView(true), http.StatusOK)
}

func (g *Game) postBurn(w http.ResponseWriter, r *http.Request) {
	p, ok := g.playerOr401(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Could not read the form.", http.StatusBadRequest)
		return
	}
	g.mu.Lock()
	if g.match == nil || !g.started {
		g.mu.Unlock()
		http.NotFound(w, r)
		return
	}
	if g.overlayActive() {
		view := g.phoneViewLocked(p)
		g.mu.Unlock()
		g.render(w, "phone.html", view, http.StatusOK)
		return
	}
	if !p.ClaimedHost {
		g.burnErr = "Only the claimed host can burn cards."
		view := g.phoneViewLocked(p)
		view.BurnOpen = true
		g.mu.Unlock()
		g.render(w, "phone.html", view, http.StatusOK)
		return
	}
	if g.burnCorrupt {
		g.burnErr = "Burn list is unreadable"
		view := g.phoneViewLocked(p)
		view.BurnOpen = true
		g.mu.Unlock()
		g.render(w, "phone.html", view, http.StatusOK)
		return
	}
	g.burnChecks = map[string]bool{}
	for _, face := range r.Form["face"] {
		kind, text, ok := strings.Cut(face, "\t")
		if !ok {
			continue
		}
		g.burnChecks[kind+"\x00"+text] = true
		g.burnPairLocked(kind, text)
	}
	g.stripBurnedFromPilesLocked(g.match)
	for i := range g.burnSnap {
		g.burnSnap[i].Burned = g.isBurned(g.burnSnap[i].Kind, g.burnSnap[i].Text)
		g.burnSnap[i].Checked = g.burnChecks[g.burnSnap[i].Kind+"\x00"+g.burnSnap[i].Text]
	}
	g.burnErr = ""
	view := g.phoneViewLocked(p)
	view.BurnOpen = true
	g.mu.Unlock()
	g.render(w, "phone.html", view, http.StatusOK)
}

func (g *Game) postReshuffleYes(w http.ResponseWriter, r *http.Request) {
	p, ok := g.playerOr401(w, r)
	if !ok {
		return
	}
	g.mu.Lock()
	if g.match == nil || !g.started {
		g.mu.Unlock()
		http.NotFound(w, r)
		return
	}
	if !p.ClaimedHost {
		g.match.PhoneErr[p.ID] = overlayWaitCopy
		view := g.phoneViewLocked(p)
		g.mu.Unlock()
		g.render(w, "phone.html", view, http.StatusOK)
		return
	}
	if g.overlay == "" || g.overlayTooSmall {
		view := g.phoneViewLocked(p)
		g.mu.Unlock()
		g.render(w, "phone.html", view, http.StatusOK)
		return
	}
	_ = g.fulfillOverlayLocked(g.match)
	g.publishLocked()
	view := g.phoneViewLocked(p)
	g.flushHostLocked()
	g.mu.Unlock()
	g.render(w, "phone.html", view, http.StatusOK)
}

func (g *Game) postEndGame(w http.ResponseWriter, r *http.Request) {
	p, ok := g.playerOr401(w, r)
	if !ok {
		return
	}
	g.mu.Lock()
	if g.match == nil || !g.started {
		g.mu.Unlock()
		http.NotFound(w, r)
		return
	}
	if !p.ClaimedHost || g.overlay == "" {
		view := g.phoneViewLocked(p)
		g.mu.Unlock()
		g.render(w, "phone.html", view, http.StatusOK)
		return
	}
	g.needFinish = true
	g.publishLocked()
	view := g.phoneViewLocked(p)
	g.flushHostLocked()
	g.mu.Unlock()
	g.render(w, "phone.html", view, http.StatusOK)
}
