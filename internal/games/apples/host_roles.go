package apples

import "github.com/KroniK907/grabbag/internal/games"

func claimedHostID(players []games.Player) string {
	for _, p := range players {
		if p.ClaimedHost {
			return p.ID
		}
	}
	return ""
}

func claimedHostInLive(live map[string]games.Player) (games.Player, bool) {
	for _, p := range live {
		if p.ClaimedHost {
			return p, true
		}
	}
	return games.Player{}, false
}

func (m *matchState) playerReveals(p games.Player) bool {
	if m == nil || m.Phase != phaseReveal {
		return false
	}
	if m.Settings.HostReveals {
		return p.ClaimedHost
	}
	return p.ID == m.JudgeID
}

func (m *matchState) hasUnrevealedPacket() bool {
	for _, pk := range m.Packets {
		if !pk.Revealed {
			return true
		}
	}
	return false
}

func (g *Game) applyHostJudgeLocked(m *matchState, seated []games.Player) {
	if m == nil || !m.Settings.HostJudge {
		return
	}
	id := claimedHostID(seated)
	if id == "" {
		return
	}
	if _, ok := m.Actors[id]; ok {
		m.JudgeID = id
	}
}
