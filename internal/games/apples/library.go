package apples

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	libraryFormatVersion = 1
	officialLibraryID    = "official"
	officialFileName     = "official.json"
	wildcardLibraryID    = "wildcard"
)

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

type failedFile struct {
	Filename string
	Reason   string
}

type catalog struct {
	Libraries []libraryFile
	Failed    []failedFile
}

func parseLibrary(filename string, raw []byte) (libraryFile, error) {
	var lib libraryFile
	if err := json.Unmarshal(raw, &lib); err != nil {
		return libraryFile{}, fmt.Errorf("%s: unreadable JSON", filename)
	}
	lib.Source = filename
	if err := validateLibrary(lib); err != nil {
		return libraryFile{}, err
	}
	for i := range lib.Packs {
		for j := range lib.Packs[i].Prompts {
			if lib.Packs[i].Prompts[j].Pick == nil {
				one := 1
				lib.Packs[i].Prompts[j].Pick = &one
			}
		}
	}
	return lib, nil
}

func validateLibrary(lib libraryFile) error {
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
		for _, card := range pack.Prompts {
			if err := validateCardID(file, card.ID, card.Text, cardIDs); err != nil {
				return err
			}
			if card.Pick != nil && *card.Pick < 1 {
				return fmt.Errorf("%s: pick must be at least 1 on card %q", file, card.ID)
			}
		}
		for _, card := range pack.Answers {
			if err := validateCardID(file, card.ID, card.Text, cardIDs); err != nil {
				return err
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
		return fmt.Errorf("%s: missing card text on %q", file, id)
	}
	if _, exists := seen[id]; exists {
		return fmt.Errorf("%s: duplicate card id %q", file, id)
	}
	seen[id] = struct{}{}
	return nil
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
		lib, err := parseLibrary(entry.Name(), raw)
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

func isFamilyLibrary(id string) bool {
	if id == wildcardLibraryID || id == officialLibraryID {
		return false
	}
	return id == "oranges" || id == "white-black"
}
