package apples

type howtoView struct {
	pageView
	Sheet      bool
	Skip       bool
	Multi      bool
	Voting     bool
	Wildcard   bool
	Multiplier bool
	Timers     bool
}

func (g *Game) howtoView(sheet bool) howtoView {
	s := g.currentSettings()
	view := howtoView{
		pageView:   g.chromeView("How to play"),
		Sheet:      sheet,
		Skip:       s.PromptMode == modeSkip,
		Multi:      s.PromptMode == modeMulti,
		Voting:     s.Voting != voteOff,
		Wildcard:   s.wildcardLibraryOn(),
		Multiplier: s.LastRoundMultiplier > 1,
		Timers:     s.AutoDrawSec > 0 || s.SubmitSec > 0 || s.BetweenRevealSec > 0 || s.JudgePickSec > 0 || s.FavoriteVoteSec > 0 || s.FinishHoldSec > 0,
	}
	g.mu.Lock()
	if g.match != nil {
		ms := g.match.Settings
		view.Skip = ms.PromptMode == modeSkip
		view.Multi = ms.PromptMode == modeMulti
		view.Voting = ms.Voting != voteOff
		view.Wildcard = g.match.WildcardAtStart
		view.Multiplier = ms.LastRoundMultiplier > 1
		view.Timers = ms.AutoDrawSec > 0 || ms.SubmitSec > 0 || ms.BetweenRevealSec > 0 || ms.JudgePickSec > 0 || ms.FavoriteVoteSec > 0 || ms.FinishHoldSec > 0
	}
	g.mu.Unlock()
	return view
}
