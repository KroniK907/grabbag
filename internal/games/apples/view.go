package apples

import (
	"net/http"
	"strconv"

	"github.com/KroniK907/grabbag/internal/games"
)

type settingsView struct {
	pageView
	Frozen          bool
	Shortage        []string
	Settings        matchSettings
	WildcardEnabled bool
	WildcardAtStart bool
	Err             settingsErr
	VotingOff       bool
}

type holeView struct {
	Index  int
	Number int
	Card   *playCard
}

type packetView struct {
	ActorID  string
	Name     string
	Cards    []playCard
	Revealed bool
	Winner   bool
	Own      bool
	Votes    int
	ShowVote int
}

type rosterView struct {
	ID    string
	Name  string
	Score int
	Judge bool
	Bot   bool
}

type boardView struct {
	pageView
	Paused     bool
	Phase      string
	Prompt     *playPrompt
	FaceDown   bool
	Multiplier string
	Sudden     bool
	Packets    []packetView
	Roster     []rosterView
	Over       bool
	WinnerName string
	NamesShown bool
}

type phoneView struct {
	pageView
	Paused      bool
	Phase       string
	Role        string
	Prompt      *playPrompt
	Choices     []*playPrompt
	Holes       []holeView
	Hand        []playCard
	Blank       *playCard
	Draft       string
	Remain      int
	Cap         int
	Discard     *playCard
	HasWildSlot bool
	LockLabel   string
	LockReady   bool
	Skip        bool
	Error       string
	Packets     []packetView
	CanVote     bool
	CanReveal   bool
	CanConfirm  bool
	CanDraw     bool
	Multiplier  string
	Sudden      bool
	JudgeName   string
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
	return shortageLines(piles, s, n, false)
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
	if g.match != nil {
		view.WildcardAtStart = g.match.WildcardAtStart
		view.WildcardEnabled = view.WildcardEnabled || g.match.WildcardAtStart
	}
	g.mu.Unlock()
	return view
}

func (g *Game) boardView() boardView {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.helper != nil {
		g.syncRosterLocked(g.helper)
	}
	return g.boardViewLocked()
}

func (g *Game) boardViewLocked() boardView {
	view := boardView{
		pageView: g.chromeView("Apples for Humanity"),
		Paused:   g.paused,
	}
	m := g.match
	if m == nil {
		return view
	}
	view.Phase = string(m.Phase)
	view.Prompt = m.LivePrompt
	view.FaceDown = m.Phase == phaseChoose || (m.Phase == phaseDrawWait)
	view.Sudden = m.Phase == phaseSudden
	view.Over = m.Phase == phaseOver
	view.NamesShown = m.NamesShown
	if m.InMultiplier && m.Settings.LastRoundMultiplier > 1 {
		view.Multiplier = "THIS ROUND IS " + strconv.Itoa(m.Settings.LastRoundMultiplier) + "X POINTS"
	}
	if m.WinnerID != "" {
		if a := m.Actors[m.WinnerID]; a != nil {
			view.WinnerName = a.Name
		}
	}
	view.Packets = g.packetViewsLocked(m, "", true)
	for _, a := range m.Actors {
		view.Roster = append(view.Roster, rosterView{
			ID: a.ID, Name: a.Name, Score: a.Score, Judge: a.ID == m.JudgeID, Bot: a.Bot,
		})
	}
	return view
}

