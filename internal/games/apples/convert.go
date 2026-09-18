package apples

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
)

const officialLicense = "Cards Against Humanity writing. License https://creativecommons.org/licenses/by-nc-sa/4.0/legalcode . Material https://github.com/crhallberg/json-against-humanity . Modified from JSON Against Humanity. No warranty. Not endorsed by Cards Against Humanity LLC."

type cahFullPack struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Official    bool       `json:"official"`
	White       []cahWhite `json:"white"`
	Black       []cahBlack `json:"black"`
}

type cahWhite struct {
	Text string `json:"text"`
}

type cahBlack struct {
	Text string `json:"text"`
	Pick int    `json:"pick"`
}

type cahCompact struct {
	White []string `json:"white"`
	Black []struct {
		Text string `json:"text"`
		Pick int    `json:"pick"`
	} `json:"black"`
	Packs map[string]cahCompactPack `json:"packs"`
}

type cahCompactPack struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Official    bool   `json:"official"`
	White       []int  `json:"white"`
	Black       []int  `json:"black"`
}

func convertOfficial(raw []byte) (libraryFile, error) {
	packs, err := decodeCAHDump(raw)
	if err != nil {
		return libraryFile{}, err
	}
	lib := libraryFile{
		FormatVersion: libraryFormatVersion,
		ID:            officialLibraryID,
		Name:          "Official imported packs",
		Description:   "Converted from JSON Against Humanity. Official packs only.",
		License:       officialLicense,
		Source:        officialFileName,
	}
	used := map[string]int{}
	// Official dumps repeat some cards and differ others only by case.
	// IDs hash normalized text, so keep the first copy of each id.
	seenCards := map[string]struct{}{}
	for _, src := range packs {
		if !src.Official || omitOfficialPack(src.Name) {
			continue
		}
		packID := uniqueSlug(src.Name, used)
		pack := packFile{ID: packID, Name: src.Name, Description: src.Description}
		for _, card := range src.Black {
			pick := card.Pick
			if pick < 1 {
				pick = 1
			}
			id := officialCardID(packID, "p", card.Text)
			if _, dup := seenCards[id]; dup {
				continue
			}
			seenCards[id] = struct{}{}
			pack.Prompts = append(pack.Prompts, promptCard{ID: id, Text: card.Text, Pick: &pick})
		}
		for _, card := range src.White {
			id := officialCardID(packID, "a", card.Text)
			if _, dup := seenCards[id]; dup {
				continue
			}
			seenCards[id] = struct{}{}
			pack.Answers = append(pack.Answers, answerCard{ID: id, Text: card.Text})
		}
		lib.Packs = append(lib.Packs, pack)
	}
	if len(lib.Packs) == 0 {
		return libraryFile{}, fmt.Errorf("no official packs to convert")
	}
	if err := validateLibrary(lib); err != nil {
		return libraryFile{}, err
	}
	return lib, nil
}

func decodeCAHDump(raw []byte) ([]cahFullPack, error) {
	trim := strings.TrimSpace(string(raw))
	if strings.HasPrefix(trim, "[") {
		var packs []cahFullPack
		if err := json.Unmarshal(raw, &packs); err != nil {
			return nil, fmt.Errorf("could not parse official dump")
		}
		return packs, nil
	}
	var compact cahCompact
	if err := json.Unmarshal(raw, &compact); err != nil {
		return nil, fmt.Errorf("could not parse official dump")
	}
	if compact.Packs == nil {
		return nil, fmt.Errorf("could not parse official dump")
	}
	var packs []cahFullPack
	for _, src := range compact.Packs {
		pack := cahFullPack{Name: src.Name, Description: src.Description, Official: src.Official}
		for _, i := range src.White {
			if i < 0 || i >= len(compact.White) {
				return nil, fmt.Errorf("could not parse official dump")
			}
			pack.White = append(pack.White, cahWhite{Text: compact.White[i]})
		}
		for _, i := range src.Black {
			if i < 0 || i >= len(compact.Black) {
				return nil, fmt.Errorf("could not parse official dump")
			}
			pack.Black = append(pack.Black, cahBlack{Text: compact.Black[i].Text, Pick: compact.Black[i].Pick})
		}
		packs = append(packs, pack)
	}
	return packs, nil
}

func omitOfficialPack(name string) bool {
	n := strings.ToLower(name)
	switch {
	case strings.Contains(n, "a.i."), strings.Contains(n, "ai pack"):
		return true
	case strings.Contains(n, "procedurally"):
		return true
	case strings.Contains(n, "pax"):
		return true
	case strings.Contains(n, "retail exclusive"):
		return true
	case strings.Contains(n, "conversion kit"), strings.Contains(n, "conversion pack"):
		return true
	default:
		return false
	}
}

func uniqueSlug(name string, used map[string]int) string {
	base := slugify(name)
	if base == "" {
		base = "pack"
	}
	used[base]++
	if used[base] == 1 {
		return base
	}
	return fmt.Sprintf("%s-%d", base, used[base])
}

func slugify(name string) string {
	var b strings.Builder
	lastDash := true
	for _, r := range strings.ToLower(name) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func officialCardID(packID, kind, text string) string {
	sum := sha256.Sum256([]byte(normalizeCardText(text)))
	return fmt.Sprintf("%s-%s-%x", packID, kind, sum[:8])
}

func normalizeCardText(text string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(text))), " ")
}
