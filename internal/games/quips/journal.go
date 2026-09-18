package quips

import (
	"os"
	"path/filepath"

	"github.com/KroniK907/grabbag/internal/games"
)

const (
	stateDirName    = "state"
	burnedFileName  = "burned.json"
	discardFileName = "discard.json"
	kindPrompt      = "prompt"
)

type burnEntry struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

type playedRow struct {
	LibraryID string `json:"libraryId"`
	CardID    string `json:"cardId"`
}

// openJournalsLocked ensures the journal directory exists. Writers and disk
// replay land in a later burn/discard task.
func (g *Game) openJournalsLocked(h games.Helper) {
	dir := filepath.Join(h.DataDir(), stateDirName)
	_ = os.MkdirAll(dir, 0o700)
}

func (g *Game) drainJournalsLocked() {}

func (g *Game) stopWritersLocked() {}
