package quips

import (
	"sort"
	"strings"
	"time"
)

func (e *engine) openWriteRoundForIDs(now time.Time) error {
	seated := e.seatedIDs()
	n := len(seated)
	if n < 2 {
		return errNeedTwoPlayers
	}
	if e.Kind == roundLastQuip {
		p, ok := popPrompt(&e.PromptPool)
		if !ok {
			return errOutOfPrompts
		}
		seg := roundSegment{Prompt: p}
		e.Segments = []roundSegment{seg}
		for _, id := range seated {
			e.Writers[id].Slots = []composeSlot{{
				SegmentIdx: 0,
				Prompt:     p,
				Draft:      "",
			}}
		}
		if e.Hooks.OnPromptAssigned != nil {
			e.Hooks.OnPromptAssigned(p.LibraryID, p.CardID)
		}
	} else {
		prompts := make([]playPrompt, n)
		for i := 0; i < n; i++ {
			p, ok := popPrompt(&e.PromptPool)
			if !ok {
				return errOutOfPrompts
			}
			prompts[i] = p
		}
		edges := pairings2Regular(seated, e.rng)
		e.Segments = make([]roundSegment, n)
		for i, edge := range edges {
			e.Segments[i] = roundSegment{Prompt: prompts[i], A: edge[0], B: edge[1]}
			if e.Hooks.OnPromptAssigned != nil {
				e.Hooks.OnPromptAssigned(prompts[i].LibraryID, prompts[i].CardID)
			}
		}
		e.ParadeOrder = make([]int, n)
		for i := range e.ParadeOrder {
			e.ParadeOrder[i] = i
		}
		e.rng.Shuffle(len(e.ParadeOrder), func(i, j int) {
			e.ParadeOrder[i], e.ParadeOrder[j] = e.ParadeOrder[j], e.ParadeOrder[i]
		})
		for _, id := range seated {
			slots := e.slotsForPlayer(id)
			sort.Slice(slots, func(i, j int) bool {
				return slots[i].SegmentIdx < slots[j].SegmentIdx
			})
			e.Writers[id].Slots = slots
		}
	}

	if e.Settings.WriteSec > 0 {
		e.armTimer(now, "write", e.Settings.WriteSec)
	} else {
		e.clearTimer()
	}
	e.Phase = phaseWrite
	return nil
}

