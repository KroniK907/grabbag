package apples

import "github.com/KroniK907/grabbag/internal/games/runtimekit"

type howtoView struct {
	runtimekit.Page
	Skip       bool
	Multi      bool
	Voting     bool
	Wildcard   bool
	Multiplier bool
	Timers     bool
}

func (g *Game) howtoView() howtoView {
	s := g.currentSettings()
	view := howtoView{
		Page:       g.chromeView("How to play"),
		Skip:       s.PromptMode == modeSkip,
		Multi:      s.PromptMode == modeMulti,
		Voting:     s.Voting != voteOff,
		Wildcard:   s.wildcardLibraryOn(),
		Multiplier: s.LastRoundMultiplier > 1,
		Timers:     s.AutoDrawSec > 0 || s.SubmitSec > 0 || s.BetweenRevealSec > 0 || s.JudgePickSec > 0 || s.FavoriteVoteSec > 0 || s.FinishHoldSec > 0,
	}
	g.run.Lock()
	if g.run.Engine() != nil {
		ms := g.run.Engine().Settings
		view.Skip = ms.PromptMode == modeSkip
		view.Multi = ms.PromptMode == modeMulti
		view.Voting = ms.Voting != voteOff
		view.Wildcard = g.run.Engine().WildcardAtStart
		view.Multiplier = ms.LastRoundMultiplier > 1
		view.Timers = ms.AutoDrawSec > 0 || ms.SubmitSec > 0 || ms.BetweenRevealSec > 0 || ms.JudgePickSec > 0 || ms.FavoriteVoteSec > 0 || ms.FinishHoldSec > 0
	}
	g.run.Unlock()
	return view
}
