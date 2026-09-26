package apples

import (
	"slices"
)

// afterConfirm decides what follows a scored round: the next judge, the
// multiplier round, sudden death on a tied lead, or the end of the match.
func (e *engine) afterConfirm() {
	finishNow := false
	switch {
	case e.Settings.WinByRounds:
		if e.Round >= e.Settings.RoundLimit {
			finishNow = true
		} else {
			e.InMultiplier = e.Round == e.Settings.RoundLimit-1 && e.Settings.LastRoundMultiplier > 1
		}
	case e.PendingFinish:
		finishNow = true
	case e.anyReached(e.Settings.WinScore):
		if e.Settings.LastRoundMultiplier <= 1 {
			finishNow = true
		} else {
			e.PendingFinish = true
			e.InMultiplier = true
		}
	}
	if finishNow && e.leadTied() {
		e.enterSudden()
		return
	}
	if finishNow {
		e.finish()
		return
	}
	e.advanceJudge()
	e.Phase = phaseDrawWait
	e.armTimer(timerAutoDraw, e.Settings.AutoDrawSec)
}

// finish shows the winner until the finish hold elapses or the host moves on.
func (e *engine) finish() {
	e.Phase = phaseOver
	e.armTimer(timerFinish, e.Settings.FinishHoldSec)
}

func (e *engine) anyReached(score int) bool {
	for _, a := range e.Actors {
		if a.Score >= score {
			return true
		}
	}
	return false
}

func (e *engine) topScore() int {
	top := -1
	for _, a := range e.Actors {
		top = max(top, a.Score)
	}
	return top
}

func (e *engine) leadTied() bool {
	top := e.topScore()
	n := 0
	for _, a := range e.Actors {
		if a.Score == top {
			n++
		}
	}
	return n > 1 && top >= 0
}

// enterSudden plays one more round among the tied leaders. The judge is a
// random live non-tied player; with none, the round auto-picks.
func (e *engine) enterSudden() {
	top := e.topScore()
	var ties []string
	for _, a := range e.Actors {
		if a.Score == top {
			ties = append(ties, a.ID)
		}
	}
	e.TieIDs = ties
	e.SuddenDeath = true
	e.Phase = phaseSudden
	e.InMultiplier = true
	var pool []string
	for id := range e.Live {
		if !slices.Contains(ties, id) {
			pool = append(pool, id)
		}
	}
	slices.Sort(pool)
	e.JudgeID = ""
	if len(pool) > 0 {
		e.JudgeID = pool[e.intn(len(pool))]
	}
}

// advanceJudge passes the judge seat to the next live player in the cycle.
func (e *engine) advanceJudge() {
	for range e.JudgeCycle {
		e.JudgeIdx = (e.JudgeIdx + 1) % len(e.JudgeCycle)
		if id := e.JudgeCycle[e.JudgeIdx]; e.Live[id] {
			e.JudgeID = id
			return
		}
	}
	if len(e.JudgeCycle) > 0 {
		e.JudgeID = ""
	}
}
