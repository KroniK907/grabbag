package borrowedtruths

import (
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/KroniK907/grabbag/internal/games"
	"github.com/KroniK907/grabbag/internal/games/runtimekit"
)

// face is a player as the board and phones draw them.
type face struct {
	ID       string
	Name     string
	Seed     string
	Speaking bool
	Locked   bool
	Owner    bool
	Count    int
	Crowd    int
	Score    int
	Rank     int
}

type awardView struct {
	Name   string
	Points int
	Why    string
}

type ribbonView struct {
	Title string
	Line  string
	Names []string
}

// boardView is everything the TV may show. Card type, owner, and author are
// filled only in the reveal phases, so the board cannot leak them early.
type boardView struct {
	runtimekit.Page
	Paused    bool
	Phase     string
	TellNum   int
	TellTotal int
	Teller    face
	Notice    string
	Facts     []face
	CardText  string
	Prompt    string
	Voters    []face
	Locked    int
	Timer     runtimekit.TimerView

	Answer      string
	AnswerTrue  bool
	Borrowed    bool
	Author      string
	TrueCount   int
	LieCount    int
	Crowd       bool
	CrowdTrue   int
	CrowdLie    int
	Awards      []awardView
	OwnerSplit  []face
	Owner       face
	Standings   []face
	Ribbons     []ribbonView
	FinalWinner string

	// This Is My. Owner and NotTheirs are set only at the reveal.
	TIM       bool
	PhotoURL  template.URL
	Claimants []face
	NoneCount int
	CrowdNone int
	NotTheirs bool
}

func (g *Game) boardView() boardView {
	g.run.Lock()
	defer g.run.Unlock()
	view := g.boardViewLocked()
	view.Page = g.run.PageLocked("Borrowed Truths")
	return view
}

func (g *Game) boardViewLocked() boardView {
	e := g.run.Engine()
	view := boardView{Paused: g.run.Paused()}
	if e == nil {
		return view
	}
	view.Phase = string(e.Phase)
	view.Notice = e.Notice
	view.Timer = e.Timer.View(g.run.Now())
	view.TellTotal = e.TellCount()
	view.TellNum = e.TellNum()
	if t := e.teller(); t != nil && e.Phase != phaseFinal {
		view.Teller = faceOf(t)
	}
	switch e.Phase {
	case phaseFacts:
		for _, p := range e.Players {
			f := faceOf(p)
			f.Locked = p.Done
			view.Facts = append(view.Facts, f)
		}
		return view
	case phasePrivate:
		return view
	case phaseStandings, phaseFinal:
		view.Standings = standingFaces(e)
		if e.Phase == phaseFinal {
			for _, r := range e.ribbons() {
				rv := ribbonView{Title: r.Title, Line: r.Line}
				for _, p := range r.Winners {
					rv.Names = append(rv.Names, p.Name)
				}
				view.Ribbons = append(view.Ribbons, rv)
			}
			var top []string
			for _, f := range view.Standings {
				if f.Rank == 1 {
					top = append(top, f.Name)
				}
			}
			switch len(top) {
			case 0:
			case 1:
				view.FinalWinner = top[0] + " wins"
			default:
				view.FinalWinner = strings.Join(top, " and ") + " win"
			}
		}
		return view
	}
	if e.Phase.isTIM() {
		g.fillTIMBoard(&view)
		return view
	}
	view.CardText = e.Facts[e.Card.Fact].Text
	switch e.Phase {
	case phasePublic, phaseQuestion:
		view.Prompt = "Grill " + view.Teller.Name + "."
	case phaseVote:
		view.Prompt = "True or lie? Lock it in."
		view.Voters, view.Locked = e.lockFaces(e.Votes)
	case phaseOwnerVote:
		view.Prompt = "It's true for someone here. Whose is it?"
		view.Voters, view.Locked = e.lockFaces(e.OwnerVotes)
	}
	if e.Phase == phaseReveal || e.Phase == phaseOwnerReveal {
		e.fillReveal(&view)
	}
	return view
}

// lockFaces shows who has acted, never what. A voter who tapped I knew it
// shows as locked so the board does not single them out.
func (e *engine) lockFaces(picks map[string]string) ([]face, int) {
	var out []face
	n := 0
	for _, p := range e.voters() {
		f := faceOf(p)
		_, picked := picks[p.ID]
		_, knew := e.Knew[p.ID]
		f.Locked = picked || knew
		if f.Locked {
			n++
		}
		out = append(out, f)
	}
	return out, n
}

