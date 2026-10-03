package runtimekit_test

import (
	"testing"
	"time"

	"github.com/KroniK907/grabbag/internal/games/runtimekit"
)

var t0 = time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)

func TestTimerRunsFreezesAndThaws(t *testing.T) {
	var tm runtimekit.Timer
	tm.Arm("vote", 20*time.Second, t0, false)
	if !tm.On() || tm.Frozen() || tm.Expired(t0.Add(19*time.Second)) {
		t.Fatalf("armed timer: %+v", tm)
	}
	tm.Freeze(t0.Add(5 * time.Second))
	if !tm.Frozen() || tm.Remaining(t0.Add(time.Hour)) != 15*time.Second || tm.Expired(t0.Add(time.Hour)) {
		t.Fatalf("frozen timer: %+v", tm)
	}
	tm.Thaw(t0.Add(time.Minute))
	if tm.Expired(t0.Add(time.Minute+14*time.Second)) || !tm.Expired(t0.Add(time.Minute+15*time.Second)) {
		t.Fatalf("thawed timer: %+v", tm)
	}
}

func TestTimerArmedWhilePausedStartsFrozen(t *testing.T) {
	var tm runtimekit.Timer
	tm.Arm("", 10*time.Second, t0, true)
	if !tm.Frozen() || tm.Remaining(t0) != 10*time.Second {
		t.Fatalf("paused arm: %+v", tm)
	}
	tm.Thaw(t0.Add(time.Minute))
	if tm.End != t0.Add(time.Minute+10*time.Second) {
		t.Fatalf("thaw end %v", tm.End)
	}
}

func TestTimerArmZeroClears(t *testing.T) {
	var tm runtimekit.Timer
	tm.Arm("a", time.Second, t0, false)
	tm.Arm("b", 0, t0, false)
	if tm.On() || tm.Kind != "" {
		t.Fatalf("zero arm left %+v", tm)
	}
}

func TestTimerExtend(t *testing.T) {
	var tm runtimekit.Timer
	tm.Arm("", 10*time.Second, t0, false)
	tm.Extend(30 * time.Second)
	if tm.Remaining(t0) != 40*time.Second || tm.Total != 40*time.Second {
		t.Fatalf("running extend: %+v", tm)
	}
	tm.Freeze(t0)
	tm.Extend(5 * time.Second)
	if tm.Remaining(t0) != 45*time.Second {
		t.Fatalf("frozen extend: %+v", tm)
	}
	var off runtimekit.Timer
	off.Extend(time.Second)
	if off.On() {
		t.Fatal("extend turned on a cleared timer")
	}
}

func TestTimerView(t *testing.T) {
	var tm runtimekit.Timer
	if v := tm.View(t0); v.On {
		t.Fatalf("off view %+v", v)
	}
	tm.Arm("vote", 20*time.Second, t0, false)
	v := tm.View(t0.Add(4600 * time.Millisecond))
	if !v.On || v.Frozen || v.Kind != "vote" || v.Seconds != 16 || v.Clock() != "0:16" || v.TotalSeconds != 20 || v.EndMs != t0.Add(20*time.Second).UnixMilli() {
		t.Fatalf("running view %+v", v)
	}
	tm.Freeze(t0.Add(5 * time.Second))
	v = tm.View(t0.Add(time.Hour))
	if !v.Frozen || v.Seconds != 15 || v.EndUnix != 0 || v.EndMs != 0 {
		t.Fatalf("frozen view %+v", v)
	}
	tm.Arm("", 90*time.Second, t0, true)
	if got := tm.View(t0).Clock(); got != "1:30" {
		t.Fatalf("clock %q", got)
	}
}
