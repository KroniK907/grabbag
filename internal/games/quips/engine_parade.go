package quips

import (
	"sort"
	"time"
)

func (e *engine) afterWriteClosed(now time.Time, out Outcome) Outcome {
	e.clearTimer()
	if e.Settings.HostControlledReveals {
		e.Phase = phaseParadeWait
		out.Changed = true
		out.Events = appendUniqueEvent(out.Events, eventQuips)
		return out
	}
	return e.startCurrentSegment(now, out)
}

func (e *engine) startCurrentSegment(now time.Time, out Outcome) Outcome {
	segIdx := e.activeSegmentIdx()
	if segIdx < 0 {
		return e.roundComplete(now, out)
	}
	e.clearComposePhoneErr()
	e.Vote = voteState{Picks: map[string]votePick{}}
	e.HoldAwards = nil
	if e.Kind == roundLastQuip {
		if e.Settings.HostControlledReveals {
			e.Revealed = 0
			e.Phase = phaseReveal
		} else {
			e.Revealed = e.lastQuipRevealTarget()
			e.Phase = phaseVote
			e.openVoteTimer(now)
		}
	} else if e.Settings.HostControlledReveals {
		e.Revealed = 0
		e.Phase = phaseReveal
	} else {
		e.Revealed = 2
		e.Phase = phaseVote
		e.openVoteTimer(now)
	}
	out.Changed = true
	out.Events = appendUniqueEvent(out.Events, eventQuips)
	return out
}

func (e *engine) lastQuipRevealTarget() int {
	n := len(e.seatedIDs())
	if n < 1 {
		return 0
	}
	return n
}

func (e *engine) activeSegmentIdx() int {
	if e.Kind == roundLastQuip {
		if len(e.Segments) == 0 {
			return -1
		}
		return 0
	}
	if e.ParadePos >= len(e.ParadeOrder) {
		return -1
	}
	return e.ParadeOrder[e.ParadePos]
}

func (e *engine) segmentWriters(segIdx int) []string {
	if segIdx < 0 || segIdx >= len(e.Segments) {
		return nil
	}
	if e.Kind == roundLastQuip {
		return e.seatedIDs()
	}
	seg := e.Segments[segIdx]
	return []string{seg.A, seg.B}
}

func (e *engine) quipForWriter(segIdx int, writerID string) string {
	w := e.Writers[writerID]
	if w == nil {
		return ""
	}
	for _, slot := range w.Slots {
		if slot.SegmentIdx == segIdx {
			return slot.Final
		}
	}
	return ""
}