func (e *engine) fillReveal(view *boardView) {
	view.AnswerTrue = !e.Card.isLie()
	view.Answer = "LIE"
	if view.AnswerTrue {
		view.Answer = "TRUE"
	}
	view.Borrowed = e.Card.Kind == kindBorrowed
	f := e.Facts[e.Card.Fact]
	if e.Card.Kind == kindLie && f.Owner != "" {
		view.Author = e.player(f.Owner).Name
	}
	for _, pick := range e.Votes {
		if pick == pickTrue {
			view.TrueCount++
		} else {
			view.LieCount++
		}
	}
	view.Crowd = len(e.Crowd) > 0
	for _, pick := range e.Crowd {
		if pick == pickTrue {
			view.CrowdTrue++
		} else {
			view.CrowdLie++
		}
	}
	view.Awards = e.awardViews()
	if e.Phase != phaseOwnerReveal {
		return
	}
	counts := map[string]int{}
	for _, target := range e.OwnerVotes {
		counts[target]++
	}
	for _, p := range e.voters() {
		fc := faceOf(p)
		fc.Count = counts[p.ID]
		fc.Owner = p.ID == f.Owner
		if fc.Owner {
			view.Owner = fc
		}
		view.OwnerSplit = append(view.OwnerSplit, fc)
	}
}

func (e *engine) awardViews() []awardView {
	var out []awardView
	for _, a := range e.Awards {
		out = append(out, awardView{Name: e.player(a.ID).Name, Points: a.Points, Why: a.Why})
	}
	return out
}

func standingFaces(e *engine) []face {
	var out []face
	rank, last := 0, -1
	for i, p := range e.standings() {
		if p.Score != last {
			rank, last = i+1, p.Score
		}
		f := faceOf(p)
		f.Score = p.Score
		f.Rank = rank
		out = append(out, f)
	}
	return out
}

func faceOf(p *player) face {
	return face{ID: p.ID, Name: p.Name, Seed: p.Seed}
}

// Phone roles.
const (
	roleWriter   = "writer"
	roleTeller   = "teller"
	roleVoter    = "voter"
	roleAudience = "audience"
	roleOut      = "out"
)

// badgeView is the private HELLO name tag on teller and insider phones.
type badgeView struct {
	Small  string
	Marker string
}

type hostView struct {
	Show       bool
	Continue   string
	CanVoid    bool
	CanExtend  bool
	Paused     bool
	PhaseLabel string
	PhaseStart int64
}

type pickOption struct {
	Value string
	Label string
	Small string
	Class string
}

// phoneView is one player's column. Private fields are set only for the
// player who may see them.
type phoneView struct {
	runtimekit.Page
	Paused     bool
	Phase      string
	Role       string
	Error      string
	TellerName string
	CardText   string
	Badge      badgeView
	Hint       string
	Timer      runtimekit.TimerView

	TruthSlots []int
	LieSlots   []int
	MaxChars   int
	FactsDone  bool
	FactsLeft  int

	CanSkip    bool
	SkipsLeft  int
	CanLockIn  bool
	CanReveal  bool
	RevealLine string

	CanKnew    bool
	KnewPicked string
	Names      []face

	CanVote      bool
	VoteOptions  []pickOption
	MyVote       string
	CanOwnerVote bool
	SatOut       bool

	Answer     string
	AnswerTrue bool
	MyPoints   int
	Standings  []face
	Ribbons    []ribbonView
	Host       hostView

	// This Is My.
	TIM         bool
	PhotoURL    template.URL
	HasPhoto    bool
	PhotoAsk    bool
	TIMClaimant bool
	TIMYours    bool
	Speaker     string
	MySpeak     bool
	TIMOptions  []face
}

func (g *Game) phoneView(r *http.Request, msg string) phoneView {
	var p games.Player
	if h := g.run.Helper(); h != nil {
		p, _, _ = h.PlayerFromRequest(r)
	}
	g.run.Lock()
	defer g.run.Unlock()
	return g.phoneViewLocked(p, msg)
}

func (g *Game) phoneViewLocked(p games.Player, msg string) phoneView {
	view := g.phoneViewFor(p, g.run.Now())
	view.Page = g.run.PageLocked("Borrowed Truths")
	view.Error = msg
	return view
}

