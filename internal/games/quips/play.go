package quips

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/KroniK907/grabbag/internal/games"
	"github.com/KroniK907/grabbag/internal/games/runtimekit"
)

// step turns an engine call into a kit action. It syncs the roster first and
// applies the Outcome's phone errors, which may name other players.
func (g *Game) step(fn func(*engine, games.Player) Outcome) runtimekit.Action[*engine] {
	return func(e *engine, p games.Player, _ time.Time) runtimekit.Result {
		if h := g.run.HelperLocked(); h != nil {
			g.syncRosterLocked(h)
		}
		out := fn(e, p)
		for id, msg := range out.PhoneErr {
			if msg != "" {
				e.PhoneErr[id] = msg
			} else {
				delete(e.PhoneErr, id)
			}
		}
		return g.resultLocked(out)
	}
}

// withEngine runs fn for the signed-in player and answers with their phone.
func (g *Game) withEngine(w http.ResponseWriter, r *http.Request, fn func(*engine, games.Player) Outcome) {
	g.run.Act(w, r, g.step(fn))
}

func (g *Game) postDraft(w http.ResponseWriter, r *http.Request) {
	p, ok := g.run.Player(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Could not read the form.", http.StatusBadRequest)
		return
	}
	slot, err := strconv.Atoi(r.FormValue("slot"))
	if err != nil {
		http.Error(w, "Bad slot.", http.StatusBadRequest)
		return
	}
	g.run.Lock()
	if !g.run.Running() || g.run.Paused() {
		g.run.Unlock()
		http.NotFound(w, r)
		return
	}
	if h := g.run.HelperLocked(); h != nil {
		g.syncRosterLocked(h)
	}
	out := g.run.Engine().Do(Command{
		Kind:  CmdDraft,
		Actor: p.ID,
		Slot:  slot,
		Text:  r.FormValue("text"),
	}, g.run.Now())
	for id, msg := range out.PhoneErr {
		if msg != "" {
			g.run.Engine().PhoneErr[id] = msg
		} else {
			delete(g.run.Engine().PhoneErr, id)
		}
	}
	if out.PhoneErr[p.ID] != "" {
		view := g.phoneViewLockedWithRequest(p, r)
		g.run.Unlock()
		g.run.Render(w, "phone.html", view, http.StatusBadRequest)
		return
	}
	g.run.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func (g *Game) postLock(w http.ResponseWriter, r *http.Request) {
	g.withEngine(w, r, func(eng *engine, p games.Player) Outcome {
		w := eng.Writers[p.ID]
		if w == nil {
			return Outcome{PhoneErr: map[string]string{p.ID: "You are not in this match."}}
		}
		drafts := make([]string, len(w.Slots))
		for i := range w.Slots {
			drafts[i] = w.Slots[i].Draft
		}
		return eng.Do(Command{Kind: CmdLock, Actor: p.ID, Drafts: drafts}, g.run.Now())
	})
}

func (g *Game) postVote(w http.ResponseWriter, r *http.Request) {
	g.withEngine(w, r, func(eng *engine, p games.Player) Outcome {
		h := g.run.HelperLocked()
		seated := false
		if h != nil {
			for _, row := range h.Seated() {
				if row.ID == p.ID {
					seated = true
					break
				}
			}
		}
		return eng.Do(Command{
			Kind:       CmdVote,
			Actor:      p.ID,
			VoteTarget: r.FormValue("target"),
			VoterSeat:  seated,
		}, g.run.Now())
	})
}

func (g *Game) postHostAction(w http.ResponseWriter, r *http.Request, kind CommandKind) {
	g.run.HostAct(w, r, g.step(func(eng *engine, _ games.Player) Outcome {
		return eng.Do(Command{Kind: kind, Actor: "host"}, g.run.Now())
	}))
}

func (g *Game) postHostReveal(w http.ResponseWriter, r *http.Request) {
	g.postHostAction(w, r, CmdHostReveal)
}

func (g *Game) postHostNextSegment(w http.ResponseWriter, r *http.Request) {
	g.postHostAction(w, r, CmdHostNextSegment)
}

func (g *Game) postHostSkipHold(w http.ResponseWriter, r *http.Request) {
	g.postHostAction(w, r, CmdHostSkipHold)
}

func (g *Game) postHostEndMatch(w http.ResponseWriter, r *http.Request) {
	g.postHostAction(w, r, CmdHostEndMatch)
}

func (g *Game) getBoardPartial(w http.ResponseWriter, r *http.Request) {
	g.run.Render(w, "board-frame", g.boardView(), http.StatusOK)
}

func (g *Game) getPhonePartial(w http.ResponseWriter, r *http.Request) {
	g.run.Render(w, "phone-frame", g.phoneView(r), http.StatusOK)
}

func (g *Game) postBurn(w http.ResponseWriter, r *http.Request) {
	p, ok := g.run.Player(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Could not read the form.", http.StatusBadRequest)
		return
	}
	g.run.Lock()
	if !g.run.Running() {
		g.run.Unlock()
		http.NotFound(w, r)
		return
	}
	if g.overlayActive() {
		view := g.phoneViewLockedWithRequest(p, r)
		g.run.Unlock()
		g.run.Render(w, "phone.html", view, http.StatusOK)
		return
	}
	if !p.ClaimedHost {
		g.burnErr = "Only the claimed host can burn cards."
		view := g.phoneViewLockedWithRequest(p, r)
		view.BurnOpen = true
		g.run.Unlock()
		g.run.Render(w, "phone.html", view, http.StatusOK)
		return
	}
	if g.burnCorrupt {
		g.burnErr = "Burn list is unreadable"
		view := g.phoneViewLockedWithRequest(p, r)
		view.BurnOpen = true
		g.run.Unlock()
		g.run.Render(w, "phone.html", view, http.StatusOK)
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
	g.stripBurnedFromPoolLocked()
	g.burnErr = ""
	view := g.phoneViewLockedWithRequest(p, r)
	view.BurnOpen = true
	g.run.Unlock()
	g.run.Render(w, "phone.html", view, http.StatusOK)
}

func (g *Game) postReshuffleYes(w http.ResponseWriter, r *http.Request) {
	p, ok := g.run.Player(w, r)
	if !ok {
		return
	}
	g.run.Lock()
	if !g.run.Running() {
		g.run.Unlock()
		http.NotFound(w, r)
		return
	}
	if !p.ClaimedHost {
		g.run.Engine().PhoneErr[p.ID] = overlayWaitCopy
		view := g.phoneViewLockedWithRequest(p, r)
		g.run.Unlock()
		g.run.Render(w, "phone.html", view, http.StatusOK)
		return
	}
	if g.overlay == "" || g.overlayTooSmall {
		view := g.phoneViewLockedWithRequest(p, r)
		g.run.Unlock()
		g.run.Render(w, "phone.html", view, http.StatusOK)
		return
	}
	_ = g.fulfillOverlayLocked()
	view := g.phoneViewLockedWithRequest(p, r)
	g.run.Unlock()
	g.run.Render(w, "phone.html", view, http.StatusOK)
}

func (g *Game) postEndGame(w http.ResponseWriter, r *http.Request) {
	p, ok := g.run.Player(w, r)
	if !ok {
		return
	}
	g.run.Lock()
	if !g.run.Running() {
		g.run.Unlock()
		http.NotFound(w, r)
		return
	}
	if !p.ClaimedHost || g.overlay == "" {
		view := g.phoneViewLockedWithRequest(p, r)
		g.run.Unlock()
		g.run.Render(w, "phone.html", view, http.StatusOK)
		return
	}
	g.run.Apply(runtimekit.Result{Changed: true, Finish: true})
	view := g.phoneViewLockedWithRequest(p, r)
	g.run.Unlock()
	g.run.Render(w, "phone.html", view, http.StatusOK)
}
