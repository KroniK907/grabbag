package runtimekit

import (
	"fmt"
	"time"
)

// Timer is one phase countdown an engine keeps in its own state. It is plain
// data with no goroutine: the engine passes the clock in, and the Runner's
// tick asks the engine whether it has expired. A frozen timer has a zero End
// and keeps its remainder in Left.
type Timer struct {
	// Kind names what fires on expiry. Engines with one timer per phase
	// may leave it empty.
	Kind  string
	End   time.Time
	Total time.Duration
	Left  time.Duration
}

// Arm starts a countdown of d. A d of zero or less clears the timer. When
// paused is true the timer starts frozen and runs from Thaw.
func (t *Timer) Arm(kind string, d time.Duration, now time.Time, paused bool) {
	if d <= 0 {
		t.Clear()
		return
	}
	*t = Timer{Kind: kind, Total: d}
	if paused {
		t.Left = d
		return
	}
	t.End = now.Add(d)
}

// Clear turns the timer off.
func (t *Timer) Clear() { *t = Timer{} }

// On reports a running or frozen countdown.
func (t Timer) On() bool { return t.Total > 0 }

// Frozen reports a countdown held by Freeze.
func (t Timer) Frozen() bool { return t.On() && t.End.IsZero() }

// Freeze holds the remainder. It does nothing to a frozen or cleared timer.
func (t *Timer) Freeze(now time.Time) {
	if !t.On() || t.End.IsZero() {
		return
	}
	t.Left = max(t.End.Sub(now), 0)
	t.End = time.Time{}
}

// Thaw restarts a frozen timer from its remainder.
func (t *Timer) Thaw(now time.Time) {
	if !t.Frozen() {
		return
	}
	t.End = now.Add(t.Left)
	t.Left = 0
}

// Extend adds d to the countdown, frozen or running.
func (t *Timer) Extend(d time.Duration) {
	if !t.On() {
		return
	}
	t.Total += d
	if t.End.IsZero() {
		t.Left += d
		return
	}
	t.End = t.End.Add(d)
}

// Remaining is the time left at now. It is never negative.
func (t Timer) Remaining(now time.Time) time.Duration {
	if !t.On() {
		return 0
	}
	if t.End.IsZero() {
		return t.Left
	}
	return max(t.End.Sub(now), 0)
}

// Expired reports a running timer at or past its end. A frozen timer never
// expires.
func (t Timer) Expired(now time.Time) bool {
	return t.On() && !t.End.IsZero() && !now.Before(t.End)
}

// View is the timer as templates draw it.
func (t Timer) View(now time.Time) TimerView {
	if !t.On() {
		return TimerView{}
	}
	v := TimerView{
		On:           true,
		Kind:         t.Kind,
		Frozen:       t.Frozen(),
		Seconds:      ceilSeconds(t.Remaining(now)),
		TotalSeconds: ceilSeconds(t.Total),
	}
	if !v.Frozen {
		v.EndUnix = t.End.Unix()
		v.EndMs = t.End.UnixMilli()
	}
	return v
}

func ceilSeconds(d time.Duration) int {
	return int((d + time.Second - 1) / time.Second)
}

// TimerView is a Timer for templates. Seconds rounds up, so the last second
// shows 0:01 and not 0:00. EndUnix and EndMs are zero while frozen, so a
// client counts down from End and shows Seconds when frozen. Pass it to the
// ui-countdown template to wire the shared countdown script.
type TimerView struct {
	On           bool
	Kind         string
	Frozen       bool
	EndUnix      int64
	EndMs        int64
	Seconds      int
	TotalSeconds int
}

// Clock is Seconds as m:ss, the text a countdown shows before the script
// runs.
func (v TimerView) Clock() string {
	return fmt.Sprintf("%d:%02d", v.Seconds/60, v.Seconds%60)
}
