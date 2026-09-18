package apples

import (
	"fmt"
	"strings"
)

const (
	startRefuseMsg = "Not enough cards in your library to start the game"
	startUnburnMsg = "You may need to unburn some cards or get more libraries to continue."
	blankCardID    = "__wildcard-blank__"
)

type playCard struct {
	LibraryID string
	CardID    string
	Text      string
	Wildcard  bool
	Blank     bool
}

func (c playCard) key() string {
	return c.LibraryID + "\x00" + c.CardID
}

type playPrompt struct {
	LibraryID string
	CardID    string
	Text      string
	Pick      int
}

func (p playPrompt) key() string {
	return p.LibraryID + "\x00" + p.CardID
}

func (p playPrompt) dealable(handSize int) bool {
	return p.Pick >= 1 && p.Pick <= handSize
}

type enabledPiles struct {
	Prompts []playPrompt
	Answers []playCard
}

func buildPiles(cat catalog, settings matchSettings) enabledPiles {
	var out enabledPiles
	for _, lib := range cat.Libraries {
		for _, pack := range lib.Packs {
			if !settings.packOn(lib.ID, pack.ID) {
				continue
			}
			for _, card := range pack.Prompts {
				if !settings.cardTagsAllowed(card.Tags) {
					continue
				}
				pick := 1
				if card.Pick != nil {
					pick = *card.Pick
				}
				out.Prompts = append(out.Prompts, playPrompt{
					LibraryID: lib.ID, CardID: card.ID, Text: card.Text, Pick: pick,
				})
			}
			for _, card := range pack.Answers {
				if !settings.cardTagsAllowed(card.Tags) {
					continue
				}
				out.Answers = append(out.Answers, playCard{
					LibraryID: lib.ID, CardID: card.ID, Text: card.Text,
				})
			}
		}
	}
	return out
}

func dealablePromptCount(piles enabledPiles, handSize int) int {
	n := 0
	for _, p := range piles.Prompts {
		if p.dealable(handSize) {
			n++
		}
	}
	return n
}

func openingNeeds(settings matchSettings, seatedHumans int) (prompts, answers int) {
	answers = settings.HandSize * seatedHumans
	prompts = 1
	if settings.PromptMode == modeMulti {
		prompts = 2
	}
	return
}

func startShortage(piles enabledPiles, settings matchSettings, seatedHumans int) string {
	if seatedHumans < 1 {
		return ""
	}
	needP, needA := openingNeeds(settings, seatedHumans)
	if dealablePromptCount(piles, settings.HandSize) < needP || len(piles.Answers) < needA {
		return startRefuseMsg
	}
	return ""
}

func burnedWouldHelp(raw, burned enabledPiles, settings matchSettings, seatedHumans int) bool {
	return startShortage(burned, settings, seatedHumans) != "" && startShortage(raw, settings, seatedHumans) == ""
}

func shortageLines(piles enabledPiles, settings matchSettings, seatedHumans int, burnedWouldHelp bool) []string {
	if startShortage(piles, settings, seatedHumans) == "" {
		return nil
	}
	lines := []string{startRefuseMsg}
	if burnedWouldHelp {
		lines = append(lines, startUnburnMsg)
	}
	return lines
}

func promptPick(p *playPrompt) int {
	if p == nil {
		return 1
	}
	if p.Pick < 1 {
		return 1
	}
	return p.Pick
}

func wildcardID(text string) string {
	return officialCardID(wildcardLibraryID, "a", normalizeCardText(text))
}

func parseBanned(list string) []string {
	parts := strings.Split(list, ",")
	var out []string
	for _, p := range parts {
		n := normalizeCardText(p)
		if n != "" {
			out = append(out, n)
		}
	}
	return out
}

func bannedHit(text, list string) string {
	norm := normalizeCardText(text)
	if norm == "" {
		return ""
	}
	for _, phrase := range parseBanned(list) {
		if phrase == "" {
			continue
		}
		if phrase == norm {
			return phrase
		}
		// Whole word: surround with spaces after padding.
		padded := " " + norm + " "
		needle := " " + phrase + " "
		if strings.Contains(padded, needle) {
			return phrase
		}
	}
	return ""
}

func wildcardReject(text string, settings matchSettings, existing []string) error {
	trim := strings.TrimSpace(text)
	if trim == "" {
		return fmt.Errorf("Type an answer")
	}
	if settings.WildcardCap > 0 && len([]rune(trim)) > settings.WildcardCap {
		return fmt.Errorf("Too many characters")
	}
	if hit := bannedHit(trim, settings.WildcardBanned); hit != "" {
		if settings.WildcardShowMatchedWord {
			return fmt.Errorf("Banned: %s", hit)
		}
		return fmt.Errorf("That answer is not allowed")
	}
	if settings.WildcardDuplicateBlock {
		want := normalizeCardText(trim)
		for _, have := range existing {
			if normalizeCardText(have) == want {
				return fmt.Errorf("Wildcard already exists")
			}
		}
	}
	return nil
}
