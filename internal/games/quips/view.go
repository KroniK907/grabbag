package quips

type settingsView struct {
	pageView
	Frozen   bool
	Shortage []string
	Settings matchSettings
	Err      settingsErr
}

type pickerView struct {
	pageView
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
	h := g.helperNow()
	if h == nil {
		return factorySettings()
	}
	cat := scanDataDir(h.DataDir())
	s, ok := g.loadSettings(h)
	return reconcileSettings(s, ok, cat)
}

func (g *Game) settingsView(rowErr settingsErr) settingsView {
	return settingsView{
		pageView: g.pageView("Quick Quips settings"),
		Frozen:   g.matchFrozen(),
		Shortage: g.shortageNow(),
		Settings: g.currentSettings(),
		Err:      rowErr,
	}
}

func (g *Game) shortageNow() []string {
	h := g.helperNow()
	if h == nil {
		return nil
	}
	s := g.currentSettings()
	cat := scanDataDir(h.DataDir())
	return shortageLines(cat, s, len(h.Seated()))
}

func (g *Game) pickerView(rowErr pickerErr) pickerView {
	view := pickerView{
		pageView:     g.pageView("Prompt Library"),
		RowError:     rowErr.Msg,
		ErrorLibrary: rowErr.LibraryID,
		ErrorPack:    rowErr.PackID,
		Shortage:     g.shortageNow(),
	}
	h := g.helperNow()
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
