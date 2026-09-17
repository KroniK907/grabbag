package apples

import "encoding/json"

const kvMatchSettings = "match-settings"

type matchSettings struct {
	Enabled map[string]map[string]bool `json:"enabled"`
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

func (s *matchSettings) setAll(cat catalog, on bool) {
	s.Enabled = map[string]map[string]bool{}
	for _, lib := range cat.Libraries {
		for _, pack := range lib.Packs {
			s.setPack(lib.ID, pack.ID, on)
		}
	}
}

func parseMatchSettings(raw []byte) (matchSettings, bool) {
	var s matchSettings
	if len(raw) == 0 {
		return matchSettings{}, false
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		return matchSettings{}, false
	}
	if s.Enabled == nil {
		return matchSettings{}, false
	}
	return s, true
}

func reconcileSettings(existing matchSettings, ok bool, cat catalog) matchSettings {
	out := matchSettings{Enabled: map[string]map[string]bool{}}
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
