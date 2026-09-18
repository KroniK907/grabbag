package apples

import (
	"fmt"
	"net/http"
	"strconv"
)

type settingsErr struct {
	Field string
	Msg   string
}

func (g *Game) writeSettingsOK(w http.ResponseWriter, r *http.Request) {
	if hxRequest(r) {
		g.render(w, "settings.html", g.settingsView(settingsErr{}), http.StatusOK)
		return
	}
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

func (g *Game) writeSettingsRefuse(w http.ResponseWriter, field, msg string) {
	g.render(w, "settings.html", g.settingsView(settingsErr{Field: field, Msg: msg}), http.StatusOK)
}

func (g *Game) mutateSettings(w http.ResponseWriter, r *http.Request, field string, apply func(*matchSettings) error) {
	h := g.helperNow()
	if h == nil {
		http.NotFound(w, r)
		return
	}
	if g.matchFrozen() && !(wildcardPolicyField(field) && g.wildcardPolicyLive()) {
		g.writeSettingsRefuse(w, field, "This match is running. Setup is locked.")
		return
	}
	g.mu.Lock()
	wildcardLive := g.match != nil && g.match.WildcardAtStart
	g.mu.Unlock()
	if g.matchFrozen() && wildcardPolicyField(field) && !wildcardLive {
		g.writeSettingsRefuse(w, field, "This match is running. Setup is locked.")
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Could not read the form.", http.StatusBadRequest)
		return
	}
	cat := scanDataDir(h.DataDir())
	settings, ok := g.loadSettings(h)
	settings = reconcileSettings(settings, ok, cat)
	if err := apply(&settings); err != nil {
		g.writeSettingsRefuse(w, field, err.Error())
		return
	}
	if err := g.saveSettings(h, settings); err != nil {
		g.writeSettingsRefuse(w, field, "Could not save that setting.")
		return
	}
	if wildcardPolicyField(field) {
		g.mu.Lock()
		if g.match != nil {
			g.match.Settings = settings
		}
		g.mu.Unlock()
	}
	g.writeSettingsOK(w, r)
}

func (g *Game) wildcardPolicyLive() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.match != nil && g.match.WildcardAtStart
}

func wildcardPolicyField(field string) bool {
	switch field {
	case "wildcard-cap", "wildcard-save", "wildcard-favorites", "wildcard-deal-previous",
		"wildcard-duplicate", "wildcard-banned", "wildcard-show-word", "wildcard-journal-delay":
		return true
	default:
		return false
	}
}

func (g *Game) postHandSize(w http.ResponseWriter, r *http.Request) {
	g.mutateSettings(w, r, "hand-size", func(s *matchSettings) error {
		n, err := atoiForm(r, "hand_size")
		if err != nil || !inRange(n, 3, 12) {
			return fmt.Errorf("Hand size is 3 to 12.")
		}
		s.HandSize = n
		return nil
	})
}

func (g *Game) postWinScore(w http.ResponseWriter, r *http.Request) {
	g.mutateSettings(w, r, "win-score", func(s *matchSettings) error {
		n, err := atoiForm(r, "win_score")
		if err != nil || !inRange(n, 100, 10000) {
			return fmt.Errorf("Score to win is 100 to 10000.")
		}
		s.WinScore = n
		return nil
	})
}

func (g *Game) postWinByRounds(w http.ResponseWriter, r *http.Request) {
	g.mutateSettings(w, r, "win-by-rounds", func(s *matchSettings) error {
		s.WinByRounds = formEnabled(r)
		return nil
	})
}

func (g *Game) postRoundLimit(w http.ResponseWriter, r *http.Request) {
	g.mutateSettings(w, r, "round-limit", func(s *matchSettings) error {
		n, err := atoiForm(r, "round_limit")
		if err != nil || !inRange(n, 1, 20) {
			return fmt.Errorf("Rounds is 1 to 20.")
		}
		s.RoundLimit = n
		return nil
	})
}

func (g *Game) postWinnerPoints(w http.ResponseWriter, r *http.Request) {
	g.mutateSettings(w, r, "winner-points", func(s *matchSettings) error {
		n, err := atoiForm(r, "winner_points")
		if err != nil || !inRange(n, 0, 1000) {
			return fmt.Errorf("Winner points are 0 to 1000.")
		}
		s.WinnerPoints = n
		return nil
	})
}

