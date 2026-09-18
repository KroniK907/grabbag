package quips

type settingsView struct {
	pageView
	Frozen   bool
	Settings matchSettings
	Err      settingsErr
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
		Settings: g.currentSettings(),
		Err:      rowErr,
	}
}
