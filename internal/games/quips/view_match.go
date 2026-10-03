package quips

import (
	"net/http"
	"sort"
	"strings"

	"github.com/KroniK907/grabbag/internal/games"
	"github.com/KroniK907/grabbag/internal/games/runtimekit"
)

type boardView struct {
	runtimekit.Page
	Paused       bool
	Overlay      bool
	OverlayCopy  string
	Phase        string
	Round        int
	Multiplier   int
	CenterPrompt string
	WritingBeat  bool
	LastQuip     bool
	LastIntro    bool
	VoteIntro    bool
	PlayIntro    bool
	ParadeBeat   bool
	Matchup      bool
	MatchupKey   string
	Quips        []quipBoardView
	HoldAwards   []holdAwardView
	Champions    []string
	WinnerScore  int
	Roster       []rosterView
	TimerLabel   string
	Timer        runtimekit.TimerView
	LiveCounts   map[string]int
}

type quipBoardView struct {
	WriterID string
	Name     string
	Text     string
	Empty    bool
	Revealed bool
	Count    int
	Won      bool
}

type holdAwardView struct {
	Name   string
	Points int
}

type rosterView struct {
	Name   string
	Locked bool
	Score  int
}

type phoneView struct {
	runtimekit.Page
	Paused       bool
	ClaimedHost  bool
	BurnFaces    []burnFace
	BurnErr      string
	BurnOpen     bool
	Overlay      bool
	OverlayCopy  string
	OverlayYes   bool
	Role         string
	VoteIntro    bool
	PlayIntro    bool
	LastQuip     bool
	LastIntro    bool
	Error        string
	WaitCopy     string
	Slots        []slotView
	VoteOptions  []voteOptionView
	Voted        bool
	LockLabel    string
	LockReady    bool
	Locked       bool
	Policy       composePolicy
	HostBar      bool
	HostReveal   bool
	HostNext     bool
	HostSkipHold bool
	HostEndMatch bool
	TimerLabel   string
	Timer        runtimekit.TimerView
}

type voteOptionView struct {
	Target string
	Label  string
	Count  int
	Chosen bool
	Self   bool
}

type slotView struct {
	Index      int
	PromptText string
	Draft      string
	Remain     int
	Cap        int
	DupOutline bool
	Disabled   bool
}

func (g *Game) boardView() boardView {
	g.run.Lock()
	defer g.run.Unlock()
	return g.boardViewLocked()
}

func (g *Game) boardViewLocked() boardView {
	view := boardView{
		Page:  g.run.PageLocked("Quick Quips"),
		Phase: "setup",
	}
	if g.overlay != "" {
		view.Overlay = true
		view.OverlayCopy = overlayTVCopy
		if g.overlayTooSmall {
			view.OverlayCopy = overlayFailCopy
		}
	}
	if g.run.Engine() == nil {
		return view
	}
	eng := g.run.Engine()
	view.Paused = g.run.Paused()
	view.Phase = string(eng.Phase)
	view.Round = eng.Round
	view.Multiplier = eng.Multiplier
	view.WritingBeat = eng.Phase == phaseWrite
	view.LastQuip = eng.Kind == roundLastQuip && eng.Phase == phaseWrite
	view.LastIntro = view.LastQuip && eng.Timer.Kind == timerLastIntro
	view.VoteIntro = eng.Phase == phaseVoteIntro
	view.PlayIntro = eng.Phase == phaseWrite && eng.Timer.Kind == timerPlayIntro
	view.ParadeBeat = eng.Phase == phaseReveal || eng.Phase == phaseVote || eng.Phase == phaseHold
	view.Roster = g.rosterViewsLocked(eng)
	view.TimerLabel, view.Timer = g.timerViewLocked(eng)
	view.LiveCounts = eng.liveVoteCounts()

	segIdx := eng.activeSegmentIdx()
	if segIdx >= 0 && segIdx < len(eng.Segments) && eng.Phase != phaseWrite && eng.Phase != phaseVoteIntro {
		view.CenterPrompt = eng.Segments[segIdx].Prompt.Text
	}
	revealed := eng.revealedWriterIDs(segIdx)
	revealedSet := map[string]struct{}{}
	for _, id := range revealed {
		revealedSet[id] = struct{}{}
	}
	writers := eng.segmentWriters(segIdx)
	for _, id := range writers {
		w := eng.Writers[id]
		name := id
		if w != nil {
			name = w.Name
		}
		text := eng.quipForWriter(segIdx, id)
		_, isRev := revealedSet[id]
		show := isRev || eng.Phase == phaseVote || eng.Phase == phaseHold || eng.Phase == phaseFinalScores
		if eng.Kind == roundLastQuip && !eng.Settings.HostControlledReveals && eng.Phase != phaseWrite {
			show = true
		}
		if !eng.Settings.HostControlledReveals && eng.Phase == phaseReveal {
			show = true
		}
		q := quipBoardView{
			WriterID: id,
			Name:     name,
			Text:     text,
			Empty:    text == "",
			Revealed: show,
		}
		if view.LiveCounts != nil {
			q.Count = view.LiveCounts[id]
		}
		view.Quips = append(view.Quips, q)
	}
	if eng.Phase == phaseHold {
		best := 0
		for _, pts := range eng.HoldAwards {
			if pts > best {
				best = pts
			}
		}
		if best > 0 {
			for i := range view.Quips {
				if eng.HoldAwards[view.Quips[i].WriterID] == best {
					view.Quips[i].Won = true
				}
			}
		}
	}
	if view.ParadeBeat && !(eng.Phase == phaseReveal && eng.Kind == roundLastQuip) {
		var ids []string
		for _, q := range view.Quips {
			if q.Revealed {
				ids = append(ids, q.WriterID)
			}
		}
		if len(ids) == 2 {
			view.Matchup = true
			view.MatchupKey = strings.Join(ids, "|")
		}
	}
	if eng.Phase == phaseHold {
		ids := make([]string, 0, len(eng.HoldAwards))
		for id := range eng.HoldAwards {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			name := id
			if w := eng.Writers[id]; w != nil {
				name = w.Name
			}
			view.HoldAwards = append(view.HoldAwards, holdAwardView{Name: name, Points: eng.HoldAwards[id]})
		}
	}
	if eng.Phase == phaseFinalScores || eng.Phase == phaseOver {
		for _, id := range eng.Champions {
			if w := eng.Writers[id]; w != nil {
				view.Champions = append(view.Champions, w.Name)
				view.WinnerScore = eng.Scores[id]
			}
		}
	}
	return view
}

