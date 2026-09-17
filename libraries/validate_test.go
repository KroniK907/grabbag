// Package libraries_test checks shipped library JSON against CAH-GAME-GM-007 through GM-011.
package libraries_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

type card struct {
	ID   string   `json:"id"`
	Text string   `json:"text"`
	Pick *int     `json:"pick"`
	Tags []string `json:"tags"`
}

type pack struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Prompts     []card `json:"prompts"`
	Answers     []card `json:"answers"`
}

type libraryFile struct {
	FormatVersion int    `json:"formatVersion"`
	ID            string `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Packs         []pack `json:"packs"`
}

// greenText matches Adjective - (syn, syn) or Adjective - (syn, syn, syn).
var greenText = regexp.MustCompile(`^.+ - \([^,]+, [^,]+(?:, [^,]+)?\)$`)

func loadLibrary(t *testing.T, name string) libraryFile {
	t.Helper()
	path := filepath.Join(".", name)
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", name, err)
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	var lib libraryFile
	if err := dec.Decode(&lib); err != nil {
		t.Fatalf("decode %s: %v", name, err)
	}
	return lib
}

func TestOrangesLibrarySchema(t *testing.T) {
	lib := loadLibrary(t, "oranges.json")
	if lib.FormatVersion != 1 {
		t.Fatalf("formatVersion=%d want 1", lib.FormatVersion)
	}
	if lib.ID != "oranges" || lib.Name != "Oranges to Oranges" {
		t.Fatalf("id=%q name=%q", lib.ID, lib.Name)
	}
	if len(lib.Packs) != 1 {
		t.Fatalf("packs=%d want 1", len(lib.Packs))
	}
	p := lib.Packs[0]
	if len(p.Prompts) < 50 {
		t.Fatalf("prompts=%d want at least 50", len(p.Prompts))
	}
	if len(p.Answers) < 140 {
		t.Fatalf("answers=%d want at least 140", len(p.Answers))
	}

	ids := map[string]bool{}
	packIDs := map[string]bool{}
	for _, pack := range lib.Packs {
		if pack.ID == "" || pack.Name == "" {
			t.Fatal("pack missing id or name")
		}
		if packIDs[pack.ID] {
			t.Fatalf("duplicate pack id %q", pack.ID)
		}
		packIDs[pack.ID] = true
		checkCards(t, ids, pack.Prompts, true)
		checkCards(t, ids, pack.Answers, false)
	}
}

func checkCards(t *testing.T, ids map[string]bool, cards []card, prompts bool) {
	t.Helper()
	for _, c := range cards {
		if c.ID == "" || strings.TrimSpace(c.Text) == "" {
			t.Fatalf("card missing id or text: %+v", c)
		}
		if ids[c.ID] {
			t.Fatalf("duplicate card id %q", c.ID)
		}
		ids[c.ID] = true
		if c.Pick != nil {
			if *c.Pick < 1 {
				t.Fatalf("pick %d on %q", *c.Pick, c.ID)
			}
			if prompts {
				t.Fatalf("oranges prompt %q has pick", c.ID)
			}
		}
		if len(c.Tags) > 0 {
			t.Fatalf("card %q has tags", c.ID)
		}
		if prompts {
			if strings.Contains(c.Text, "_") {
				t.Fatalf("prompt %q contains a blank", c.ID)
			}
			if !greenText.MatchString(c.Text) {
				t.Fatalf("prompt %q text %q is not green-card shape", c.ID, c.Text)
			}
		}
	}
}

func TestWildcardLibrarySchema(t *testing.T) {
	lib := loadLibrary(t, "wildcard.json")
	if lib.FormatVersion != 1 {
		t.Fatalf("formatVersion=%d want 1", lib.FormatVersion)
	}
	if lib.ID != "wildcard" {
		t.Fatalf("id=%q want wildcard", lib.ID)
	}
	if len(lib.Packs) != 1 || lib.Packs[0].ID != "wildcard" {
		t.Fatalf("want one pack id wildcard, got %+v", lib.Packs)
	}
	p := lib.Packs[0]
	if len(p.Prompts) != 0 {
		t.Fatalf("wildcard prompts=%d want 0", len(p.Prompts))
	}
	if len(p.Answers) != 1 {
		t.Fatalf("wildcard answers=%d want 1", len(p.Answers))
	}
	a := p.Answers[0]
	if a.ID == "" || strings.TrimSpace(a.Text) == "" {
		t.Fatal("wildcard answer missing id or text")
	}
	if a.Pick != nil {
		t.Fatalf("wildcard answer has pick")
	}
}
