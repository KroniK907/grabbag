package apples

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/KroniK907/grabbag/internal/games"
	"github.com/KroniK907/grabbag/internal/games/runtimekit"
)

// withEngine runs one engine command for the signed-in player and renders
// their phone.
func (g *Game) withEngine(w http.ResponseWriter, r *http.Request, build func(games.Player) (command, error)) {
	g.run.Act(w, r, func(e *engine, p games.Player, now time.Time) runtimekit.Result {
		if h := g.run.HelperLocked(); h != nil {
			g.syncRosterLocked(h)
		}
		cmd, err := build(p)
		if err != nil {
			return runtimekit.Result{Err: err.Error()}
		}
		cmd.Actor = p.ID
		out := e.Do(cmd, now)
		res := g.resultLocked(e, out)
		if out.Err != nil {
			res.Err = out.Err.Error()
			return res
		}
		delete(e.PhoneErr, p.ID)
		res.Changed = true
		return res
	})
}

// post returns a handler for a command that needs no form parsing beyond
// the card field.
func (g *Game) post(kind commandKind) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		g.withEngine(w, r, func(games.Player) (command, error) {
			return command{Kind: kind, Card: r.FormValue(cardField[kind]), Text: r.FormValue("text")}, nil
		})
	}
}

// cardField is the form field each command reads into command.Card.
var cardField = map[commandKind]string{
	cmdChoosePrompt: "prompt",
	cmdSlot:         "card",
	cmdDiscard:      "card",
	cmdConfirm:      "winner",
}

func (g *Game) postUnslot(w http.ResponseWriter, r *http.Request) {
	g.withEngine(w, r, func(games.Player) (command, error) {
		n, err := strconv.Atoi(r.FormValue("hole"))
		if err != nil {
			n = -1
		}
		return command{Kind: cmdUnslot, Hole: n}, nil
	})
}

func (g *Game) postVote(w http.ResponseWriter, r *http.Request) {
	g.withEngine(w, r, func(p games.Player) (command, error) {
		if !playerMayVote(g.run.Engine(), p) {
			return command{}, fmt.Errorf("You cannot vote now.")
		}
		return command{Kind: cmdVote, Card: r.FormValue("target")}, nil
	})
}

func (g *Game) getBoardPartial(w http.ResponseWriter, r *http.Request) {
	g.run.Render(w, "board-frame", g.boardView(r), http.StatusOK)
}

func (g *Game) getPhonePartial(w http.ResponseWriter, r *http.Request) {
	g.run.Render(w, "phone-frame", g.phoneView(r), http.StatusOK)
}

func (g *Game) getHowto(w http.ResponseWriter, r *http.Request) {
	if g.run.Helper() == nil {
		http.NotFound(w, r)
		return
	}
	g.run.Render(w, "howto.html", g.howtoView(false), http.StatusOK)
}

func (g *Game) getHowtoSheet(w http.ResponseWriter, r *http.Request) {
	if g.run.Helper() == nil {
		http.NotFound(w, r)
		return
	}
	g.run.Render(w, "howto-sheet", g.howtoView(true), http.StatusOK)
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
		view := g.phoneViewLocked(p)
		g.run.Unlock()
		g.run.Render(w, "phone.html", view, http.StatusOK)
		return
	}
	if !p.ClaimedHost {
		g.burnErr = "Only the claimed host can burn cards."
		view := g.phoneViewLocked(p)
		view.BurnOpen = true
		g.run.Unlock()
		g.run.Render(w, "phone.html", view, http.StatusOK)
		return
	}
	if g.burnCorrupt {
		g.burnErr = "Burn list is unreadable"
		view := g.phoneViewLocked(p)
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
	g.run.Engine().StripBurned(g.isBurned)
	for i := range g.burnSnap {
		g.burnSnap[i].Burned = g.isBurned(g.burnSnap[i].Kind, g.burnSnap[i].Text)
		g.burnSnap[i].Checked = g.burnChecks[g.burnSnap[i].Kind+"\x00"+g.burnSnap[i].Text]
	}
	g.burnErr = ""
	view := g.phoneViewLocked(p)
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
		view := g.phoneViewLocked(p)
		g.run.Unlock()
		g.run.Render(w, "phone.html", view, http.StatusOK)
		return
	}
	if g.overlay == "" || g.overlayTooSmall {
		view := g.phoneViewLocked(p)
		g.run.Unlock()
		g.run.Render(w, "phone.html", view, http.StatusOK)
		return
	}
	g.fulfillOverlayLocked()
	g.run.Apply(runtimekit.Result{Changed: true})
	view := g.phoneViewLocked(p)
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
		view := g.phoneViewLocked(p)
		g.run.Unlock()
		g.run.Render(w, "phone.html", view, http.StatusOK)
		return
	}
	g.run.Apply(runtimekit.Result{Changed: true, Finish: true})
	view := g.phoneViewLocked(p)
	g.run.Unlock()
	g.run.Render(w, "phone.html", view, http.StatusOK)
}

func (g *Game) postFinish(w http.ResponseWriter, r *http.Request) {
	h := g.run.Helper()
	if h == nil {
		http.NotFound(w, r)
		return
	}
	_ = r.ParseForm()
	p, ok, err := h.PlayerFromRequest(r)
	host := err == nil && ok && p.ClaimedHost
	if !h.HasAdmin(r) && !host {
		http.Error(w, "Only the host can continue.", http.StatusForbidden)
		return
	}
	g.run.Lock()
	if !g.run.Running() || g.run.Engine().Phase != phaseOver {
		g.run.Unlock()
		http.NotFound(w, r)
		return
	}
	g.run.Apply(runtimekit.Result{Finish: true})
	g.run.Unlock()
	next := "/"
	if r.FormValue("return") == "/board" {
		next = "/board"
	}
	http.Redirect(w, r, next, http.StatusSeeOther)
}

// playerMayVote reports whether p's Lobby role may cast a favorite vote now.
func playerMayVote(m *engine, p games.Player) bool {
	if m == nil || m.Settings.Voting == voteOff || m.Phase != phaseReveal || m.NamesShown {
		return false
	}
	if p.ID == m.JudgeID {
		return false
	}
	switch m.Settings.Voting {
	case voteAudience:
		return p.Audience || p.Waiting
	case voteSeated:
		return p.Seated
	case voteBoth:
		return p.Seated || p.Audience || p.Waiting
	default:
		return false
	}
}