func (e *engine) voteTargets(voterID string) []string {
	segIdx := e.activeSegmentIdx()
	writers := e.segmentWriters(segIdx)
	var out []string
	for _, id := range writers {
		if !e.Settings.AllowSelfVote && id == voterID {
			continue
		}
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func (e *engine) doVote(actor string, target string, seated bool, out Outcome) Outcome {
	if e.Phase != phaseVote {
		out.PhoneErr[actor] = "You cannot vote now."
		return out
	}
	allowed := e.voteTargets(actor)
	ok := false
	for _, id := range allowed {
		if id == target {
			ok = true
			break
		}
	}
	if !ok {
		out.PhoneErr[actor] = "That quip is not on the board."
		return out
	}
	if e.Vote.Picks == nil {
		e.Vote.Picks = map[string]votePick{}
	}
	e.Vote.Picks[actor] = votePick{Voter: actor, Target: target, Seated: seated}
	out.Changed = true
	out.Events = appendUniqueEvent(out.Events, eventQuips)
	return out
}

func (e *engine) doHostReveal(out Outcome) Outcome {
	if !e.Settings.HostControlledReveals {
		out.PhoneErr["host"] = "Reveals are automatic."
		return out
	}
	if e.Phase != phaseReveal {
		out.PhoneErr["host"] = "Nothing to reveal now."
		return out
	}
	target := 2
	if e.Kind == roundLastQuip {
		target = e.lastQuipRevealTarget()
	}
	if e.Revealed >= target {
		out.PhoneErr["host"] = "All quips are already revealed."
		return out
	}
	e.Revealed++
	out.Changed = true
	out.Events = appendUniqueEvent(out.Events, eventQuips)
	if e.Revealed >= target {
		e.Phase = phaseVote
	}
	return out
}

func (e *engine) doHostRevealAt(now time.Time, out Outcome) Outcome {
	out = e.doHostReveal(out)
	if e.Phase == phaseVote && e.TimerKind == "" {
		e.openVoteTimer(now)
	}
	return out
}

func (e *engine) doHostNextSegment(now time.Time, out Outcome) Outcome {
	if !e.Settings.HostControlledReveals {
		out.PhoneErr["host"] = "The parade advances on its own."
		return out
	}
	switch e.Phase {
	case phaseParadeWait:
		return e.startCurrentSegment(now, out)
	case phaseHold:
		return e.afterHold(now, out)
	default:
		out.PhoneErr["host"] = "Nothing to advance now."
		return out
	}
}

func (e *engine) doHostSkipHold(now time.Time, out Outcome) Outcome {
	if e.Phase != phaseHold {
		out.PhoneErr["host"] = "No hold to skip."
		return out
	}
	if !e.Settings.HostControlledReveals {
		out.PhoneErr["host"] = "Hold skips are host-only."
		return out
	}
	e.clearTimer()
	return e.afterHold(now, Outcome{Changed: true, Events: []string{eventQuips}})
}

func (e *engine) doHostEndMatch(out Outcome) Outcome {
	if e.Phase != phaseFinalScores {
		out.PhoneErr["host"] = "The match is not on final scores."
		return out
	}
	return e.endMatch(out)
}

func (e *engine) openVoteTimer(now time.Time) {
	e.clearComposePhoneErr()
	if e.Hooks.OnSegmentVoteOpen != nil {
		segIdx := e.activeSegmentIdx()
		if segIdx >= 0 && segIdx < len(e.Segments) {
			e.Hooks.OnSegmentVoteOpen(e.Segments[segIdx].Prompt)
		}
	}
	e.armTimer(now, timerVote, e.Settings.VoteSec)
}

func (e *engine) closeVote(now time.Time, out Outcome) Outcome {
	e.clearTimer()
	awards := tallySegmentPoints(
		e.Vote.Picks,
		e.Settings.SeatedVotePoints,
		e.Settings.AudienceVotePoints,
		e.Multiplier,
	)
	e.HoldAwards = awards
	for id, pts := range awards {
		e.Scores[id] += pts
	}
	e.Phase = phaseHold
	sec := e.Settings.WinnerScreenSec
	if sec <= 0 {
		if !e.Settings.HostControlledReveals {
			return e.afterHold(now, out)
		}
		out.Changed = true
		out.Events = appendUniqueEvent(out.Events, eventQuips)
		return out
	}
	e.armTimer(now, timerWinner, sec)
	out.Changed = true
	out.Events = appendUniqueEvent(out.Events, eventQuips)
	return out
}

func (e *engine) afterHold(now time.Time, out Outcome) Outcome {
	e.clearTimer()
	e.HoldAwards = nil
	if e.Kind == roundLastQuip {
		return e.enterFinalScores(now, out)
	}
	e.ParadePos++
	if e.ParadePos >= len(e.ParadeOrder) {
		return e.roundComplete(now, out)
	}
	return e.startCurrentSegment(now, out)
}

func (e *engine) roundComplete(now time.Time, out Outcome) Outcome {
	if e.Round >= e.Settings.RoundCount {
		return e.enterFinalScores(now, out)
	}
	e.Round++
	if e.Settings.LastQuipEnabled && e.Round == e.Settings.RoundCount {
		e.Kind = roundLastQuip
	} else {
		e.Kind = roundStandard
	}
	e.Multiplier = scoringMultiplier(e.Round, e.Settings.RoundMultiplierIncreaseBy)
	e.ParadePos = 0
	for _, w := range e.Writers {
		w.Locked = false
	}
	if err := e.openWriteRoundForIDs(now); err != nil {
		out.DealShortage = true
		out.Changed = true
		return out
	}
	out.Changed = true
	out.Events = appendUniqueEvent(out.Events, eventQuips)
	return out
}

func (e *engine) enterFinalScores(now time.Time, out Outcome) Outcome {
	e.Phase = phaseFinalScores
	e.Champions = coChampions(e.Scores, e.seatedIDs())
	e.clearTimer()
	if e.Settings.FinalScoresSec > 0 {
		e.armTimer(now, timerFinal, e.Settings.FinalScoresSec)
		out.Changed = true
		out.Events = appendUniqueEvent(out.Events, eventQuips)
		return out
	}
	if !e.Settings.HostControlledReveals {
		return e.endMatch(out)
	}
	out.Changed = true
	out.Events = appendUniqueEvent(out.Events, eventQuips)
	return out
}

func (e *engine) endMatch(out Outcome) Outcome {
	e.Phase = phaseOver
	e.MatchOver = true
	e.clearTimer()
	out.Finish = true
	out.Changed = true
	out.Events = appendUniqueEvent(out.Events, eventQuips)
	return out
}

func (e *engine) liveVoteCounts() map[string]int {
	if !e.Settings.LiveVoteCounts || e.Phase != phaseVote {
		return nil
	}
	counts := map[string]int{}
	for _, pick := range e.Vote.Picks {
		if pick.Target != "" {
			counts[pick.Target]++
		}
	}
	return counts
}

func (e *engine) revealedWriterIDs(segIdx int) []string {
	if e.Kind == roundLastQuip {
		all := e.seatedIDs()
		if e.Revealed >= len(all) {
			return all
		}
		return all[:e.Revealed]
	}
	if e.Revealed <= 0 {
		return nil
	}
	seg := e.Segments[segIdx]
	if e.Revealed == 1 {
		return []string{seg.A}
	}
	return []string{seg.A, seg.B}
}
