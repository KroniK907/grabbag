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
	Phase        string
	CenterPrompt string
	WritingBeat  bool
	Roster       []rosterView
	TimerLabel   string
	TimerText    string
	TimerSeconds int
	TimerTotal   int
	TimerEndUnix int64
}

type rosterView struct {
	Name   string
	Locked bool
	Score  int
}

type phoneView struct {
	pageView
	Paused         bool
	Role           string
	Error          string
	WaitCopy       string
	Slots          []slotView
	LockLabel      string
	LockReady      bool
	Locked         bool
	Policy         composePolicy
	TimerLabel     string
	TimerText      string
	TimerSeconds   int
	TimerTotal     int
	TimerEndUnix   int64
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
	if g.engine == nil {
		return view
	}
	eng := g.engine
	view.Paused = g.paused
	view.Phase = string(eng.Phase)
	view.WritingBeat = eng.Phase == phaseWrite
	if eng.Kind == roundLastQuip && len(eng.Segments) == 1 {
		view.CenterPrompt = eng.Segments[0].Prompt.Text
	}
	view.Roster = g.rosterViewsLocked(eng)
	view.TimerLabel, view.TimerText, view.TimerSeconds, view.TimerTotal, view.TimerEndUnix = g.timerViewLocked(eng)
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
	return g.phoneViewLocked(p)
}

func (g *Game) phoneViewLocked(p games.Player) phoneView {
	view := phoneView{
		pageView: g.pageViewLocked("Quick Quips"),
		Role:     "wait",
		WaitCopy: "Writers are working on their quips.",
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
	view.TimerLabel, view.TimerText, view.TimerSeconds, view.TimerTotal, view.TimerEndUnix = g.timerViewLocked(eng)
	if eng.Phase != phaseWrite {
		view.Role = "wait"
		view.WaitCopy = "Vote parade wiring lands in the next task."
		return view
	}
	w := eng.Writers[p.ID]
	if w == nil {
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
		return view
	}
	view.LockReady = composeReady(eng.Policy, w)
	if view.LockReady {
		view.LockLabel = "Lock"
	} else {
		view.LockLabel = "Lock"
	}
	return view
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
		out = append(out, rosterView{Name: w.Name, Locked: w.Locked})
	}
	return out
}

func (g *Game) timerViewLocked(eng *engine) (label, text string, seconds, total int, endUnix int64) {
	if eng.TimerKind == "" {
		return "", "", 0, 0, 0
	}
	label = "Write"
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
