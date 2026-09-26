package apples

type pickerView struct {
	pageView
	DataDir      string
	Frozen       bool
	RowError     string
	ErrorLibrary string
	ErrorPack    string
	ErrorTag     string
	Shortage     []string
	Tags         []tagView
	Libraries    []libraryView
	Failed       []failedFile
}

type tagView struct {
	ID      string
	Label   string
	Color   string
	Enabled bool
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
	AnswerCount int
	Dots        []tagView
}

type settingsView struct {
	pageView
	Frozen          bool
	Shortage        []string
	Settings        matchSettings
	WildcardEnabled bool
	WildcardAtStart bool
	Err             settingsErr
	VotingOff       bool
	Burns           []burnEntry
	BurnCorrupt     bool
	DiscardCorrupt  bool
	DiscardEmpty    bool
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

func (g *Game) shortageNow() []string {
	h := g.helperNow()
	if h == nil {
		return nil
	}
	s := g.currentSettings()
	cat := scanDataDir(h.DataDir())
	piles := buildPiles(cat, s)
	n := len(h.Seated())
	g.mu.Lock()
	noBurn := filterBurns(piles, g.burns)
	help := burnedWouldHelp(piles, noBurn, s, n)
	g.mu.Unlock()
	return shortageLines(noBurn, s, n, help)
}

func (g *Game) settingsView(rowErr settingsErr) settingsView {
	s := g.currentSettings()
	view := settingsView{
		pageView:        g.chromeView("Apples for Humanity"),
		Frozen:          g.matchFrozen(),
		Shortage:        g.shortageNow(),
		Settings:        s,
		WildcardEnabled: s.wildcardLibraryOn(),
		Err:             rowErr,
		VotingOff:       s.Voting == voteOff,
	}
	g.mu.Lock()
	view.BurnCorrupt = g.burnCorrupt
	view.DiscardCorrupt = g.discardCorrupt
	view.DiscardEmpty = len(g.played) == 0
	if !g.burnCorrupt {
		view.Burns = g.lastBurns(10)
	}
	if g.engine != nil {
		view.WildcardAtStart = g.engine.WildcardAtStart
		view.WildcardEnabled = view.WildcardEnabled || g.engine.WildcardAtStart
	}
	g.mu.Unlock()
	return view
}

func (g *Game) pickerView(rowErr pickerErr) pickerView {
	view := pickerView{
		pageView:     g.chromeView("Deck Library"),
		RowError:     rowErr.Msg,
		ErrorLibrary: rowErr.LibraryID,
		ErrorPack:    rowErr.PackID,
		ErrorTag:     rowErr.Tag,
		Shortage:     g.shortageNow(),
	}
	h := g.helperNow()
	if h == nil {
		return view
	}
	view.DataDir = h.DataDir()
	view.Frozen = g.matchFrozen()
	g.mu.Lock()
	if view.RowError == "" {
		view.RowError = g.importErr
	}
	g.mu.Unlock()
	cat := scanDataDir(h.DataDir())
	settings, ok := g.loadSettings(h)
	settings = reconcileSettings(settings, ok, cat)
	view.Tags = tagViews(pickerTagIDs(cat), settings)
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
				AnswerCount: len(pack.Answers),
				Dots:        tagViews(packDotIDs(pack), settings),
			})
		}
		view.Libraries = append(view.Libraries, item)
	}
	view.Failed = cat.Failed
	return view
}
