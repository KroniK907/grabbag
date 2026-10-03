package borrowedtruths

import (
	"net/http"
	"time"

	"github.com/KroniK907/grabbag/internal/games"
)

// action is one phone post against the engine. It returns a phone error, or
// "" when the post changed run state.
type action func(e *engine, p games.Player, r *http.Request, now time.Time) string

// withEngine runs a signed-in player's fn under the lock and answers with the
// fresh phone column.
func (g *Game) withEngine(w http.ResponseWriter, r *http.Request, fn action) {
	h := g.helperNow()
	if h == nil {
		http.NotFound(w, r)
		return
	}
	p, ok, err := h.PlayerFromRequest(r)
	if err != nil || !ok {
		http.Error(w, "Sign in to play.", http.StatusUnauthorized)
		return
	}
	g.runAction(w, r, p, fn)
}

// withHost is withEngine for host posts. An admin session needs no player
// cookie, so p may be the zero Player.
func (g *Game) withHost(w http.ResponseWriter, r *http.Request, fn action) {
	p, ok := g.hostOnly(w, r)
	if !ok {
		return
	}
	g.runAction(w, r, p, fn)
}

func (g *Game) runAction(w http.ResponseWriter, r *http.Request, p games.Player, fn action) {
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
	msg := "The match is paused."
	if !g.paused {
		before := g.engine.Phase
		msg = fn(g.engine, p, r, g.clock())
		if msg == "" && before == phaseFacts && g.engine != nil && g.engine.Phase == phaseFacts {
			// Facts ticks repaint only the board, so a half-typed phone is not swapped.
			g.publishFactsLocked()
			g.afterLocked(false, false)
		} else {
			g.afterLocked(false, msg == "")
		}
	}
	view := g.phoneViewLocked(p, msg)
	g.mu.Unlock()
	g.render(w, "phone-frame", view, http.StatusOK)
}

func (g *Game) postFacts(w http.ResponseWriter, r *http.Request) {
	g.withEngine(w, r, func(e *engine, p games.Player, r *http.Request, now time.Time) string {
		if msg := e.SubmitFacts(p.ID, r.Form["truth"], r.Form["lie"]); msg != "" {
			return msg
		}
		if e.FactsReady() {
			e.Continue(now)
		}
		return ""
	})
}

func (g *Game) postSkip(w http.ResponseWriter, r *http.Request) {
	g.withEngine(w, r, func(e *engine, p games.Player, _ *http.Request, now time.Time) string {
		return e.Skip(p.ID, now)
	})
}

func (g *Game) postLockIn(w http.ResponseWriter, r *http.Request) {
	g.withEngine(w, r, func(e *engine, p games.Player, _ *http.Request, now time.Time) string {
		return e.LockIn(p.ID, now)
	})
}

func (g *Game) postKnew(w http.ResponseWriter, r *http.Request) {
	g.withEngine(w, r, func(e *engine, p games.Player, r *http.Request, now time.Time) string {
		return e.CallKnew(p.ID, r.FormValue("pick"), r.FormValue("owner"), now)
	})
}

func (g *Game) postVote(w http.ResponseWriter, r *http.Request) {
	g.withEngine(w, r, func(e *engine, p games.Player, r *http.Request, _ time.Time) string {
		return e.Vote(p.ID, r.FormValue("pick"), p.Seated)
	})
}

func (g *Game) postOwnerVote(w http.ResponseWriter, r *http.Request) {
	g.withEngine(w, r, func(e *engine, p games.Player, r *http.Request, now time.Time) string {
		return e.OwnerVote(p.ID, r.FormValue("target"), now)
	})
}

func (g *Game) postReveal(w http.ResponseWriter, r *http.Request) {
	g.withEngine(w, r, func(e *engine, p games.Player, _ *http.Request, now time.Time) string {
		return e.Reveal(p.ID, now)
	})
}

// hostOnly lets the claimed host or an admin session through. It returns the
// request's player, which is the zero Player for an admin with no player
// cookie.
func (g *Game) hostOnly(w http.ResponseWriter, r *http.Request) (games.Player, bool) {
	h := g.helperNow()
	if h == nil {
		http.NotFound(w, r)
		return games.Player{}, false
	}
	p, ok, err := h.PlayerFromRequest(r)
	if err != nil || !ok {
		p = games.Player{}
	}
	if h.HasAdmin(r) || p.ClaimedHost {
		return p, true
	}
	http.Error(w, "Host only.", http.StatusForbidden)
	return games.Player{}, false
}

func (g *Game) postHostContinue(w http.ResponseWriter, r *http.Request) {
	g.withHost(w, r, func(e *engine, _ games.Player, _ *http.Request, now time.Time) string {
		finish, changed := e.Continue(now)
		if finish {
			g.needFinish = true
		}
		if !changed && !finish {
			return "Nothing to continue."
		}
		return ""
	})
}

func (g *Game) postHostVoid(w http.ResponseWriter, r *http.Request) {
	g.withHost(w, r, func(e *engine, _ games.Player, _ *http.Request, now time.Time) string {
		return e.Void(now)
	})
}

func (g *Game) postHostExtend(w http.ResponseWriter, r *http.Request) {
	g.withHost(w, r, func(e *engine, _ games.Player, _ *http.Request, now time.Time) string {
		return e.Extend(now)
	})
}

func (g *Game) postHostPause(w http.ResponseWriter, r *http.Request) {
	g.withHost(w, r, func(*engine, games.Player, *http.Request, time.Time) string {
		g.needPause = true
		return ""
	})
}

// postHostResume is the one host post that runs while paused.
func (g *Game) postHostResume(w http.ResponseWriter, r *http.Request) {
	p, ok := g.hostOnly(w, r)
	if !ok {
		return
	}
	g.mu.Lock()
	if g.engine == nil || !g.started {
		g.mu.Unlock()
		http.NotFound(w, r)
		return
	}
	g.needResume = true
	g.afterLocked(false, false)
	view := g.phoneViewLocked(p, "")
	g.mu.Unlock()
	g.render(w, "phone-frame", view, http.StatusOK)
}

func (g *Game) getBoardPartial(w http.ResponseWriter, r *http.Request) {
	g.render(w, "board-frame", g.boardView(), http.StatusOK)
}

func (g *Game) getPhonePartial(w http.ResponseWriter, r *http.Request) {
	g.render(w, "phone-frame", g.phoneView(r, ""), http.StatusOK)
}