func (g *Game) postFav1(w http.ResponseWriter, r *http.Request) {
	g.postFav(w, r, "fav-1", "fav_1", func(s *matchSettings, n int) { s.Fav1 = n })
}
func (g *Game) postFav2(w http.ResponseWriter, r *http.Request) {
	g.postFav(w, r, "fav-2", "fav_2", func(s *matchSettings, n int) { s.Fav2 = n })
}
func (g *Game) postFav3(w http.ResponseWriter, r *http.Request) {
	g.postFav(w, r, "fav-3", "fav_3", func(s *matchSettings, n int) { s.Fav3 = n })
}

func (g *Game) postFav(w http.ResponseWriter, r *http.Request, field, name string, set func(*matchSettings, int)) {
	g.mutateSettings(w, r, field, func(s *matchSettings) error {
		n, err := atoiForm(r, name)
		if err != nil || !inRange(n, 0, 1000) {
			return fmt.Errorf("Favorite points are 0 to 1000.")
		}
		set(s, n)
		return nil
	})
}

func (g *Game) postMultiplier(w http.ResponseWriter, r *http.Request) {
	g.mutateSettings(w, r, "multiplier", func(s *matchSettings) error {
		n, err := atoiForm(r, "multiplier")
		if err != nil || !inRange(n, 1, 5) {
			return fmt.Errorf("Last-round multiplier is 1 to 5.")
		}
		s.LastRoundMultiplier = n
		return nil
	})
}

func (g *Game) postPromptMode(w http.ResponseWriter, r *http.Request) {
	g.mutateSettings(w, r, "prompt-mode", func(s *matchSettings) error {
		v := r.FormValue("prompt_mode")
		if !validPromptMode(v) {
			return fmt.Errorf("Prompt mode is Single, Skip, or Multi.")
		}
		s.PromptMode = v
		return nil
	})
}

func (g *Game) postBotCount(w http.ResponseWriter, r *http.Request) {
	g.mutateSettings(w, r, "bot-count", func(s *matchSettings) error {
		n, err := atoiForm(r, "bot_count")
		if err != nil || !inRange(n, 0, 10) {
			return fmt.Errorf("Bot count is 0 to 10.")
		}
		s.BotCount = n
		return nil
	})
}

func (g *Game) postVoting(w http.ResponseWriter, r *http.Request) {
	g.mutateSettings(w, r, "voting", func(s *matchSettings) error {
		v := r.FormValue("voting")
		if !validVoting(v) {
			return fmt.Errorf("Voting is Off, audience, seated, or both.")
		}
		s.Voting = v
		return nil
	})
}

func (g *Game) postLiveCounts(w http.ResponseWriter, r *http.Request) {
	g.mutateSettings(w, r, "live-counts", func(s *matchSettings) error {
		s.LiveVoteCounts = formEnabled(r)
		return nil
	})
}

func (g *Game) postAutoDraw(w http.ResponseWriter, r *http.Request) {
	g.postTimer(w, r, "timer-auto-draw", "auto_draw")
}
func (g *Game) postSubmitSec(w http.ResponseWriter, r *http.Request) {
	g.postTimer(w, r, "timer-submit", "submit")
}
func (g *Game) postBetweenSec(w http.ResponseWriter, r *http.Request) {
	g.postTimer(w, r, "timer-between", "between")
}
func (g *Game) postJudgePickSec(w http.ResponseWriter, r *http.Request) {
	g.postTimer(w, r, "timer-judge-pick", "judge_pick")
}
func (g *Game) postFavoriteVoteSec(w http.ResponseWriter, r *http.Request) {
	g.postTimer(w, r, "timer-favorite-vote", "favorite_vote")
}
func (g *Game) postFinishHoldSec(w http.ResponseWriter, r *http.Request) {
	g.postTimer(w, r, "timer-finish-hold", "finish_hold")
}

func (g *Game) postTimer(w http.ResponseWriter, r *http.Request, field, name string) {
	g.mutateSettings(w, r, field, func(s *matchSettings) error {
		n, err := atoiForm(r, name)
		if err != nil || !clampTimer(n) {
			return fmt.Errorf("Timers are 0 to 300 seconds.")
		}
		switch name {
		case "auto_draw":
			s.AutoDrawSec = n
		case "submit":
			s.SubmitSec = n
		case "between":
			s.BetweenRevealSec = n
		case "judge_pick":
			s.JudgePickSec = n
		case "favorite_vote":
			s.FavoriteVoteSec = n
		case "finish_hold":
			s.FinishHoldSec = n
		}
		return nil
	})
}

