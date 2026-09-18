package quips

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/KroniK907/grabbag/internal/games"
)

type settingsErr struct {
	Field string
	Msg   string
}

func formEnabled(r *http.Request) bool {
	for _, v := range r.Form["enabled"] {
		if v == "1" {
			return true
		}
	}
	return false
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

func (g *Game) matchFrozen() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.started
}

func (g *Game) mutateSettings(w http.ResponseWriter, r *http.Request, field string, apply func(*matchSettings) error) {
	h := g.helperNow()
	if h == nil {
		http.NotFound(w, r)
		return
	}
	if g.matchFrozen() {
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
	g.writeSettingsOK(w, r)
}

func (g *Game) loadSettings(h games.Helper) (matchSettings, bool) {
	raw, found, err := h.KVGet(kvMatchSettings)
	if err != nil || !found {
		return matchSettings{}, false
	}
	return parseMatchSettings(raw)
}

func (g *Game) saveSettings(h games.Helper, s matchSettings) error {
	raw, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return h.KVSet(kvMatchSettings, raw)
}

func (g *Game) persistSettings() error {
	h := g.helperNow()
	if h == nil {
		return fmt.Errorf("game is not loaded")
	}
	cat := scanDataDir(h.DataDir())
	existing, ok := g.loadSettings(h)
	next := reconcileSettings(existing, ok, cat)
	return g.saveSettings(h, next)
}

func (g *Game) postRoundCount(w http.ResponseWriter, r *http.Request) {
	g.mutateSettings(w, r, "round-count", func(s *matchSettings) error {
		n, err := atoiForm(r, "round_count")
		if err != nil || !inRange(n, 1, 20) {
			return fmt.Errorf("Round count is 1 to 20.")
		}
		s.RoundCount = n
		return nil
	})
}

func (g *Game) postLastQuip(w http.ResponseWriter, r *http.Request) {
	g.mutateSettings(w, r, "last-quip", func(s *matchSettings) error {
		s.LastQuipEnabled = formEnabled(r)
		return nil
	})
}

func (g *Game) postRoundMultiplierIncrease(w http.ResponseWriter, r *http.Request) {
	g.mutateSettings(w, r, "round-multiplier-increase", func(s *matchSettings) error {
		n, err := atoiForm(r, "round_multiplier_increase")
		if err != nil || !inRange(n, 0, 3) {
			return fmt.Errorf("Round multiplier increase is 0 to 3.")
		}
		s.RoundMultiplierIncreaseBy = n
		return nil
	})
}

func (g *Game) postSeatedVotePoints(w http.ResponseWriter, r *http.Request) {
	g.mutateSettings(w, r, "seated-vote-points", func(s *matchSettings) error {
		n, err := atoiForm(r, "seated_vote_points")
		if err != nil || !inRange(n, 0, 1000) {
			return fmt.Errorf("Seated vote points are 0 to 1000.")
		}
		s.SeatedVotePoints = n
		return nil
	})
}

func (g *Game) postAudienceVotePoints(w http.ResponseWriter, r *http.Request) {
	g.mutateSettings(w, r, "audience-vote-points", func(s *matchSettings) error {
		n, err := atoiForm(r, "audience_vote_points")
		if err != nil || !inRange(n, 0, 1000) {
			return fmt.Errorf("Audience vote points are 0 to 1000.")
		}
		s.AudienceVotePoints = n
		return nil
	})
}

func (g *Game) postTimerWrite(w http.ResponseWriter, r *http.Request) {
	g.postTimer(w, r, "timer-write", "write")
}

func (g *Game) postTimerVote(w http.ResponseWriter, r *http.Request) {
	g.postTimer(w, r, "timer-vote", "vote")
}

func (g *Game) postTimerWinnerScreen(w http.ResponseWriter, r *http.Request) {
	g.postTimer(w, r, "timer-winner-screen", "winner_screen")
}

func (g *Game) postTimerFinalScores(w http.ResponseWriter, r *http.Request) {
	g.postTimer(w, r, "timer-final-scores", "final_scores")
}

func (g *Game) postTimer(w http.ResponseWriter, r *http.Request, field, name string) {
	g.mutateSettings(w, r, field, func(s *matchSettings) error {
		n, err := atoiForm(r, name)
		if err != nil || !clampTimer(n) {
			return fmt.Errorf("Timers are 0 to 300 seconds.")
		}
		switch name {
		case "write":
			s.WriteSec = n
		case "vote":
			s.VoteSec = n
		case "winner_screen":
			s.WinnerScreenSec = n
		case "final_scores":
			s.FinalScoresSec = n
		}
		return nil
	})
}

func (g *Game) postHostControlledReveals(w http.ResponseWriter, r *http.Request) {
	g.mutateSettings(w, r, "host-controlled-reveals", func(s *matchSettings) error {
		s.HostControlledReveals = formEnabled(r)
		return nil
	})
}

func (g *Game) postAllowSelfVote(w http.ResponseWriter, r *http.Request) {
	g.mutateSettings(w, r, "allow-self-vote", func(s *matchSettings) error {
		s.AllowSelfVote = formEnabled(r)
		return nil
	})
}

func (g *Game) postLiveCounts(w http.ResponseWriter, r *http.Request) {
	g.mutateSettings(w, r, "live-counts", func(s *matchSettings) error {
		s.LiveVoteCounts = formEnabled(r)
		return nil
	})
}

func (g *Game) postQuipCharCap(w http.ResponseWriter, r *http.Request) {
	g.mutateSettings(w, r, "quip-char-cap", func(s *matchSettings) error {
		n, err := atoiForm(r, "quip_char_cap")
		if err != nil || !inRange(n, 8, 200) {
			return fmt.Errorf("Quip character cap is 8 to 200.")
		}
		s.QuipCharCap = n
		return nil
	})
}

func (g *Game) postBannedWords(w http.ResponseWriter, r *http.Request) {
	g.mutateSettings(w, r, "banned-words", func(s *matchSettings) error {
		s.BannedWords = r.FormValue("banned_words")
		return nil
	})
}

func (g *Game) postShowMatchedWord(w http.ResponseWriter, r *http.Request) {
	g.mutateSettings(w, r, "show-matched-word", func(s *matchSettings) error {
		s.ShowMatchedWord = formEnabled(r)
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
	g.stripBurnedFromPoolLocked()
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
