package apples

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"

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
	Burns           []burnEntry
	BurnCorrupt     bool
	DiscardCorrupt  bool
	DiscardEmpty    bool
}

type holeView struct {
	Index  int
	Number int
	Card   *playCard
}

type packetView struct {
	ActorID      string
	Name         string
	Cards        []playCard
	Revealed     bool
	Winner       bool
	Own          bool
	Voted        bool
	Votes        int
	ShowVote     int
	VoteLabel    string
	FavoriteMark string
}

type rosterView struct {
	ID     string
	Name   string
	Score  int
	Judge  bool
	Bot    bool
	Locked bool
}

type boardView struct {
	pageView
	Paused       bool
	Phase        string
	Prompt       *playPrompt
	FaceDown     bool
	Multiplier   string
	Sudden       bool
	Packets      []packetView
	Roster       []rosterView
	Over         bool
	WinnerName   string
	RoundWinner  string
	NamesShown   bool
	Overlay      bool
	OverlayCopy  string
	TimerLabel   string
	TimerText    string
	TimerSeconds int
	TimerTotal   int
	TimerEndUnix int64
}

type phoneView struct {
	pageView
	Paused       bool
	Phase        string
	Role         string
	Prompt       *playPrompt
	Choices      []*playPrompt
	Holes        []holeView
	Hand         []playCard
	Blank        *playCard
	Draft        string
	Remain       int
	Cap          int
	Discard      *playCard
	HasWildSlot  bool
	LockLabel    string
	LockReady    bool
	Skip         bool
	Error        string
	Packets      []packetView
	CanVote      bool
	CanReveal    bool
	CanConfirm   bool
	CanDraw      bool
	Multiplier   string
	Sudden       bool
	Over         bool
	WinnerName   string
	RoundWinner  string
	JudgeName    string
	Help         bool
	ClaimedHost  bool
	BurnFaces    []burnFace
	BurnErr      string
	Overlay      bool
	OverlayCopy  string
	OverlayYes   bool
	TimerLabel   string
	TimerText    string
	TimerSeconds int
	TimerTotal   int
	TimerEndUnix int64
	BurnOpen     bool
	Keep         bool
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
	view.FaceDown = m.Phase == phaseChoose || m.Phase == phaseHold || m.Phase == phaseDrawWait
	view.Sudden = m.Phase == phaseSudden
	view.Over = m.Phase == phaseOver
	view.NamesShown = m.NamesShown
	view.TimerLabel, view.TimerText, view.TimerSeconds, view.TimerTotal, view.TimerEndUnix = g.timerViewLocked(m)
	if m.InMultiplier && m.Settings.LastRoundMultiplier > 1 {
		view.Multiplier = "THIS ROUND IS " + strconv.Itoa(m.Settings.LastRoundMultiplier) + "X POINTS"
	}
	if m.WinnerID != "" {
		if a := m.Actors[m.WinnerID]; a != nil {
			view.WinnerName = a.Name
			if m.NamesShown {
				view.RoundWinner = a.Name
			}
		}
	}
	if g.overlay != "" {
		view.Overlay = true
		view.OverlayCopy = overlayTVCopy
		if g.overlayTooSmall {
			view.OverlayCopy = overlayFailCopy
		}
	}
	view.Packets = g.packetViewsLocked(m, "", true)
	for _, a := range m.Actors {
		view.Roster = append(view.Roster, rosterView{
			ID:     a.ID,
			Name:   a.Name,
			Score:  a.Score,
			Judge:  a.ID == m.JudgeID,
			Bot:    a.Bot,
			Locked: m.Phase == phaseSubmit && a.Locked && a.ID != m.JudgeID,
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
	view.Over = m.Phase == phaseOver
	view.Error = m.PhoneErr[p.ID]
	view.Help = true
	view.TimerLabel, view.TimerText, view.TimerSeconds, view.TimerTotal, view.TimerEndUnix = g.timerViewLocked(m)
	view.ClaimedHost = p.ClaimedHost
	view.BurnErr = g.burnErr
	for _, f := range g.burnSnap {
		f.Checked = g.burnChecks[f.Kind+"\x00"+f.Text]
		view.BurnFaces = append(view.BurnFaces, f)
	}
	if g.overlay != "" {
		view.Overlay = true
		copy, yes := overlayCopy(p.ClaimedHost, g.overlayTooSmall)
		view.OverlayCopy = copy
		view.OverlayYes = yes
	}
	if a := m.Actors[m.JudgeID]; a != nil {
		view.JudgeName = a.Name
	}
	if m.WinnerID != "" {
		if a := m.Actors[m.WinnerID]; a != nil {
			view.WinnerName = a.Name
			if m.NamesShown {
				view.RoundWinner = a.Name
			}
		}
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
		view.Skip = m.Settings.PromptMode == modeSkip && m.Phase == phaseHold
		view.Keep = view.Skip
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
		if a.Locked && view.Role != "judge" && m.Phase == phaseSubmit {
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
	if m.Phase == phaseHold && view.Role != "judge" {
		view.Prompt = nil
	}
	view.Packets = g.packetViewsLocked(m, p.ID, false)
	view.CanVote = g.playerMayVote(m, p)
	view.CanReveal = false
	for _, pk := range view.Packets {
		if !pk.Revealed {
			view.CanConfirm = false
			if view.Role == "judge" && m.Phase == phaseReveal {
				view.CanReveal = true
			}
		}
	}
	return view
}

func (g *Game) packetViewsLocked(m *matchState, viewer string, tv bool) []packetView {
	counts := map[string]int{}
	for _, t := range m.Votes {
		counts[t]++
	}
	favorites := favoriteMarks(counts)
	votedFor := m.Votes[viewer]
	var out []packetView
	showCounts := tv && m.Settings.LiveVoteCounts && m.Settings.Voting != voteOff
	packets := m.Packets
	if tv && m.Phase == phaseSubmit {
		byActor := make(map[string]packet, len(m.Packets))
		for _, p := range m.Packets {
			byActor[p.ActorID] = p
		}
		var ids []string
		for id := range m.Actors {
			if id == m.JudgeID && !(m.SuddenDeath && containsID(m.TieIDs, id)) {
				continue
			}
			if m.SuddenDeath && !containsID(m.TieIDs, id) {
				continue
			}
			ids = append(ids, id)
		}
		sort.Strings(ids)
		packets = make([]packet, 0, len(ids))
		for _, id := range ids {
			p, ok := byActor[id]
			if !ok {
				p = packet{ActorID: id}
			}
			packets = append(packets, p)
		}
	}
	for _, p := range packets {
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
			Voted:    p.ActorID == votedFor,
			Votes:    counts[p.ActorID],
		}
		if showCounts && p.Revealed {
			pv.ShowVote = pv.Votes
			pv.VoteLabel = favoriteVoteLabel(pv.Votes)
		} else {
			pv.ShowVote = -1
		}
		if m.NamesShown {
			pv.Revealed = true
			pv.FavoriteMark = favorites[p.ActorID]
		}
		out = append(out, pv)
	}
	return out
}

func containsID(ids []string, target string) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}

func (g *Game) timerViewLocked(m *matchState) (string, string, int, int, int64) {
	if m == nil || m.TimerKind == "" {
		return "", "", 0, 0, 0
	}
	left := m.FrozenLeft
	endUnix := int64(0)
	if !g.paused && !m.TimerEnd.IsZero() {
		left = m.TimerEnd.Sub(g.clock())
		endUnix = m.TimerEnd.UnixMilli()
	}
	if left < 0 {
		left = 0
	}
	seconds := int((left + time.Second - 1) / time.Second)
	total := int((m.TimerTotal + time.Second - 1) / time.Second)
	label := map[string]string{
		"auto-draw":       "Auto-draw",
		"submit":          "Submit",
		"between-reveals": "Next reveal",
		"judge-pick":      "Judge picks",
		"finish":          "Back to lobby",
	}[m.TimerKind]
	return label, fmt.Sprintf("%d:%02d", seconds/60, seconds%60), seconds, total, endUnix
}

func favoriteVoteLabel(votes int) string {
	if votes == 1 {
		return "1 favorite vote"
	}
	return fmt.Sprintf("%d favorite votes", votes)
}

func favoriteMarks(counts map[string]int) map[string]string {
	type row struct {
		id    string
		votes int
	}
	var rows []row
	for id, votes := range counts {
		if votes > 0 {
			rows = append(rows, row{id: id, votes: votes})
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].votes == rows[j].votes {
			return rows[i].id < rows[j].id
		}
		return rows[i].votes > rows[j].votes
	})
	marks := map[string]string{}
	place := 0
	for i := 0; i < len(rows) && place < 3; {
		votes := rows[i].votes
		start := i
		for i < len(rows) && rows[i].votes == votes {
			i++
		}
		label := []string{"Favorite 1st", "Favorite 2nd", "Favorite 3rd"}[place]
		for _, tied := range rows[start:i] {
			marks[tied.id] = label
		}
		place += i - start
	}
	return marks
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
