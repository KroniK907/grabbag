package quips_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/KroniK907/grabbag/internal/games/quips"
)

func TestShippedQuipsJSONIsPromptOnly(t *testing.T) {
	t.Parallel()
	path := filepath.Join("shipped", "quips.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lib, err := quips.ParsePromptOnlyLibrary("quips.json", raw)
	if err != nil {
		t.Fatal(err)
	}
	if lib.ID != "quips" || lib.Name != "Quick Quips" {
		t.Fatalf("id=%q name=%q", lib.ID, lib.Name)
	}
	if len(lib.Packs) != 1 || lib.Packs[0].ID != "comedy" {
		t.Fatalf("packs = %#v", lib.Packs)
	}
	if len(lib.Packs[0].Prompts) < 100 {
		t.Fatalf("comedy pack prompt count = %d", len(lib.Packs[0].Prompts))
	}
}
