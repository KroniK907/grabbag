package apples

import "time"

func (e *engine) armTimer(kind string, sec int) {
	if sec <= 0 {
		e.clearTimer()
		return
	}
	e.Timer.Kind = kind
	e.Timer.Total = time.Duration(sec) * time.Second
	if e.Paused {
		e.Timer.Left = e.Timer.Total
		e.Timer.End = time.Time{}
		return
	}
	e.Timer.End = e.at.Add(e.Timer.Total)
	e.Timer.Left = 0
}

func (e *engine) clearTimer() {
	e.Timer.Kind = ""
	e.Timer.End = time.Time{}
	e.Timer.Total = 0
	e.Timer.Left = 0
}

func (e *engine) freezeTimer() {
	if e.Timer.Kind == "" || e.Timer.End.IsZero() {
		return
	}
	e.Timer.Left = max(e.Timer.End.Sub(e.at), 0)
	e.Timer.End = time.Time{}
}

func (e *engine) thawTimer() {
	if e.Timer.Kind == "" || e.Timer.Left <= 0 {
		return
	}
	e.Timer.End = e.at.Add(e.Timer.Left)
	e.Timer.Left = 0
}
