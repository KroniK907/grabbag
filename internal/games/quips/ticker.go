package quips

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
				g.fireEngineLocked()
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

func (g *Game) fireEngineLocked() {
	if g.engine == nil || !g.started || g.paused {
		return
	}
	out := g.engine.Advance(g.clock())
	g.applyOutcomeLocked(out)
	g.flushHostLocked()
}
