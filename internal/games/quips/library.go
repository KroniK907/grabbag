package quips

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const libraryFormatVersion = 1

type failedFile struct {
	Filename string
	Reason   string
}

type catalog struct {
	Libraries []libraryFile
	Failed    []failedFile
}

type libraryFile struct {
	FormatVersion int        `json:"formatVersion"`
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Description   string     `json:"description,omitempty"`
	License       string     `json:"license,omitempty"`
	Packs         []packFile `json:"packs"`
	Source        string     `json:"-"`
}

type packFile struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	Prompts     []promptCard `json:"prompts"`
	Answers     []answerCard `json:"answers"`
}

type promptCard struct {
	ID   string   `json:"id"`
	Text string   `json:"text"`
	Pick *int     `json:"pick,omitempty"`
	Tags []string `json:"tags,omitempty"`
}

type answerCard struct {
	ID   string   `json:"id"`
	Text string   `json:"text"`
	Tags []string `json:"tags,omitempty"`
}

// ValidatePromptOnlyLibrary checks a decoded library for Quick Quips prompt-only rules.
func ValidatePromptOnlyLibrary(lib libraryFile) error {
	file := lib.Source
	if lib.FormatVersion != libraryFormatVersion {
		return fmt.Errorf("%s: formatVersion must be 1", file)
	}
	if strings.TrimSpace(lib.ID) == "" {
		return fmt.Errorf("%s: missing library id", file)
	}
	if strings.TrimSpace(lib.Name) == "" {
		return fmt.Errorf("%s: missing library name", file)
	}
	packIDs := map[string]struct{}{}
	cardIDs := map[string]struct{}{}
	for _, pack := range lib.Packs {
		if strings.TrimSpace(pack.ID) == "" {
			return fmt.Errorf("%s: missing pack id", file)
		}
		if strings.TrimSpace(pack.Name) == "" {
			return fmt.Errorf("%s: missing pack name", file)
		}
		if _, exists := packIDs[pack.ID]; exists {
			return fmt.Errorf("%s: duplicate pack id %q", file, pack.ID)
		}
		packIDs[pack.ID] = struct{}{}
		if len(pack.Answers) > 0 {
			return fmt.Errorf("%s: pack %q must not contain answer cards", file, pack.ID)
		}
		for _, card := range pack.Prompts {
			if err := validateCardID(file, card.ID, card.Text, cardIDs); err != nil {
				return err
			}
			if card.Pick != nil && *card.Pick < 1 {
				return fmt.Errorf("%s: pick must be at least 1 on card %q", file, card.ID)
			}
		}
	}
	return nil
}

func validateCardID(file, id, text string, seen map[string]struct{}) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("%s: missing card id", file)
	}
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("%s: missing text on card %q", file, id)
	}
	if _, exists := seen[id]; exists {
		return fmt.Errorf("%s: duplicate card id %q", file, id)
	}
	seen[id] = struct{}{}
	return nil
}

// ParsePromptOnlyLibrary decodes and validates one on-disk library file.
func ParsePromptOnlyLibrary(filename string, raw []byte) (libraryFile, error) {
	var lib libraryFile
	if err := json.Unmarshal(raw, &lib); err != nil {
		return libraryFile{}, fmt.Errorf("%s: unreadable JSON", filename)
	}
	lib.Source = filename
	if err := ValidatePromptOnlyLibrary(lib); err != nil {
		return libraryFile{}, err
	}
	return lib, nil
}

// ValidatePromptOnlyFile reads path and runs prompt-only validation.
func ValidatePromptOnlyFile(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	name := filepath.Base(path)
	_, err = ParsePromptOnlyLibrary(name, raw)
	return err
}

func scanDataDir(dir string) catalog {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return catalog{}
	}
	type parsed struct {
		lib  libraryFile
		err  error
		name string
	}
	var rows []parsed
	byID := map[string][]string{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			rows = append(rows, parsed{name: entry.Name(), err: fmt.Errorf("%s: unreadable JSON", entry.Name())})
			continue
		}
		lib, err := ParsePromptOnlyLibrary(entry.Name(), raw)
		if err != nil {
			rows = append(rows, parsed{name: entry.Name(), err: err})
			continue
		}
		rows = append(rows, parsed{name: entry.Name(), lib: lib})
		byID[lib.ID] = append(byID[lib.ID], entry.Name())
	}
	clashed := map[string]struct{}{}
	for _, files := range byID {
		if len(files) > 1 {
			for _, name := range files {
				clashed[name] = struct{}{}
			}
		}
	}
	out := catalog{}
	for _, row := range rows {
		if row.err != nil {
			out.Failed = append(out.Failed, failedFile{Filename: row.name, Reason: row.err.Error()})
			continue
		}
		if _, clash := clashed[row.name]; clash {
			out.Failed = append(out.Failed, failedFile{
				Filename: row.name,
				Reason:   fmt.Sprintf("%s: duplicate library id %q", row.name, row.lib.ID),
			})
			continue
		}
		out.Libraries = append(out.Libraries, row.lib)
	}
	return out
}

func packExists(cat catalog, libraryID, packID string) bool {
	for _, lib := range cat.Libraries {
		if lib.ID != libraryID {
			continue
		}
		for _, pack := range lib.Packs {
			if pack.ID == packID {
				return true
			}
		}
	}
	return false
}