func (g *Game) phoneView(r *http.Request) phoneView {
	h := g.helperNow()
	var p games.Player
	if h != nil {
		p, _, _ = h.PlayerFromRequest(r)
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if h != nil {
		g.syncRosterLocked(h)
	}
	return g.phoneViewLocked(p)
}

func (g *Game) phoneViewLocked(p games.Player) phoneView {
	view := phoneView{
		pageView: g.chromeView("Apples for Humanity"),
		Paused:   g.paused,
	}
	m := g.match
	if m == nil {
		view.Role = "wait"
		return view
	}
	view.Phase = string(m.Phase)
	view.Prompt = m.LivePrompt
	view.Choices = []*playPrompt{m.Choice[0], m.Choice[1]}
	view.Sudden = m.Phase == phaseSudden
	view.Error = m.PhoneErr[p.ID]
	if a := m.Actors[m.JudgeID]; a != nil {
		view.JudgeName = a.Name
	}
	if m.InMultiplier && m.Settings.LastRoundMultiplier > 1 {
		view.Multiplier = "THIS ROUND IS " + strconv.Itoa(m.Settings.LastRoundMultiplier) + "X POINTS"
	}
	switch {
	case p.ID == m.JudgeID:
		view.Role = "judge"
		view.CanDraw = m.Phase == phaseDrawWait || m.Phase == phaseSudden
		view.CanReveal = m.Phase == phaseReveal
		view.CanConfirm = m.Phase == phaseReveal
		view.Skip = m.Settings.PromptMode == modeSkip && m.Phase == phaseSubmit
		for _, a := range m.Actors {
			if a.Locked && !a.Bot && a.ID != m.JudgeID {
				view.Skip = false
			}
		}
	case g.canSubmit(m, p.ID):
		view.Role = "submit"
	case p.Seated:
		view.Role = "seated"
	default:
		view.Role = "watch"
	}
	if a := m.Actors[p.ID]; a != nil {
		view.Hand = a.Hand
		view.Blank = a.Blank
		view.Draft = a.Draft
		view.Remain = remainingCap(a.Draft, m.Settings.WildcardCap)
		view.Cap = m.Settings.WildcardCap
		view.Discard = a.Discard
		for _, h := range a.Holes {
			if h != nil && h.Wildcard {
				view.HasWildSlot = true
			}
		}
		pick := promptPick(m.LivePrompt)
		for i := 0; i < pick; i++ {
			hv := holeView{Index: i}
			if pick > 1 {
				hv.Number = i + 1
			}
			if i < len(a.Holes) && a.Holes[i] != nil {
				hv.Card = a.Holes[i]
			}
			view.Holes = append(view.Holes, hv)
		}
		filled := filledCount(a.Holes)
		if a.Locked {
			view.LockLabel = "Locked"
			view.LockReady = false
			view.Role = "locked"
		} else if filled == pick && pick > 0 {
			if a.Discard != nil {
				view.LockLabel = "Lock and discard"
			} else {
				view.LockLabel = "Lock"
			}
			view.LockReady = true
		} else {
			view.LockLabel = strconv.Itoa(filled) + " of " + strconv.Itoa(pick)
		}
	}
	view.Packets = g.packetViewsLocked(m, p.ID, false)
	view.CanVote = g.playerMayVote(m, p)
	for _, pk := range view.Packets {
		if !pk.Revealed {
			view.CanConfirm = false
		}
	}
	return view
}

func (g *Game) packetViewsLocked(m *matchState, viewer string, tv bool) []packetView {
	counts := map[string]int{}
	for _, t := range m.Votes {
		counts[t]++
	}
	var out []packetView
	showCounts := tv && m.Settings.LiveVoteCounts && m.Settings.Voting != voteOff
	for _, p := range m.Packets {
		name := p.ActorID
		if a := m.Actors[p.ActorID]; a != nil {
			name = a.Name
		}
		pv := packetView{
			ActorID:  p.ActorID,
			Name:     name,
			Cards:    p.Cards,
			Revealed: p.Revealed,
			Winner:   m.NamesShown && p.ActorID == m.WinnerID,
			Own:      p.ActorID == viewer,
			Votes:    counts[p.ActorID],
		}
		if showCounts && p.Revealed {
			pv.ShowVote = pv.Votes
		} else {
			pv.ShowVote = -1
		}
		if m.NamesShown {
			pv.Revealed = true
		}
		out = append(out, pv)
	}
	return out
}

func (g *Game) pickerView(rowErr pickerErr) pickerView {
	view := pickerView{
		pageView:     g.chromeView("Deck Library"),
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
	g.mu.Lock()
	if view.RowError == "" {
		view.RowError = g.importErr
	}
	g.mu.Unlock()
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
				AnswerCount: len(pack.Answers),
			})
		}
		view.Libraries = append(view.Libraries, item)
	}
	view.Failed = cat.Failed
	return view
}