var voteOptions = []pickOption{
	{Value: pickTrue, Label: "TRUE", Class: "is-true"},
	{Value: pickLie, Label: "LIE", Small: "a pure lie", Class: "is-lie"},
	{Value: pickBorrowed, Label: "LIE", Small: "someone else's truth", Class: "is-borrowed"},
}

var pickLabels = map[string]string{
	pickTrue:     "True",
	pickLie:      "Lie: a pure lie",
	pickBorrowed: "Lie: someone else's truth",
}

func (g *Game) phoneViewFor(p games.Player, now time.Time) phoneView {
	e := g.run.Engine()
	view := phoneView{Paused: g.run.Paused(), Role: roleAudience, MaxChars: maxFactRunes}
	if e == nil {
		return view
	}
	view.Phase = string(e.Phase)
	view.Timer = e.Timer.View(now)
	view.Host = e.hostView(p.ClaimedHost, g.run.Paused())
	me := e.player(p.ID)
	t := e.teller()
	isTeller := me != nil && t != nil && t.ID == me.ID
	if t != nil {
		view.TellerName = t.Name
	}
	switch {
	case isTeller:
		view.Role = roleTeller
	case me != nil:
		view.Role = roleVoter
	}

	switch e.Phase {
	case phaseFacts:
		if me == nil {
			return view
		}
		view.Role = roleWriter
		view.FactsDone = me.Done
		for i := 0; i < e.Settings.TruthsPerPlayer; i++ {
			view.TruthSlots = append(view.TruthSlots, i+1)
		}
		for i := 0; i < e.Settings.LiesPerPlayer; i++ {
			view.LieSlots = append(view.LieSlots, i+1)
		}
		for _, q := range e.Players {
			if !q.Done {
				view.FactsLeft++
			}
		}
		view.PhotoAsk = e.TIMPlanned
		view.HasPhoto = e.HasPhoto(me.ID)
		return view
	case phaseStandings, phaseFinal:
		view.Standings = standingFaces(e)
		if e.Phase == phaseFinal {
			for _, r := range e.ribbons() {
				rv := ribbonView{Title: r.Title, Line: r.Line}
				for _, w := range r.Winners {
					rv.Names = append(rv.Names, w.Name)
				}
				view.Ribbons = append(view.Ribbons, rv)
			}
		}
		return view
	}

	if e.Phase.isTIM() {
		g.fillTIMPhone(&view, p)
		return view
	}
	f := e.Facts[e.Card.Fact]
	if isTeller {
		view.CardText = f.Text
		view.Badge, view.Hint = e.tellerBadge()
	} else if e.Phase != phasePrivate {
		view.CardText = f.Text
	}
	insider := me != nil && !isTeller && me.ID == e.insider()
	knew, didKnew := e.Knew[p.ID]
	if didKnew {
		view.KnewPicked = pickLabels[knew.Pick]
		if knew.Pick == pickBorrowed {
			if o := e.player(knew.Owner); o != nil {
				view.KnewPicked = "Lie: " + o.Name + "'s truth"
			}
		}
	}

	switch e.Phase {
	case phasePrivate:
		if isTeller {
			view.CanSkip = e.Skips > 0
			view.SkipsLeft = e.Skips
			view.CanLockIn = true
		}
	case phasePublic, phaseQuestion:
		if insider {
			view.Badge = badgeView{Small: "that's yours.", Marker: "Sell it."}
		}
		if isTeller {
			view.Hint = "Read it out loud. Then answer their questions."
		}
		if me != nil && !isTeller && e.Phase == phasePublic && e.Settings.KnewIt && !didKnew {
			view.CanKnew = true
			view.Names = e.nameList(me.ID)
		}
	case phaseVote:
		switch {
		case isTeller:
			view.CanReveal = e.VoteClosed
			view.RevealLine = e.revealLine()
		case didKnew:
			view.SatOut = true
		case e.VoteClosed:
		case me != nil:
			view.CanVote = e.Votes[p.ID] == ""
			view.MyVote = pickLabels[e.Votes[p.ID]]
		case e.Settings.AudienceVote:
			view.CanVote = e.Crowd[p.ID] == ""
			view.MyVote = pickLabels[e.Crowd[p.ID]]
		}
		if view.CanVote {
			view.VoteOptions = voteOptions
		}
	case phaseOwnerVote:
		switch {
		case isTeller, me == nil:
		case didKnew:
			view.SatOut = true
		default:
			target := e.OwnerVotes[p.ID]
			view.CanOwnerVote = target == ""
			if o := e.player(target); o != nil {
				view.MyVote = o.Name
			}
			view.Names = e.nameList(me.ID)
		}
	case phaseReveal, phaseOwnerReveal:
		view.AnswerTrue = !e.Card.isLie()
		view.Answer = "LIE"
		if view.AnswerTrue {
			view.Answer = "TRUE"
		}
		for _, a := range e.Awards {
			if a.ID == p.ID {
				view.MyPoints += a.Points
			}
		}
	}
	return view
}

