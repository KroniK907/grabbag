package host

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/KroniK907/grabbag/internal/games"
)

// Compressed audio budgets (GM-031). Each tenant's static/audio folder ships
// inside the binary. A game that needs more raises its own entry in
// audioBudgetFor with a comment giving the reason.
const (
	lobbyAudioBudget = 2 << 20
	gameAudioBudget  = 3 << 20
)

// audioBudgetFor holds per-game raises over gameAudioBudget. Empty for now.
var audioBudgetFor = map[string]int64{}

func TestAudioSizeBudget(t *testing.T) {
	t.Parallel()
	check := func(name, dir string, budget int64) {
		var total int64
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() {
				info, err := d.Info()
				if err != nil {
					return err
				}
				total += info.Size()
			}
			return nil
		})
		if err != nil && !os.IsNotExist(err) {
			t.Fatalf("%s: %v", name, err)
		}
		if total > budget {
			t.Errorf("%s ships %d bytes of audio, over its %d byte budget", name, total, budget)
		}
	}
	check("lobby", filepath.Join("..", "lobby", "static", "audio"), lobbyAudioBudget)
	for _, f := range games.Catalog() {
		budget := int64(gameAudioBudget)
		if raised, ok := audioBudgetFor[f.ID]; ok {
			budget = raised
		}
		// A game's folder is named for its id. Guard it, or a rename would
		// silently skip the budget.
		if _, err := os.Stat(filepath.Join("..", "games", f.ID)); err != nil {
			t.Fatalf("%s: no package folder at internal/games/%s: %v", f.ID, f.ID, err)
		}
		check(f.ID, filepath.Join("..", "games", f.ID, "static", "audio"), budget)
	}
}
