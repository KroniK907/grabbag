package quips

import (
	"fmt"
	"math"
	"net/http"

	"github.com/KroniK907/grabbag/internal/games"
)

type boardView struct {
	pageView
	Paused       bool
	Overlay      bool
	OverlayCopy  string
	Phase        string
	Round        int
	Multiplier   int
	CenterPrompt string
	WritingBeat  bool
	ParadeBeat   bool
	Quips        []quipBoardView
	HoldAwards   []holdAwardView
	Champions    []string
	Roster       []rosterView
	TimerLabel   string
	TimerText    string
	TimerSeconds int
	TimerTotal   int
	TimerEndUnix int64
	LiveCounts   map[string]int
}

type quipBoardView struct {
	WriterID string
	Name     string
	Text     string
	Empty    bool
	Revealed bool
	Count    int
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
	pageView
	Paused         bool
	ClaimedHost    bool
	BurnFaces      []burnFace
	BurnErr        string
	BurnOpen       bool
	Overlay        bool
	OverlayCopy    string
	OverlayYes     bool
	Role           string
	Error          string
	WaitCopy       string
	Slots          []slotView
	VoteOptions    []voteOptionView
	LockLabel      string
	LockReady      bool
	Locked         bool
	Policy         composePolicy
	HostBar        bool
	HostReveal     bool
	HostNext       bool
	HostSkipHold   bool
	HostEndMatch   bool
	TimerLabel     string
	TimerText      string
	TimerSeconds   int
	TimerTotal     int
	TimerEndUnix   int64
}

type voteOptionView struct {
	Target string
	Label  string
	Count  int
	Chosen bool
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
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.boardViewLocked()
}

func (g *Game) boardViewLocked() boardView {
	view := boardView{
		pageView: g.pageViewLocked("Quick Quips"),
		Phase:    "setup",
	}
	if g.overlay != "" {
		view.Overlay = true
		view.OverlayCopy = overlayTVCopy
		if g.overlayTooSmall {
			view.OverlayCopy = overlayFailCopy
		}
	}
	if g.engine == nil {
		return view
	}
	eng := g.engine
	view.Paused = g.paused
	view.Phase = string(eng.Phase)
	view.Round = eng.Round
	view.Multiplier = eng.Multiplier
	view.WritingBeat = eng.Phase == phaseWrite
	view.ParadeBeat = eng.Phase == phaseReveal || eng.Phase == phaseVote || eng.Phase == phaseHold
	view.Roster = g.rosterViewsLocked(eng)
	view.TimerLabel, view.TimerText, view.TimerSeconds, view.TimerTotal, view.TimerEndUnix = g.timerViewLocked(eng)
	view.LiveCounts = eng.liveVoteCounts()

	segIdx := eng.activeSegmentIdx()
	if segIdx >= 0 && segIdx < len(eng.Segments) {
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
		for id, pts := range eng.HoldAwards {
			name := id
			if w := eng.Writers[id]; w != nil {
				name = w.Name
			}
			view.HoldAwards = append(view.HoldAwards, holdAwardView{Name: name, Points: pts})
		}
	}
	if eng.Phase == phaseFinalScores || eng.Phase == phaseOver {
		for _, id := range eng.Champions {
			if w := eng.Writers[id]; w != nil {
				view.Champions = append(view.Champions, w.Name)
			}
		}
	}
	return view
}

func (g *Game) phoneView(r *http.Request) phoneView {
	h := g.helperNow()
	if h == nil {
		return phoneView{pageView: g.pageView("Quick Quips")}
	}
	p, ok, _ := h.PlayerFromRequest(r)
	if !ok {
		return phoneView{pageView: g.pageView("Quick Quips")}
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.phoneViewLockedWithRequest(p, r)
}

func (g *Game) phoneViewLocked(p games.Player) phoneView {
	return g.phoneViewLockedWithRequest(p, nil)
}

func (g *Game) phoneViewLockedWithRequest(p games.Player, r *http.Request) phoneView {
	view := phoneView{
		pageView:     g.pageViewLocked("Quick Quips"),
		Role:         "wait",
		WaitCopy:     "Hang tight.",
		ClaimedHost:  p.ClaimedHost,
	}
	if g.overlay != "" {
		view.Overlay = true
		copy, yes := overlayCopy(p.ClaimedHost, g.overlayTooSmall)
		view.OverlayCopy = copy
		view.OverlayYes = yes
		return view
	}
	if g.engine == nil {
		return view
	}
	eng := g.engine
	view.Paused = g.paused
	view.Policy = eng.Policy
	if msg := eng.PhoneErr[p.ID]; msg != "" {
		view.Error = msg
	}
	if msg := eng.PhoneErr["host"]; msg != "" && r != nil && g.helper != nil && g.helper.HasAdmin(r) {
		view.Error = msg
	}
	view.TimerLabel, view.TimerText, view.TimerSeconds, view.TimerTotal, view.TimerEndUnix = g.timerViewLocked(eng)

	if r != nil && g.helper != nil && g.helper.HasAdmin(r) && eng.Settings.HostControlledReveals {
		view.HostBar = true
		switch eng.Phase {
		case phaseReveal:
			view.HostReveal = true
		case phaseParadeWait:
			view.HostNext = true
		case phaseHold:
			view.HostSkipHold = true
			if eng.Settings.WinnerScreenSec == 0 {
				view.HostNext = true
			}
		case phaseFinalScores:
			view.HostEndMatch = true
		}
	}

	switch eng.Phase {
	case phaseWrite:
		return g.phoneComposeView(view, eng, p)
	case phaseVote:
		view.Role = "vote"
		counts := eng.liveVoteCounts()
		pick := ""
		if eng.Vote.Picks != nil {
			pick = eng.Vote.Picks[p.ID].Target
		}
		for _, id := range eng.voteTargets(p.ID) {
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
			opt := voteOptionView{Target: id, Label: label, Chosen: id == pick}
			if counts != nil {
				opt.Count = counts[id]
			}
			view.VoteOptions = append(view.VoteOptions, opt)
		}
		return g.attachBurnDrawer(view, p)
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
	if p.ClaimedHost && g.started {
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

func (g *Game) timerViewLocked(eng *engine) (label, text string, seconds, total int, endUnix int64) {
	if eng.TimerKind == "" {
		return "", "", 0, 0, 0
	}
	switch eng.TimerKind {
	case "write":
		label = "Write"
	case timerVote:
		label = "Vote"
	case timerWinner:
		label = "Scores"
	case timerFinal:
		label = "Final"
	default:
		label = "Timer"
	}
	total = int(eng.TimerTotal.Seconds())
	if eng.Paused || g.paused {
		left := eng.FrozenLeft
		if left <= 0 && eng.TimerTotal > 0 {
			left = eng.TimerTotal
		}
		seconds = int(math.Ceil(left.Seconds()))
		text = formatTimer(seconds)
		return label, text, seconds, total, 0
	}
	if eng.TimerEnd.IsZero() {
		return label, "", 0, total, 0
	}
	left := eng.TimerEnd.Sub(g.clock())
	if left < 0 {
		left = 0
	}
	seconds = int(math.Ceil(left.Seconds()))
	text = formatTimer(seconds)
	return label, text, seconds, total, eng.TimerEnd.UnixMilli() / 1000
}

func formatTimer(sec int) string {
	return fmt.Sprintf("%d:%02d", sec/60, sec%60)
}