// tellerBadge is the private card type the teller sees, and what to do.
func (e *engine) tellerBadge() (badgeView, string) {
	f := e.Facts[e.Card.Fact]
	switch e.Card.Kind {
	case kindYours:
		return badgeView{Small: "this truth is", Marker: "Yours"}, "It's true. Tell it straight."
	case kindBorrowed:
		name := e.player(f.Owner).Name
		return badgeView{Small: "this truth belongs to", Marker: name}, "Claim it. " + name + " might back you up."
	default:
		return badgeView{Small: "this lie is", Marker: "Yours now"}, "It's a lie. Make it sound true."
	}
}

func (e *engine) revealLine() string {
	switch e.Card.Kind {
	case kindYours:
		return "It's true."
	case kindBorrowed:
		return "It's a lie. But it's true for someone here."
	default:
		return "It's a lie."
	}
}

// nameList is every seated player a voter could name: not the teller, not
// themselves. Every phone gets the same shape of list.
func (e *engine) nameList(self string) []face {
	var out []face
	for _, p := range e.voters() {
		if p.ID != self {
			out = append(out, faceOf(p))
		}
	}
	return out
}

func (e *engine) hostView(claimed, paused bool) hostView {
	if !claimed {
		return hostView{}
	}
	v := hostView{
		Show:       true,
		Paused:     paused,
		PhaseStart: e.PhaseStart.Unix(),
		CanExtend:  e.Timer.On() && !e.Extended,
	}
	switch e.Phase {
	case phaseFacts:
		v.PhaseLabel, v.Continue = "Facts", "Close facts"
	case phasePrivate:
		v.PhaseLabel, v.Continue = "Private read", "Show the card"
	case phasePublic:
		v.PhaseLabel, v.Continue = "Public read", "Start questions"
	case phaseQuestion:
		v.PhaseLabel, v.Continue = "Questioning", "Open the vote"
	case phaseVote:
		v.PhaseLabel, v.Continue = "Vote", "Close the vote"
		if e.VoteClosed {
			v.Continue = "Reveal"
		}
	case phaseReveal:
		v.PhaseLabel, v.Continue = "Reveal", "Standings"
		if e.Card.Kind == kindBorrowed {
			v.Continue = "Whose truth?"
		}
	case phaseOwnerVote:
		v.PhaseLabel, v.Continue = "Owner vote", "Name the owner"
	case phaseOwnerReveal:
		v.PhaseLabel, v.Continue = "Owner reveal", "Standings"
	case phaseStandings:
		v.PhaseLabel, v.Continue = "Standings", "Next tell"
		if e.Tell+1 >= len(e.Order) {
			v.Continue = "Final scores"
		} else if e.Order[e.Tell+1].TIM {
			v.Continue = "This Is My"
		}
	case phaseLook:
		v.PhaseLabel, v.Continue = "This Is My: look", "Start claims"
	case phaseClaims:
		v.PhaseLabel, v.Continue = "This Is My: claims", "Next claimant"
		if e.Round.Speaker+1 >= len(e.Round.Claimants) {
			v.Continue = "Start questions"
		}
	case phaseTIMQuestion:
		v.PhaseLabel, v.Continue = "This Is My: questioning", "Open the vote"
	case phaseTIMVote:
		v.PhaseLabel, v.Continue = "This Is My: vote", "Close the vote"
		if e.VoteClosed {
			v.Continue = "Reveal"
		}
	case phaseTIMReveal:
		v.PhaseLabel, v.Continue = "This Is My: reveal", "Standings"
	case phaseFinal:
		v.PhaseLabel, v.Continue = "Final", "End game"
	}
	switch e.Phase {
	case phasePrivate, phasePublic, phaseQuestion, phaseVote,
		phaseLook, phaseClaims, phaseTIMQuestion, phaseTIMVote:
		v.CanVoid = true
	}
	return v
}
