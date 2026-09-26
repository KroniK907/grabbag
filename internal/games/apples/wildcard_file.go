package apples

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const wildcardFileName = "wildcard.json"

func readWildcardLibrary(dir string) (libraryFile, bool) {
	raw, err := os.ReadFile(filepath.Join(dir, wildcardFileName))
	if err != nil {
		return libraryFile{}, false
	}
	lib, err := parseLibrary(wildcardFileName, raw)
	if err != nil {
		return libraryFile{}, false
	}
	return lib, true
}

// wildcardTexts is every saved wildcard answer in dir.
func wildcardTexts(dir string) []string {
	lib, ok := readWildcardLibrary(dir)
	if !ok {
		return nil
	}
	var out []string
	for _, pack := range lib.Packs {
		for _, a := range pack.Answers {
			out = append(out, a.Text)
		}
	}
	return out
}

// saveWildcards appends cards to the wildcard library in dir, skipping text
// it already holds. It returns the cards it added, as dealable answers.
func saveWildcards(dir string, cards []playCard) []playCard {
	if len(cards) == 0 {
		return nil
	}
	lib, ok := readWildcardLibrary(dir)
	if !ok {
		return nil
	}
	have := map[string]bool{}
	for _, pack := range lib.Packs {
		for _, a := range pack.Answers {
			have[normalizeCardText(a.Text)] = true
		}
	}
	if len(lib.Packs) == 0 {
		lib.Packs = []packFile{{ID: wildcardLibraryID, Name: "Wildcard"}}
	}
	var added []playCard
	for _, c := range cards {
		norm := normalizeCardText(c.Text)
		if have[norm] {
			continue
		}
		have[norm] = true
		id := c.CardID
		if id == "" || id == blankCardID {
			id = wildcardID(c.Text)
		}
		lib.Packs[0].Answers = append(lib.Packs[0].Answers, answerCard{ID: id, Text: c.Text})
		added = append(added, playCard{LibraryID: wildcardLibraryID, CardID: id, Text: c.Text})
	}
	encoded, err := json.MarshalIndent(lib, "", "  ")
	if err != nil {
		return nil
	}
	if err := os.WriteFile(filepath.Join(dir, wildcardFileName), encoded, 0o600); err != nil {
		return nil
	}
	return added
}
