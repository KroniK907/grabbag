package apples

import "time"

func (e *engine) armTimer(kind string, sec int) {
	if sec <= 0 {
		e.clearTimer()
		return
	}
	e.TimerKind = kind
	e.TimerTotal = time.Duration(sec) * time.Second
	if e.Paused {
		e.FrozenLeft = e.TimerTotal
		e.TimerEnd = time.Time{}
		return
	}
	e.TimerEnd = e.at.Add(e.TimerTotal)
	e.FrozenLeft = 0
}

func (e *engine) clearTimer() {
	e.TimerKind = ""
	e.TimerEnd = time.Time{}
	e.TimerTotal = 0
	e.FrozenLeft = 0
}

func (e *engine) freezeTimer() {
	if e.TimerKind == "" || e.TimerEnd.IsZero() {
		return
	}
	e.FrozenLeft = max(e.TimerEnd.Sub(e.at), 0)
	e.TimerEnd = time.Time{}
}

func (e *engine) thawTimer() {
	if e.TimerKind == "" || e.FrozenLeft <= 0 {
		return
	}
	e.TimerEnd = e.at.Add(e.FrozenLeft)
	e.FrozenLeft = 0
}
