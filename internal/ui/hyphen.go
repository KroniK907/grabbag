package ui

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// softHyphenEvery is how many letters or digits sit between break points.
// The hyphen only appears when the card is too narrow for the whole word.
const softHyphenEvery = 8

// SoftHyphen inserts soft hyphens into long unbroken tokens.
// A word that fits the card stays on one line. A word that does not
// wraps at a soft hyphen and draws a hyphen at the break.
func SoftHyphen(s string) string {
	if utf8.RuneCountInString(s) <= softHyphenEvery {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 8)
	run := 0
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			run++
			if run > softHyphenEvery {
				b.WriteRune('\u00ad')
				run = 1
			}
		} else {
			run = 0
		}
		b.WriteRune(r)
	}
	return b.String()
}
