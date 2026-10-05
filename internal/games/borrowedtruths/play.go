package borrowedtruths

import (
	"net/http"
	"time"

	"github.com/KroniK907/grabbag/internal/games"
	"github.com/KroniK907/grabbag/internal/games/runtimekit"
)

// action is one phone post against the engine. It returns a phone error, or
// "" when the post changed run state.
type action func(e *engine, p games.Player, r *http.Request, now time.Time) string

// step turns fn into a kit action. A change that leaves the match in facts
// publishes only the facts tick, which repaints the board and leaves a
// half-typed phone alone.
func step(r *http.Request, fn action) runtimekit.Action[*engine] {
	return func(e *engine, p games.Player, now time.Time) runtimekit.Result {
		before := e.Phase
		if msg := fn(e, p, r, now); msg != "" {
			return runtimekit.Result{Err: msg}
		}
		if before == phaseFacts && e.Phase == phaseFacts {
			return runtimekit.Result{Events: []string{eventFacts}}
		}
		return runtimekit.Result{Changed: true}
	}
}

// withEngine runs a signed-in player's fn and answers with the fresh phone
// column.
func (g *Game) withEngine(w http.ResponseWriter, r *http.Request, fn action) {
	g.run.Act(w, r, step(r, fn))
}

// withHost is withEngine for the claimed host or an admin session.
func (g *Game) withHost(w http.ResponseWriter, r *http.Request, fn action) {
	g.run.HostAct(w, r, step(r, fn))
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

func (g *Game) postHelp(w http.ResponseWriter, r *http.Request) {
	g.withEngine(w, r, func(e *engine, p games.Player, r *http.Request, now time.Time) string {
		return e.CallHelp(p.ID, r.FormValue("helped") == "1", now)
	})
}

func (g *Game) postReveal(w http.ResponseWriter, r *http.Request) {
	g.withEngine(w, r, func(e *engine, p games.Player, _ *http.Request, now time.Time) string {
		return e.Reveal(p.ID, now)
	})
}

func (g *Game) postHostContinue(w http.ResponseWriter, r *http.Request) {
	g.run.HostAct(w, r, func(e *engine, _ games.Player, now time.Time) runtimekit.Result {
		finish, changed := e.Continue(now)
		if !changed && !finish {
			return runtimekit.Result{Err: "Nothing to continue."}
		}
		return runtimekit.Result{Changed: true, Finish: finish}
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

func (g *Game) getBoardPartial(w http.ResponseWriter, r *http.Request) {
	g.run.Render(w, "board-frame", g.boardView(), http.StatusOK)
}

func (g *Game) getPhonePartial(w http.ResponseWriter, r *http.Request) {
	g.run.Render(w, "phone-frame", g.phoneView(r, ""), http.StatusOK)
}