func (g *Game) phoneView(r *http.Request) phoneView {
	h := g.run.Helper()
	if h == nil {
		return phoneView{Page: g.run.Page("Quick Quips")}
	}
	p, ok, _ := h.PlayerFromRequest(r)
	if !ok {
		return phoneView{Page: g.run.Page("Quick Quips")}
	}
	g.run.Lock()
	defer g.run.Unlock()
	return g.phoneViewLockedWithRequest(p, r)
}

func (g *Game) phoneViewLocked(p games.Player) phoneView {
	return g.phoneViewLockedWithRequest(p, nil)
}

func (g *Game) phoneViewLockedWithRequest(p games.Player, r *http.Request) phoneView {
	view := phoneView{
		Page:        g.run.PageLocked("Quick Quips"),
		Role:        "wait",
		WaitCopy:    "Hang tight.",
		ClaimedHost: p.ClaimedHost,
	}
	if g.overlay != "" {
		view.Overlay = true
		copy, yes := overlayCopy(p.ClaimedHost, g.overlayTooSmall)
		view.OverlayCopy = copy
		view.OverlayYes = yes
		return view
	}
	if g.run.Engine() == nil {
		return view
	}
	eng := g.run.Engine()
	view.Paused = g.run.Paused()
	view.Policy = eng.Policy
	if msg := eng.PhoneErr[p.ID]; msg != "" {
		view.Error = msg
	}
	if msg := eng.PhoneErr["host"]; msg != "" && r != nil && g.run.HelperLocked() != nil && g.run.HelperLocked().HasAdmin(r) {
		view.Error = msg
	}
	view.TimerLabel, view.Timer = g.timerViewLocked(eng)

	canHost := p.ClaimedHost
	if r != nil && g.run.HelperLocked() != nil && g.run.HelperLocked().HasAdmin(r) {
		canHost = true
	}
	if canHost {
		switch eng.Phase {
		case phaseReveal:
			view.HostReveal = eng.Settings.HostControlledReveals
		case phaseParadeWait:
			view.HostNext = eng.Settings.HostControlledReveals
		case phaseHold:
			if eng.Settings.WinnerScreenSec == 0 {
				view.HostNext = true
			} else {
				view.HostSkipHold = eng.Settings.HostControlledReveals
			}
		case phaseFinalScores:
			view.HostEndMatch = eng.Settings.FinalScoresSec == 0 || eng.Settings.HostControlledReveals
		}
		view.HostBar = view.HostReveal || view.HostNext || view.HostSkipHold || view.HostEndMatch
	}

	switch eng.Phase {
	case phaseWrite:
		if eng.Timer.Kind == timerPlayIntro {
			view.Role = "wait"
			view.PlayIntro = true
			view.TimerLabel, view.Timer = "", runtimekit.TimerView{}
			return g.attachBurnDrawer(view, p)
		}
		if eng.Kind == roundLastQuip && eng.Timer.Kind == timerLastIntro {
			view.Role = "wait"
			view.LastQuip = true
			view.LastIntro = true
			view.TimerLabel, view.Timer = "", runtimekit.TimerView{}
			return g.attachBurnDrawer(view, p)
		}
		if eng.Kind == roundLastQuip {
			view.LastQuip = true
		}
		return g.phoneComposeView(view, eng, p)
	case phaseVote:
		view.Role = "vote"
		counts := eng.liveVoteCounts()
		pick := ""
		if eng.Vote.Picks != nil {
			pick = eng.Vote.Picks[p.ID].Target
		}
		view.Voted = pick != ""
		for _, id := range eng.segmentWriters(eng.activeSegmentIdx()) {
			label := id
			if w := eng.Writers[id]; w != nil {
				label = w.Name
			}
			text := eng.quipForWriter(eng.activeSegmentIdx(), id)
			if text != "" {
				label = text
			} else if text == "" {
				label = "Empty quip"
			}
			opt := voteOptionView{
				Target: id,
				Label:  label,
				Chosen: id == pick,
				Self:   !eng.Settings.AllowSelfVote && id == p.ID,
			}
			if counts != nil {
				opt.Count = counts[id]
			}
			view.VoteOptions = append(view.VoteOptions, opt)
		}
		return g.attachBurnDrawer(view, p)
	case phaseVoteIntro:
		view.VoteIntro = true
	case phaseParadeWait:
		view.WaitCopy = "Waiting for the host to start the vote parade."
	case phaseReveal:
		view.WaitCopy = "Quips are being revealed."
	case phaseHold:
		view.WaitCopy = "Segment scores are on the board."
	case phaseFinalScores:
		view.WaitCopy = "Final scores are on the board."
	default:
		view.WaitCopy = "Waiting for the next beat."
	}
	return g.attachBurnDrawer(view, p)
}

