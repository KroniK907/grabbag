package apples

import "time"

func (g *Game) startTickerLocked() {
	g.stopTickerLocked()
	stop := make(chan struct{})
	g.stopTick = stop
	go func() {
		tick := time.NewTicker(200 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-stop:
				return
			case <-tick.C:
				g.mu.Lock()
				g.fireTimerLocked()
				g.mu.Unlock()
			}
		}
	}()
}

func (g *Game) stopTickerLocked() {
	if g.stopTick != nil {
		close(g.stopTick)
		g.stopTick = nil
	}
}

func (g *Game) fireTimerLocked() {
	if g.engine == nil || g.paused {
		return
	}
	g.seatLocked()
	out := g.engine.Advance(g.clock())
	if !out.Publish {
		return
	}
	g.applyOutcomeLocked(out)
	g.flushHostLocked()
}

// holdLocked pauses the engine clock while the host is paused or the
// reshuffle overlay is up.
func (g *Game) holdLocked() {
	if g.engine != nil {
		g.engine.SetPaused(g.paused || g.overlayActive(), g.clock())
	}
}
