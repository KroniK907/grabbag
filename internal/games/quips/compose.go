package quips

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// composePolicy holds match settings used for quip validation (GM-058-067).
type composePolicy struct {
	Cap             int
	Banned          string
	ShowMatchedWord bool
}

func composePolicyFrom(s matchSettings) composePolicy {
	return composePolicy{
		Cap:             s.QuipCharCap,
		Banned:          s.BannedWords,
		ShowMatchedWord: s.ShowMatchedWord,
	}
}

func trimToCap(text string, cap int) string {
	trim := strings.TrimSpace(text)
	if cap <= 0 {
		return trim
	}
	if utf8.RuneCountInString(trim) <= cap {
		return trim
	}
	runes := []rune(trim)
	return string(runes[:cap])
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
		padded := " " + norm + " "
		needle := " " + phrase + " "
		if strings.Contains(padded, needle) {
			return phrase
		}
	}
	return ""
}

func (p composePolicy) draftOK(text string) bool {
	trim := trimToCap(text, p.Cap)
	if strings.TrimSpace(trim) == "" {
		return true
	}
	return bannedHit(trim, p.Banned) == ""
}

type lockIssue struct {
	Slot    int
	Empty   bool
	Banned  string
	Dup     bool
}

func (p composePolicy) lockIssues(texts []string, lockedNorms []string) []lockIssue {
	var issues []lockIssue
	for i, raw := range texts {
		trim := trimToCap(raw, p.Cap)
		if strings.TrimSpace(trim) == "" {
			issues = append(issues, lockIssue{Slot: i, Empty: true})
			continue
		}
		if hit := bannedHit(trim, p.Banned); hit != "" {
			issues = append(issues, lockIssue{Slot: i, Banned: hit})
			continue
		}
		norm := normalizeCardText(trim)
		for _, have := range lockedNorms {
			if norm == have {
				issues = append(issues, lockIssue{Slot: i, Dup: true})
				break
			}
		}
	}
	return issues
}

func (p composePolicy) bannedAlert(hit string) string {
	if hit == "" {
		return "That quip is not allowed."
	}
	if p.ShowMatchedWord {
		return fmt.Sprintf("Banned: %s", hit)
	}
	return "That quip is not allowed."
}

func (p composePolicy) lockAlert(issues []lockIssue) string {
	var parts []string
	for _, iss := range issues {
		switch {
		case iss.Empty:
			parts = append(parts, "Fill in every quip before locking.")
		case iss.Banned != "":
			parts = append(parts, p.bannedAlert(iss.Banned))
		case iss.Dup:
			parts = append(parts, "That quip was already used.")
		}
	}
	if len(parts) == 0 {
		return ""
	}
	seen := map[string]struct{}{}
	var uniq []string
	for _, s := range parts {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		uniq = append(uniq, s)
	}
	return strings.Join(uniq, " ")
}

func (p composePolicy) timerSubmit(draft string, lockedNorms []string) string {
	trim := trimToCap(draft, p.Cap)
	if strings.TrimSpace(trim) == "" {
		return ""
	}
	if bannedHit(trim, p.Banned) != "" {
		return ""
	}
	if p.Cap > 0 && utf8.RuneCountInString(trim) > p.Cap {
		return ""
	}
	norm := normalizeCardText(trim)
	for _, have := range lockedNorms {
		if norm == have {
			return ""
		}
	}
	return trim
}
