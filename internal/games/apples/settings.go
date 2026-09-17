package apples

import (
	"encoding/json"
	"strings"
)

const kvMatchSettings = "match-settings"

const (
	modeSingle = "single"
	modeSkip   = "skip"
	modeMulti  = "multi"

	voteOff      = "off"
	voteAudience = "audience"
	voteSeated   = "seated"
	voteBoth     = "both"

	saveWinners = "winners"
	saveAll     = "all"

	keepOff = "off"
	keepTop = "top"
	keepAll = "all"
)

// matchSettings is the game KV document at match-settings.
type matchSettings struct {
	Enabled                  map[string]map[string]bool `json:"enabled"`
	HandSize                 int                        `json:"handSize"`
	WinScore                 int                        `json:"winScore"`
	WinByRounds              bool                       `json:"winByRounds"`
	RoundLimit               int                        `json:"roundLimit"`
	WinnerPoints             int                        `json:"winnerPoints"`
	Fav1                     int                        `json:"fav1"`
	Fav2                     int                        `json:"fav2"`
	Fav3                     int                        `json:"fav3"`
	LastRoundMultiplier      int                        `json:"lastRoundMultiplier"`
	PromptMode               string                     `json:"promptMode"`
	BotCount                 int                        `json:"botCount"`
	Voting                   string                     `json:"voting"`
	LiveVoteCounts           bool                       `json:"liveVoteCounts"`
	AutoDrawSec              int                        `json:"autoDrawSec"`
	SubmitSec                int                        `json:"submitSec"`
	BetweenRevealSec         int                        `json:"betweenRevealSec"`
	JudgePickSec             int                        `json:"judgePickSec"`
	WildcardCap              int                        `json:"wildcardCap"`
	WildcardSave             string                     `json:"wildcardSave"`
	WildcardFavoritesKeep    string                     `json:"wildcardFavoritesKeep"`
	WildcardDealPrevious     bool                       `json:"wildcardDealPrevious"`
	WildcardDuplicateBlock   bool                       `json:"wildcardDuplicateBlock"`
	WildcardBanned           string                     `json:"wildcardBanned"`
	WildcardShowMatchedWord  bool                       `json:"wildcardShowMatchedWord"`
	WildcardJournalDelay     bool                       `json:"wildcardJournalDelay"`
	ReshuffleDiscardOnUnload bool                       `json:"reshuffleDiscardOnUnload"`
}

func factorySettings() matchSettings {
	return matchSettings{
		Enabled:                map[string]map[string]bool{},
		HandSize:               7,
		WinScore:               500,
		RoundLimit:             5,
		WinnerPoints:           100,
		Fav1:                   25,
		Fav2:                   10,
		Fav3:                   5,
		LastRoundMultiplier:    2,
		PromptMode:             modeSkip,
		Voting:                 voteBoth,
		LiveVoteCounts:         true,
		SubmitSec:              45,
		JudgePickSec:           60,
		WildcardCap:            80,
		WildcardSave:           saveWinners,
		WildcardFavoritesKeep:  keepOff,
		WildcardDealPrevious:   true,
		WildcardDuplicateBlock: true,
	}
}

func (s *matchSettings) packOn(libraryID, packID string) bool {
	if s == nil || s.Enabled == nil {
		return false
	}
	return s.Enabled[libraryID][packID]
}

func (s *matchSettings) setPack(libraryID, packID string, on bool) {
	if s.Enabled == nil {
		s.Enabled = map[string]map[string]bool{}
	}
	if s.Enabled[libraryID] == nil {
		s.Enabled[libraryID] = map[string]bool{}
	}
	s.Enabled[libraryID][packID] = on
}

func (s *matchSettings) setAllEnabled(cat catalog, on bool) {
	s.Enabled = map[string]map[string]bool{}
	for _, lib := range cat.Libraries {
		for _, pack := range lib.Packs {
			s.setPack(lib.ID, pack.ID, on)
		}
	}
}

func (s matchSettings) wildcardLibraryOn() bool {
	return s.packOn(wildcardLibraryID, wildcardLibraryID)
}

func parseMatchSettings(raw []byte) (matchSettings, bool) {
	if len(raw) == 0 {
		return matchSettings{}, false
	}
	var s matchSettings
	if err := json.Unmarshal(raw, &s); err != nil {
		return matchSettings{}, false
	}
	if s.Enabled == nil {
		return matchSettings{}, false
	}
	return fillSettingDefaults(s), true
}

func fillSettingDefaults(s matchSettings) matchSettings {
	// Task 1 only stored Enabled. Missing knobs get factory values once.
	if s.HandSize == 0 && s.PromptMode == "" && s.Voting == "" {
		keep := s.Enabled
		s = factorySettings()
		s.Enabled = keep
	}
	return s
}

func reconcileSettings(existing matchSettings, ok bool, cat catalog) matchSettings {
	out := factorySettings()
	if ok {
		out = existing
		out.Enabled = map[string]map[string]bool{}
	}
	for _, lib := range cat.Libraries {
		for _, pack := range lib.Packs {
			if !ok {
				out.setPack(lib.ID, pack.ID, factoryEnabled(lib.ID))
				continue
			}
			if packs := existing.Enabled[lib.ID]; packs != nil {
				if was, known := packs[pack.ID]; known {
					out.setPack(lib.ID, pack.ID, was)
					continue
				}
			}
			out.setPack(lib.ID, pack.ID, false)
		}
	}
	return out
}

func factoryEnabled(libraryID string) bool {
	return isFamilyLibrary(libraryID)
}

func inRange(n, lo, hi int) bool {
	return n >= lo && n <= hi
}

func validPromptMode(v string) bool {
	return v == modeSingle || v == modeSkip || v == modeMulti
}

func validVoting(v string) bool {
	return v == voteOff || v == voteAudience || v == voteSeated || v == voteBoth
}

func validSave(v string) bool {
	return v == saveWinners || v == saveAll
}

func validKeep(v string) bool {
	return v == keepOff || v == keepTop || v == keepAll
}

func clampTimer(n int) bool {
	return inRange(n, 0, 300)
}

func normalizeSpaces(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}
