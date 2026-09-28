package borrowedtruths

import (
	"html/template"

	"github.com/KroniK907/grabbag/internal/games"
)

// photoURL is where a shown photo is served. Previews swap it for inline art.
func (g *Game) photoURL(id string) template.URL {
	if g.photoSrc != nil {
		return g.photoSrc(id)
	}
	return template.URL("/play/photo/" + id)
}

// timFaces are the claimants in speaking order. speaking marks who has the
// floor during the claims.
func (e *engine) timFaces() []face {
	var out []face
	for i, id := range e.Round.Claimants {
		f := faceOf(e.player(id))
		f.Speaking = e.Phase == phaseClaims && i == e.Round.Speaker
		out = append(out, f)
	}
	return out
}

// fillTIMBoard draws a This Is My round. The owner and Not theirs stay out
// until the reveal.
func (g *Game) fillTIMBoard(view *boardView) {
	e := g.engine
	r := e.Round
	view.TIM = true
	view.PhotoURL = g.photoURL(e.Photos[r.Photo].ID)
	view.Claimants = e.timFaces()
	switch e.Phase {
	case phaseLook:
		view.Prompt = "One of them took this photo. Maybe."
	case phaseClaims:
		view.Prompt = e.player(r.Claimants[r.Speaker]).Name + " is telling you about it."
	case phaseTIMQuestion:
		view.Prompt = "Grill all three."
	case phaseTIMVote:
		view.Prompt = "Whose photo is it?"
		view.Voters, view.Locked = e.lockFaces(e.Votes)
	case phaseTIMReveal:
		owner := e.Photos[r.Photo].Owner
		counts := map[string]int{}
		for _, pick := range e.Votes {
			counts[pick]++
		}
		for i := range view.Claimants {
			view.Claimants[i].Count = counts[view.Claimants[i].ID]
			view.Claimants[i].Owner = view.Claimants[i].ID == owner
		}
		view.NoneCount = counts[pickNone]
		view.NotTheirs = r.NotTheirs
		view.Owner = faceOf(e.player(owner))
		view.Awards = e.awardViews()
	}
}

// fillTIMPhone draws one player's This Is My phone. A claimant learns only
// whether the photo is theirs. A hidden owner gets the insider line and the
// same vote screen as everyone else.
func (g *Game) fillTIMPhone(view *phoneView, p games.Player) {
	e := g.engine
	r := e.Round
	view.TIM = true
	view.PhotoURL = g.photoURL(e.Photos[r.Photo].ID)
	view.Speaker = e.player(r.Claimants[r.Speaker]).Name
	me := e.player(p.ID)
	owner := e.Photos[r.Photo].Owner
	claimant := me != nil && e.isClaimant(me.ID)
	switch {
	case claimant:
		view.Role = roleTeller
		view.TIMClaimant = true
		view.TIMYours = me.ID == owner
		if view.TIMYours {
			view.Badge = badgeView{Small: "this photo is", Marker: "Yours"}
			view.Hint = "Tell the truth."
		} else {
			view.Badge = badgeView{Small: "this photo is", Marker: "Not yours"}
			view.Hint = "Claim it anyway."
		}
		view.MySpeak = e.Phase == phaseClaims && r.Claimants[r.Speaker] == me.ID
	case me != nil && me.ID == e.insider() && e.Phase != phaseTIMReveal:
		view.Badge = badgeView{Small: "that's yours.", Marker: "Sell it."}
		view.Hint = "Nobody else knows. Join the questions if you like."
	}
	view.TIMOptions = e.timFaces()
	knew, didKnew := e.Knew[p.ID]
	if didKnew {
		view.KnewPicked = e.timPickLabel(knew.Pick)
	}
	switch e.Phase {
	case phaseLook:
		view.CanKnew = me != nil && !claimant && e.Settings.KnewIt && !didKnew
	case phaseTIMVote:
		switch {
		case claimant:
		case didKnew:
			view.SatOut = true
		case e.VoteClosed:
		case me != nil:
			view.CanVote = e.Votes[p.ID] == ""
			view.MyVote = e.timPickLabel(e.Votes[p.ID])
		case e.Settings.AudienceVote:
			view.CanVote = e.Crowd[p.ID] == ""
			view.MyVote = e.timPickLabel(e.Crowd[p.ID])
		}
	case phaseTIMReveal:
		if r.NotTheirs {
			view.Answer = "NONE OF THEM"
		} else {
			view.Answer = e.player(owner).Name
		}
		view.AnswerTrue = true
		for _, a := range e.Awards {
			if a.ID == p.ID {
				view.MyPoints += a.Points
			}
		}
	}
}

func (e *engine) timPickLabel(pick string) string {
	if pick == pickNone {
		return "None of them"
	}
	if p := e.player(pick); p != nil {
		return p.Name
	}
	return ""
}
