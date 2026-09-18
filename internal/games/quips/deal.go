package quips

import (
	"strings"
)

type playPrompt struct {
	LibraryID string
	CardID    string
	Text      string
}

func (p playPrompt) key() string {
	return p.LibraryID + "\x00" + p.CardID
}

type promptPiles struct {
	Prompts []playPrompt
}

func buildPromptPiles(cat catalog, settings matchSettings) promptPiles {
	var out promptPiles
	for _, lib := range cat.Libraries {
		for _, pack := range lib.Packs {
			if !settings.packOn(lib.ID, pack.ID) {
				continue
			}
			for _, card := range pack.Prompts {
				out.Prompts = append(out.Prompts, playPrompt{
					LibraryID: lib.ID,
					CardID:    card.ID,
					Text:      card.Text,
				})
			}
		}
	}
	return out
}

func filterPlayed(piles promptPiles, played map[string]playedRow) promptPiles {
	if len(played) == 0 {
		return piles
	}
	out := promptPiles{Prompts: make([]playPrompt, 0, len(piles.Prompts))}
	for _, p := range piles.Prompts {
		if _, spent := played[p.key()]; spent {
			continue
		}
		out.Prompts = append(out.Prompts, p)
	}
	return out
}

func popPrompt(pool *[]playPrompt) (playPrompt, bool) {
	if pool == nil || len(*pool) == 0 {
		return playPrompt{}, false
	}
	n := len(*pool) - 1
	p := (*pool)[n]
	*pool = (*pool)[:n]
	return p, true
}

func normalizeCardText(text string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(text))), " ")
}