func (e *engine) seatedIDs() []string {
	ids := make([]string, 0, len(e.Writers))
	for id := range e.Writers {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func (e *engine) slotsForPlayer(id string) []composeSlot {
	var slots []composeSlot
	for segIdx, seg := range e.Segments {
		switch id {
		case seg.A:
			slots = append(slots, composeSlot{
				SegmentIdx: segIdx,
				OpponentID: seg.B,
				Prompt:     seg.Prompt,
			})
		case seg.B:
			slots = append(slots, composeSlot{
				SegmentIdx: segIdx,
				OpponentID: seg.A,
				Prompt:     seg.Prompt,
			})
		}
	}
	return slots
}

func (e *engine) clearComposePhoneErr() {
	for id := range e.Writers {
		delete(e.PhoneErr, id)
	}
}

func (e *engine) doDraft(w *writerState, cmd Command, out Outcome) Outcome {
	if cmd.Slot < 0 || cmd.Slot >= len(w.Slots) {
		out.PhoneErr[cmd.Actor] = "Unknown quip slot."
		return out
	}
	text := trimToCap(cmd.Text, e.Policy.Cap)
	w.Slots[cmd.Slot].Draft = text
	if strings.TrimSpace(text) != "" {
		if hit := bannedHit(text, e.Policy.Banned); hit != "" {
			out.PhoneErr[cmd.Actor] = e.Policy.bannedAlert(hit)
			out.Changed = true
			return out
		}
	}
	w.Slots[cmd.Slot].DupOutline = false
	delete(e.PhoneErr, cmd.Actor)
	out.Changed = true
	return out
}

func (e *engine) doLock(w *writerState, cmd Command, now time.Time, out Outcome) Outcome {
	for i := range w.Slots {
		w.Slots[i].DupOutline = false
	}
	texts := cmd.Drafts
	if len(texts) == 0 {
		texts = make([]string, len(w.Slots))
		for i := range w.Slots {
			texts[i] = w.Slots[i].Draft
		}
	}
	if len(texts) != len(w.Slots) {
		out.PhoneErr[cmd.Actor] = "Send every quip before locking."
		return out
	}
	var lockedNorms [][]string
	for i := range w.Slots {
		lockedNorms = append(lockedNorms, e.lockedNormsOnSegment(w.Slots[i].SegmentIdx, cmd.Actor))
	}
	var issues []lockIssue
	for i, text := range texts {
		slotIssues := e.Policy.lockIssues([]string{text}, lockedNorms[i])
		for _, iss := range slotIssues {
			iss.Slot = i
			issues = append(issues, iss)
		}
	}
	if len(issues) > 0 {
		for _, iss := range issues {
			if iss.Dup {
				w.Slots[iss.Slot].DupOutline = true
			}
		}
		out.PhoneErr[cmd.Actor] = e.Policy.lockAlert(issues)
		out.Changed = true
		return out
	}
	for i, text := range texts {
		trim := trimToCap(text, e.Policy.Cap)
		w.Slots[i].Final = trim
		w.Slots[i].Draft = trim
		w.Slots[i].Locked = true
	}
	w.Locked = true
	delete(e.PhoneErr, cmd.Actor)
	out.Changed = true
	out.Events = append(out.Events, eventQuipsLock)
	return e.finishWriteIfReady(now, out)
}

func (e *engine) lockedNormsOnSegment(segIdx int, skipID string) []string {
	if segIdx < 0 || segIdx >= len(e.Segments) {
		return nil
	}
	seg := e.Segments[segIdx]
	var out []string
	for _, id := range []string{seg.A, seg.B} {
		if id == skipID {
			continue
		}
		w := e.Writers[id]
		if w == nil {
			continue
		}
		for _, slot := range w.Slots {
			if slot.SegmentIdx != segIdx || !slot.Locked {
				continue
			}
			if n := normalizeCardText(slot.Final); n != "" {
				out = append(out, n)
			}
		}
	}
	return out
}

func (e *engine) timerSubmitAll() {
	for _, w := range e.Writers {
		if w.Locked {
			continue
		}
		for i := range w.Slots {
			if w.Slots[i].Locked {
				continue
			}
			norms := e.lockedNormsOnSegment(w.Slots[i].SegmentIdx, w.ID)
			final := e.Policy.timerSubmit(w.Slots[i].Draft, norms)
			w.Slots[i].Final = final
			w.Slots[i].Draft = w.Slots[i].Draft
			w.Slots[i].Locked = true
		}
		w.Locked = true
	}
}

func (e *engine) armTimer(now time.Time, kind string, sec int) {
	if sec <= 0 {
		e.clearTimer()
		return
	}
	if e.Paused {
		e.TimerKind = kind
		e.TimerTotal = time.Duration(sec) * time.Second
		e.FrozenLeft = e.TimerTotal
		e.TimerEnd = time.Time{}
		return
	}
	e.TimerKind = kind
	e.TimerTotal = time.Duration(sec) * time.Second
	e.TimerEnd = e.clock(now).Add(e.TimerTotal)
	e.FrozenLeft = 0
}

func (e *engine) clearTimer() {
	e.TimerKind = ""
	e.TimerEnd = time.Time{}
	e.TimerTotal = 0
	e.FrozenLeft = 0
}

func (e *engine) freezeTimer(now time.Time) {
	if e.TimerKind == "" || e.TimerEnd.IsZero() {
		return
	}
	left := e.TimerEnd.Sub(e.clock(now))
	if left < 0 {
		left = 0
	}
	e.FrozenLeft = left
	e.TimerEnd = time.Time{}
}

func (e *engine) thawTimer(now time.Time) {
	if e.TimerKind == "" || e.FrozenLeft <= 0 {
		return
	}
	e.TimerEnd = e.clock(now).Add(e.FrozenLeft)
	e.FrozenLeft = 0
}

var (
	errNeedTwoPlayers = errEngineStart("need at least two seated players")
	errOutOfPrompts   = errEngineStart("not enough prompts to deal")
)

type errEngineStart string

func (e errEngineStart) Error() string { return string(e) }