func (g *Game) attachBurnDrawer(view phoneView, p games.Player) phoneView {
	if p.ClaimedHost && g.run.Running() {
		view.BurnFaces = g.burnDrawerFacesLocked()
		view.BurnErr = g.burnErr
	}
	return view
}

func (g *Game) phoneComposeView(view phoneView, eng *engine, p games.Player) phoneView {
	w := eng.Writers[p.ID]
	if w == nil {
		view.WaitCopy = "You are watching this round."
		return view
	}
	view.Role = "compose"
	for i, slot := range w.Slots {
		remain := remainingCap(slot.Draft, eng.Policy.Cap)
		view.Slots = append(view.Slots, slotView{
			Index:      i,
			PromptText: slot.Prompt.Text,
			Draft:      slot.Draft,
			Remain:     remain,
			Cap:        eng.Policy.Cap,
			DupOutline: slot.DupOutline,
			Disabled:   w.Locked,
		})
	}
	if w.Locked {
		view.Role = "locked"
		view.Locked = true
		view.LockLabel = "Locked"
		return g.attachBurnDrawer(view, p)
	}
	view.LockReady = composeReady(eng.Policy, w)
	view.LockLabel = "Lock"
	return g.attachBurnDrawer(view, p)
}

func composeReady(p composePolicy, w *writerState) bool {
	for _, slot := range w.Slots {
		text := trimToCap(slot.Draft, p.Cap)
		if text == "" {
			return false
		}
		if bannedHit(text, p.Banned) != "" {
			return false
		}
	}
	return true
}

func remainingCap(draft string, cap int) int {
	if cap <= 0 {
		return 0
	}
	n := cap - len([]rune(draft))
	if n < 0 {
		return 0
	}
	return n
}

func (g *Game) rosterViewsLocked(eng *engine) []rosterView {
	ids := eng.seatedIDs()
	out := make([]rosterView, 0, len(ids))
	for _, id := range ids {
		w := eng.Writers[id]
		if w == nil {
			continue
		}
		out = append(out, rosterView{Name: w.Name, Locked: w.Locked, Score: eng.Scores[id]})
	}
	return out
}

// timerViewLocked is the running timer's label and countdown. Intro beats
// run on the clock but show no timer.
func (g *Game) timerViewLocked(eng *engine) (string, runtimekit.TimerView) {
	switch eng.Timer.Kind {
	case "", timerLastIntro, timerVersusIntro, timerVoteIntro, timerPlayIntro:
		return "", runtimekit.TimerView{}
	case "write":
		return "Write", eng.Timer.View(g.run.Now())
	case timerVote:
		return "Vote", eng.Timer.View(g.run.Now())
	case timerWinner:
		return "Scores", eng.Timer.View(g.run.Now())
	case timerFinal:
		return "Final", eng.Timer.View(g.run.Now())
	}
	return "Timer", eng.Timer.View(g.run.Now())
}
