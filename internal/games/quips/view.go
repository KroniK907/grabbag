package quips

import "github.com/KroniK907/grabbag/internal/games/runtimekit"

type settingsView struct {
	runtimekit.Page
	Frozen         bool
	Shortage       []string
	Settings       matchSettings
	Err            settingsErr
	Burns          []burnEntry
	BurnCorrupt    bool
	DiscardCorrupt bool
	DiscardEmpty   bool
}

type pickerView struct {
	runtimekit.Page
	DataDir      string
	Frozen       bool
	Shortage     []string
	RowError     string
	ErrorLibrary string
	ErrorPack    string
	Libraries    []libraryView
	Failed       []failedFile
}

type libraryView struct {
	ID          string
	Name        string
	Description string
	License     string
	Filename    string
	Packs       []packView
}

type packView struct {
	LibraryID   string
	ID          string
	Name        string
	Description string
	Enabled     bool
	PromptCount int
}

func (g *Game) currentSettings() matchSettings {
	h := g.run.Helper()
	if h == nil {
		return factorySettings()
	}
	cat := scanDataDir(h.DataDir())
	s, ok := g.loadSettings(h)
	return reconcileSettings(s, ok, cat)
}

func (g *Game) settingsView(rowErr settingsErr) settingsView {
	view := settingsView{
		Page:     g.run.Page("Quick Quips settings"),
		Frozen:   g.matchFrozen(),
		Shortage: g.shortageNow(),
		Settings: g.currentSettings(),
		Err:      rowErr,
	}
	g.run.Lock()
	view.BurnCorrupt = g.burnCorrupt
	view.DiscardCorrupt = g.discardCorrupt
	view.DiscardEmpty = len(g.played) == 0
	if !g.burnCorrupt {
		view.Burns = g.lastBurns(10)
	}
	g.run.Unlock()
	return view
}

func (g *Game) shortageNow() []string {
	h := g.run.Helper()
	if h == nil {
		return nil
	}
	s := g.currentSettings()
	cat := scanDataDir(h.DataDir())
	piles := buildPromptPiles(cat, s)
	n := len(h.Seated())
	g.run.Lock()
	noBurn := filterBurns(piles, g.burns)
	help := burnedWouldHelp(piles, noBurn, s, n)
	g.run.Unlock()
	return shortageLines(noBurn, s, n, help)
}

func (g *Game) pickerView(rowErr pickerErr) pickerView {
	view := pickerView{
		Page:         g.run.Page("Prompt Library"),
		RowError:     rowErr.Msg,
		ErrorLibrary: rowErr.LibraryID,
		ErrorPack:    rowErr.PackID,
		Shortage:     g.shortageNow(),
	}
	h := g.run.Helper()
	if h == nil {
		return view
	}
	view.DataDir = h.DataDir()
	view.Frozen = g.matchFrozen()
	cat := scanDataDir(h.DataDir())
	settings, ok := g.loadSettings(h)
	settings = reconcileSettings(settings, ok, cat)
	for _, lib := range cat.Libraries {
		item := libraryView{
			ID:          lib.ID,
			Name:        lib.Name,
			Description: lib.Description,
			License:     lib.License,
			Filename:    lib.Source,
		}
		for _, pack := range lib.Packs {
			item.Packs = append(item.Packs, packView{
				LibraryID:   lib.ID,
				ID:          pack.ID,
				Name:        pack.Name,
				Description: pack.Description,
				Enabled:     settings.packOn(lib.ID, pack.ID),
				PromptCount: len(pack.Prompts),
			})
		}
		view.Libraries = append(view.Libraries, item)
	}
	view.Failed = cat.Failed
	return view
}
