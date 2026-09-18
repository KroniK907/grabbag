package quips

import (
	"math/rand"
	"time"
)

const (
	eventQuips     = "quips"
	eventQuipsLock = "quips-lock"
)

type enginePhase string

const (
	phaseWrite        enginePhase = "write"
	phaseParadeReady  enginePhase = "parade-ready"
)

type roundKind int

const (
	roundStandard roundKind = iota
	roundLastQuip
)

type roundSegment struct {
	Prompt playPrompt
	A      string
	B      string
}

type composeSlot struct {
	SegmentIdx int
	OpponentID string
	Prompt     playPrompt
	Draft      string
	Final      string
	Locked     bool
	DupOutline bool
}

type writerState struct {
	ID     string
	Name   string
	Slots  []composeSlot
	Locked bool
}

// EngineHooks are journal callbacks injected at Begin (stubs OK until task 89).
type EngineHooks struct {
	OnPromptAssigned  func(libraryID, cardID string)
	OnSegmentVoteOpen func(prompt playPrompt)
}

type CommandKind int

const (
	CmdDraft CommandKind = iota
	CmdLock
)

type Command struct {
	Kind   CommandKind
	Actor  string
	Slot   int
	Text   string
	Drafts []string
}

type Outcome struct {
	Changed  bool
	PhoneErr map[string]string
	Finish   bool
	Pause    bool
	Events   []string
}

type engine struct {
	Settings     matchSettings
	Policy       composePolicy
	Hooks        EngineHooks
	Phase        enginePhase
	Round        int
	Kind         roundKind
	Segments     []roundSegment
	ParadeOrder  []int
	Writers      map[string]*writerState
	PromptPool   []playPrompt
	PhoneErr     map[string]string
	TimerKind    string
	TimerEnd     time.Time
	TimerTotal   time.Duration
	FrozenLeft   time.Duration
	Paused       bool
	rng          *rand.Rand
	now          func() time.Time
}

type rosterRow struct {
	ID   string
	Name string
}

type promptDeal struct {
	Pool []playPrompt
}

func (e *engine) clock(now time.Time) time.Time {
	if e.now != nil {
		return e.now()
	}
	return now
}

func (e *engine) Begin(settings matchSettings, roster []rosterRow, deal promptDeal, hooks EngineHooks, rng *rand.Rand, now time.Time) error {
	e.Settings = settings
	e.Policy = composePolicyFrom(settings)
	e.Hooks = hooks
	e.Phase = phaseWrite
	e.Round = 1
	e.Kind = roundStandard
	if settings.LastQuipEnabled && settings.RoundCount == 1 {
		e.Kind = roundLastQuip
	}
	e.PhoneErr = map[string]string{}
	e.PromptPool = append([]playPrompt(nil), deal.Pool...)
	e.rng = rng
	e.now = func() time.Time { return now }
	e.Writers = map[string]*writerState{}
	for _, row := range roster {
		e.Writers[row.ID] = &writerState{ID: row.ID, Name: row.Name}
	}
	return e.openWriteRoundForIDs(now)
}

func (e *engine) syncRoster(rows []rosterRow) {
	for _, row := range rows {
		if w := e.Writers[row.ID]; w != nil {
			w.Name = row.Name
		}
	}
}

func (e *engine) Do(cmd Command, now time.Time) Outcome {
	out := Outcome{PhoneErr: map[string]string{}}
	if e.Phase != phaseWrite || e.Paused {
		out.PhoneErr[cmd.Actor] = "You cannot compose now."
		return out
	}
	w := e.Writers[cmd.Actor]
	if w == nil {
		out.PhoneErr[cmd.Actor] = "You are not in this match."
		return out
	}
	if w.Locked {
		out.PhoneErr[cmd.Actor] = "You already locked."
		return out
	}
	switch cmd.Kind {
	case CmdDraft:
		return e.doDraft(w, cmd, out)
	case CmdLock:
		return e.doLock(w, cmd, now, out)
	default:
		return out
	}
}

func (e *engine) Advance(now time.Time) Outcome {
	out := Outcome{PhoneErr: map[string]string{}}
	if e.Phase != phaseWrite || e.Paused {
		return out
	}
	if e.TimerKind == "" || e.TimerEnd.IsZero() {
		return out
	}
	if e.clock(now).Before(e.TimerEnd) {
		return out
	}
	e.clearTimer()
	e.timerSubmitAll()
	return e.finishWriteIfReady(now, Outcome{Changed: true, Events: []string{eventQuips}})
}

func (e *engine) SetPaused(paused bool, now time.Time) {
	e.Paused = paused
	if paused {
		e.freezeTimer(now)
		return
	}
	e.thawTimer(now)
}

func (e *engine) allWritersLocked() bool {
	for _, w := range e.Writers {
		if !w.Locked {
			return false
		}
	}
	return len(e.Writers) > 0
}

func (e *engine) finishWriteIfReady(now time.Time, base Outcome) Outcome {
	if !e.writeGateMet() {
		return base
	}
	e.Phase = phaseParadeReady
	e.clearTimer()
	if !base.Changed {
		base.Changed = true
	}
	base.Events = appendUniqueEvent(base.Events, eventQuips)
	return base
}

func (e *engine) writeGateMet() bool {
	for _, w := range e.Writers {
		if !w.Locked {
			return false
		}
		for _, slot := range w.Slots {
			if !slot.Locked {
				return false
			}
		}
	}
	return len(e.Writers) > 0
}

func appendUniqueEvent(events []string, name string) []string {
	for _, e := range events {
		if e == name {
			return events
		}
	}
	return append(events, name)
}
