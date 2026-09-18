package quips

import (
	"net/http"
	"strconv"

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

func (g *Game) withEngine(w http.ResponseWriter, r *http.Request, fn func(*engine, games.Player) Outcome) {
	p, ok := g.playerOr401(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Could not read the form.", http.StatusBadRequest)
		return
	}
	g.mu.Lock()
	if g.engine == nil || !g.started {
		g.mu.Unlock()
		http.NotFound(w, r)
		return
	}
	if g.paused {
		g.engine.PhoneErr[p.ID] = "Match is paused."
		view := g.phoneViewLockedWithRequest(p, r)
		g.mu.Unlock()
		g.render(w, "phone.html", view, http.StatusOK)
		return
	}
	if g.helper != nil {
		g.syncRosterLocked(g.helper)
	}
	out := fn(g.engine, p)
	for id, msg := range out.PhoneErr {
		if msg != "" {
			g.engine.PhoneErr[id] = msg
		} else {
			delete(g.engine.PhoneErr, id)
		}
	}
	g.applyOutcomeLocked(out)
	view := g.phoneViewLockedWithRequest(p, r)
	g.flushHostLocked()
	g.mu.Unlock()
	g.render(w, "phone.html", view, http.StatusOK)
}

func (g *Game) postDraft(w http.ResponseWriter, r *http.Request) {
	p, ok := g.playerOr401(w, r)
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
	g.mu.Lock()
	if g.engine == nil || !g.started || g.paused {
		g.mu.Unlock()
		http.NotFound(w, r)
		return
	}
	if g.helper != nil {
		g.syncRosterLocked(g.helper)
	}
	out := g.engine.Do(Command{
		Kind:  CmdDraft,
		Actor: p.ID,
		Slot:  slot,
		Text:  r.FormValue("text"),
	}, g.clock())
	for id, msg := range out.PhoneErr {
		if msg != "" {
			g.engine.PhoneErr[id] = msg
		}
	}
	g.mu.Unlock()
	if out.PhoneErr[p.ID] != "" {
		http.Error(w, out.PhoneErr[p.ID], http.StatusBadRequest)
		return
	}
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
		return eng.Do(Command{Kind: CmdLock, Actor: p.ID, Drafts: drafts}, g.clock())
	})
}

func (g *Game) postVote(w http.ResponseWriter, r *http.Request) {
	g.withEngine(w, r, func(eng *engine, p games.Player) Outcome {
		h := g.helper
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
		}, g.clock())
	})
}

func (g *Game) postHostAction(w http.ResponseWriter, r *http.Request, kind CommandKind) {
	h := g.helperNow()
	if h == nil || !h.HasAdmin(r) {
		http.Error(w, "Host only.", http.StatusForbidden)
		return
	}
	g.withEngine(w, r, func(eng *engine, _ games.Player) Outcome {
		return eng.Do(Command{Kind: kind, Actor: "host"}, g.clock())
	})
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
	g.render(w, "board-frame", g.boardView(), http.StatusOK)
}

func (g *Game) getPhonePartial(w http.ResponseWriter, r *http.Request) {
	g.render(w, "phone-frame", g.phoneView(r), http.StatusOK)
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
