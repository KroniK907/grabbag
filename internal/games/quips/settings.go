package quips

import (
	"encoding/json"
)

const kvMatchSettings = "match-settings"

// matchSettings is the game KV document at match-settings (GM-043).
type matchSettings struct {
	Enabled                   map[string]map[string]bool `json:"enabled"`
	RoundCount                int                        `json:"roundCount"`
	LastQuipEnabled           bool                       `json:"lastQuipEnabled"`
	RoundMultiplierIncreaseBy int                        `json:"roundMultiplierIncreaseBy"`
	SeatedVotePoints          int                        `json:"seatedVotePoints"`
	AudienceVotePoints        int                        `json:"audienceVotePoints"`
	WriteSec                  int                        `json:"writeSec"`
	VoteSec                   int                        `json:"voteSec"`
	WinnerScreenSec           int                        `json:"winnerScreenSec"`
	FinalScoresSec            int                        `json:"finalScoresSec"`
	HostControlledReveals     bool                       `json:"hostControlledReveals"`
	AllowSelfVote             bool                       `json:"allowSelfVote"`
	LiveVoteCounts            bool                       `json:"liveVoteCounts"`
	QuipCharCap               int                        `json:"quipCharCap"`
	BannedWords               string                     `json:"bannedWords"`
	ShowMatchedWord           bool                       `json:"showMatchedWord"`
	ReshuffleDiscardOnUnload  bool                       `json:"reshuffleDiscardOnUnload"`
}

func factorySettings() matchSettings {
	return matchSettings{
		Enabled:                   map[string]map[string]bool{},
		RoundCount:                3,
		LastQuipEnabled:           true,
		RoundMultiplierIncreaseBy: 1,
		SeatedVotePoints:          100,
		AudienceVotePoints:        10,
		WriteSec:                  75,
		VoteSec:                   30,
		WinnerScreenSec:           5,
		FinalScoresSec:            10,
		HostControlledReveals:     false,
		AllowSelfVote:             false,
		LiveVoteCounts:            false,
		QuipCharCap:               120,
		BannedWords:               "",
		ShowMatchedWord:           false,
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
	// KV that only stored pack enablement gets factory knobs once.
	if s.RoundCount == 0 && s.WriteSec == 0 && s.VoteSec == 0 {
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
				out.setPack(lib.ID, pack.ID, true)
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

func inRange(n, lo, hi int) bool {
	return n >= lo && n <= hi
}

func clampTimer(n int) bool {
	return inRange(n, 0, 300)
}
