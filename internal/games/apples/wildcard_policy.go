package apples

import (
	"fmt"
	"strings"
)

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

func remainingCap(draft string, cap int) int {
	if cap <= 0 {
		return 0
	}
	n := cap - len([]rune(draft))
	if n < 0 {
		return 0
	}
	return n
}