func (g *Game) postWildcardCap(w http.ResponseWriter, r *http.Request) {
	g.mutateSettings(w, r, "wildcard-cap", func(s *matchSettings) error {
		n, err := atoiForm(r, "wildcard_cap")
		if err != nil || !inRange(n, 1, 500) {
			return fmt.Errorf("Character cap is 1 to 500.")
		}
		s.WildcardCap = n
		return nil
	})
}

func (g *Game) postWildcardSave(w http.ResponseWriter, r *http.Request) {
	g.mutateSettings(w, r, "wildcard-save", func(s *matchSettings) error {
		v := r.FormValue("wildcard_save")
		if !validSave(v) {
			return fmt.Errorf("Save is judge winners or all.")
		}
		s.WildcardSave = v
		return nil
	})
}

func (g *Game) postWildcardFavorites(w http.ResponseWriter, r *http.Request) {
	g.mutateSettings(w, r, "wildcard-favorites", func(s *matchSettings) error {
		v := r.FormValue("wildcard_favorites")
		if !validKeep(v) {
			return fmt.Errorf("Favorites keep is Off, top, or all.")
		}
		s.WildcardFavoritesKeep = v
		return nil
	})
}

func (g *Game) postDealPrevious(w http.ResponseWriter, r *http.Request) {
	g.mutateSettings(w, r, "wildcard-deal-previous", func(s *matchSettings) error {
		s.WildcardDealPrevious = formEnabled(r)
		return nil
	})
}

func (g *Game) postDuplicateBlock(w http.ResponseWriter, r *http.Request) {
	g.mutateSettings(w, r, "wildcard-duplicate", func(s *matchSettings) error {
		s.WildcardDuplicateBlock = formEnabled(r)
		return nil
	})
}

func (g *Game) postBanned(w http.ResponseWriter, r *http.Request) {
	g.mutateSettings(w, r, "wildcard-banned", func(s *matchSettings) error {
		s.WildcardBanned = r.FormValue("wildcard_banned")
		return nil
	})
}

func (g *Game) postShowWord(w http.ResponseWriter, r *http.Request) {
	g.mutateSettings(w, r, "wildcard-show-word", func(s *matchSettings) error {
		s.WildcardShowMatchedWord = formEnabled(r)
		return nil
	})
}

func (g *Game) postJournalDelay(w http.ResponseWriter, r *http.Request) {
	g.mutateSettings(w, r, "wildcard-journal-delay", func(s *matchSettings) error {
		s.WildcardJournalDelay = formEnabled(r)
		return nil
	})
}

func (g *Game) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	h := g.helperNow()
	if h == nil {
		http.NotFound(w, r)
		return false
	}
	if !h.HasAdmin(r) {
		http.Error(w, "Admin session required.", http.StatusUnauthorized)
		return false
	}
	return true
}

func (g *Game) postUnburn(w http.ResponseWriter, r *http.Request) {
	if !g.requireAdmin(w, r) {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Could not read the form.", http.StatusBadRequest)
		return
	}
	g.mu.Lock()
	if g.burnCorrupt {
		g.mu.Unlock()
		g.writeSettingsRefuse(w, "unburn", "Burn list is unreadable")
		return
	}
	g.unburnPairLocked(r.FormValue("kind"), r.FormValue("text"))
	if g.match != nil {
		g.stripBurnedFromPilesLocked(g.match)
		g.rebuildUnplayedLocked(g.match)
	}
	g.mu.Unlock()
	g.writeSettingsOK(w, r)
}

func (g *Game) postReshuffleOnUnload(w http.ResponseWriter, r *http.Request) {
	g.mutateSettings(w, r, "reshuffle-on-unload", func(s *matchSettings) error {
		s.ReshuffleDiscardOnUnload = formEnabled(r)
		return nil
	})
}

func (g *Game) postReshuffleDiscard(w http.ResponseWriter, r *http.Request) {
	if !g.requireAdmin(w, r) {
		return
	}
	if g.matchFrozen() {
		g.writeSettingsRefuse(w, "reshuffle-discard", "This match is running. Setup is locked.")
		return
	}
	g.mu.Lock()
	if g.discardCorrupt {
		g.mu.Unlock()
		g.writeSettingsRefuse(w, "reshuffle-discard", "Discard list is unreadable")
		return
	}
	if len(g.played) == 0 {
		g.mu.Unlock()
		g.writeSettingsRefuse(w, "reshuffle-discard", "")
		return
	}
	g.wipeDiscardLocked()
	g.drainJournalsLocked()
	g.mu.Unlock()
	g.writeSettingsOK(w, r)
}

func atoiForm(r *http.Request, name string) (int, error) {
	return strconv.Atoi(r.FormValue(name))
}
