package apples

import (
	"fmt"
	"math/rand"
	"slices"
	"strings"
	"time"

	"github.com/KroniK907/grabbag/internal/games"
)

const eventMatch = "apples"

type phase string

const (
	phaseDrawWait phase = "draw-wait"
	phaseChoose   phase = "choose"
	phaseSubmit   phase = "submit"
	phaseReveal   phase = "reveal"
	phaseSudden   phase = "sudden-death"
	phaseOver     phase = "over"
)

type actor struct {
	ID      string
	Name    string
	Bot     bool
	BotNum  int
	Score   int
	Hand    []playCard
	Blank   *playCard
	Draft   string
	Holes   []*playCard
	Discard *playCard
	Locked  bool
}

type packet struct {
	ActorID  string
	Cards    []playCard
	Revealed bool
}

type matchState struct {
	Settings        matchSettings
	WildcardAtStart bool
	Phase           phase
	Round           int
	InMultiplier    bool
	PendingFinish   bool
	SuddenDeath     bool
	TieIDs          []string
	JudgeCycle      []string
	JudgeIdx        int
	JudgeID         string
	Actors          map[string]*actor
	NextBot         int
	Prompts         []playPrompt
	Answers         []playCard
	LivePrompt      *playPrompt
	Choice          [2]*playPrompt
	Packets         []packet
	Votes           map[string]string
	WinnerID        string
	NamesShown      bool
	TimerKind       string
	TimerEnd        time.Time
	FrozenLeft      time.Duration
	PhoneErr        map[string]string
}

func filledCount(holes []*playCard) int {
	n := 0
	for _, h := range holes {
		if h != nil {
			n++
		}
	}
	return n
}

func firstEmptyHole(holes []*playCard) int {
	for i, h := range holes {
		if h == nil {
			return i
		}
	}
	return -1
}

func holeCards(holes []*playCard) []playCard {
	var out []playCard
	for _, h := range holes {
		if h != nil {
			out = append(out, *h)
		}
	}
	return out
}

func (g *Game) rngLocked() *rand.Rand {
	if g.rng == nil {
		g.rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	return g.rng
}

func (g *Game) clock() time.Time {
	if g.now != nil {
		return g.now()
	}
	return time.Now()
}

func (g *Game) publishLocked() {
	if g.helper != nil {
		g.helper.Publish(eventMatch)
	}
}

func (g *Game) beginMatchLocked(h games.Helper) error {
	cat := scanDataDir(h.DataDir())
	settings, ok := g.loadSettings(h)
	settings = reconcileSettings(settings, ok, cat)
	seated := h.Seated()
	if len(seated) < 1 {
		return fmt.Errorf("Start needs a seated player.")
	}
	piles := buildPiles(cat, settings)
	if msg := startShortage(piles, settings, len(seated)); msg != "" {
		return fmt.Errorf("%s", msg)
	}

	m := &matchState{
		Settings:        settings,
		WildcardAtStart: settings.wildcardLibraryOn(),
		Phase:           phaseDrawWait,
		Actors:          map[string]*actor{},
		Votes:           map[string]string{},
		PhoneErr:        map[string]string{},
		Prompts:         append([]playPrompt(nil), piles.Prompts...),
		Answers:         append([]playCard(nil), piles.Answers...),
		NextBot:         1,
	}
	g.rngLocked().Shuffle(len(m.Prompts), func(i, j int) { m.Prompts[i], m.Prompts[j] = m.Prompts[j], m.Prompts[i] })
	g.rngLocked().Shuffle(len(m.Answers), func(i, j int) { m.Answers[i], m.Answers[j] = m.Answers[j], m.Answers[i] })

	ids := make([]string, 0, len(seated))
	for _, p := range seated {
		ids = append(ids, p.ID)
		m.Actors[p.ID] = &actor{ID: p.ID, Name: p.DisplayName}
	}
	g.rngLocked().Shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })
	m.JudgeCycle = ids
	m.JudgeIdx = 0
	m.JudgeID = ids[0]

	needBots := settings.BotCount
	if len(seated)+needBots < 3 {
		needBots = 3 - len(seated)
	}
	for i := 0; i < needBots; i++ {
		g.spawnBotLocked(m)
	}
	for _, p := range seated {
		g.dealHumanLocked(m, m.Actors[p.ID], settings.HandSize)
	}
	g.match = m
	g.started = true
	g.paused = false
	g.armTimerLocked(m, "auto-draw", settings.AutoDrawSec)
	g.startTickerLocked()
	return nil
}

func (g *Game) spawnBotLocked(m *matchState) *actor {
	n := m.NextBot
	m.NextBot++
	id := fmt.Sprintf("bot:%d", n)
	a := &actor{ID: id, Name: fmt.Sprintf("Bot %d", n), Bot: true, BotNum: n}
	m.Actors[id] = a
	return a
}

func (g *Game) dealHumanLocked(m *matchState, a *actor, handSize int) {
	for len(a.Hand) < handSize {
		card, ok := g.popAnswerLocked(m)
		if !ok {
			break
		}
		a.Hand = append(a.Hand, card)
	}
	if m.WildcardAtStart && a.Blank == nil {
		blank := playCard{LibraryID: wildcardLibraryID, CardID: blankCardID, Blank: true, Wildcard: true}
		a.Blank = &blank
	}
}

func (g *Game) popAnswerLocked(m *matchState) (playCard, bool) {
	if len(m.Answers) == 0 {
		return playCard{}, false
	}
	n := len(m.Answers) - 1
	card := m.Answers[n]
	m.Answers = m.Answers[:n]
	return card, true
}

func (g *Game) popDealablePromptLocked(m *matchState) (playPrompt, bool) {
	for i, p := range m.Prompts {
		if p.dealable(m.Settings.HandSize) {
			m.Prompts = slices.Delete(m.Prompts, i, i+1)
			return p, true
		}
	}
	return playPrompt{}, false
}

func (g *Game) syncRosterLocked(h games.Helper) {
	if g.match == nil || !g.started {
		return
	}
	m := g.match
	seated := h.Seated()
	live := map[string]games.Player{}
	for _, p := range seated {
		live[p.ID] = p
		if _, ok := m.Actors[p.ID]; !ok {
			m.Actors[p.ID] = &actor{ID: p.ID, Name: p.DisplayName}
			m.JudgeCycle = append(m.JudgeCycle, p.ID)
			if m.Phase == phaseSubmit {
				g.dealHumanLocked(m, m.Actors[p.ID], m.Settings.HandSize)
			}
		} else {
			m.Actors[p.ID].Name = p.DisplayName
		}
	}
	humans := 0
	bots := 0
	for id, a := range m.Actors {
		if a.Bot {
			bots++
			continue
		}
		if _, ok := live[id]; !ok {
			a.Hand = nil
			a.Blank = nil
			a.Holes = nil
			a.Locked = true
			continue
		}
		humans++
	}
	if humans == 0 {
		g.needPause = true
		return
	}
	for humans+bots < 3 {
		bot := g.spawnBotLocked(m)
		bots++
		if m.Phase == phaseSubmit && m.LivePrompt != nil {
			g.dealBotSubmitLocked(m, bot)
		}
		g.publishLocked()
	}
	if _, ok := live[m.JudgeID]; !ok && m.JudgeID != "" && !m.SuddenDeath {
		g.advanceJudgeLocked(m, live)
	}
}

func (g *Game) advanceJudgeLocked(m *matchState, live map[string]games.Player) {
	if len(m.JudgeCycle) == 0 {
		return
	}
	for n := 0; n < len(m.JudgeCycle); n++ {
		m.JudgeIdx = (m.JudgeIdx + 1) % len(m.JudgeCycle)
		id := m.JudgeCycle[m.JudgeIdx]
		if _, ok := live[id]; ok {
			m.JudgeID = id
			return
		}
	}
	m.JudgeID = ""
}

func (g *Game) seatedLiveLocked(h games.Helper) map[string]games.Player {
	live := map[string]games.Player{}
	if h == nil {
		return live
	}
	for _, p := range h.Seated() {
		live[p.ID] = p
	}
	return live
}

func (g *Game) drawLocked(m *matchState) error {
	if m.Phase != phaseDrawWait && m.Phase != phaseSudden {
		return fmt.Errorf("Draw waits until this round is over.")
	}
	m.WinnerID = ""
	m.NamesShown = false
	m.Packets = nil
	m.Votes = map[string]string{}
	m.PhoneErr = map[string]string{}
	m.LivePrompt = nil
	m.Choice = [2]*playPrompt{}
	if m.Settings.PromptMode == modeMulti {
		p1, ok1 := g.popDealablePromptLocked(m)
		p2, ok2 := g.popDealablePromptLocked(m)
		if !ok1 || !ok2 {
			return fmt.Errorf("%s", startRefuseMsg)
		}
		m.Choice[0], m.Choice[1] = &p1, &p2
		m.Phase = phaseChoose
		g.clearTimerLocked(m)
		return nil
	}
	p, ok := g.popDealablePromptLocked(m)
	if !ok {
		return fmt.Errorf("%s", startRefuseMsg)
	}
	m.LivePrompt = &p
	g.enterSubmitLocked(m)
	return nil
}

func (g *Game) skipLocked(m *matchState) error {
	if m.Settings.PromptMode != modeSkip || m.LivePrompt == nil {
		return fmt.Errorf("Skip is not available.")
	}
	if m.Phase != phaseSubmit {
		return fmt.Errorf("Skip is not available.")
	}
	for _, a := range m.Actors {
		if a.Locked && !a.Bot && a.ID != m.JudgeID {
			return fmt.Errorf("Skip is gone after the first lock.")
		}
	}
	p, ok := g.popDealablePromptLocked(m)
	if !ok {
		return fmt.Errorf("%s", startRefuseMsg)
	}
	m.LivePrompt = &p
	g.resetSubmitLocked(m)
	g.enterSubmitLocked(m)
	return nil
}

func (g *Game) choosePromptLocked(m *matchState, cardID string) error {
	if m.Phase != phaseChoose {
		return fmt.Errorf("No prompt to choose.")
	}
	var keep, drop *playPrompt
	for i := 0; i < 2; i++ {
		if m.Choice[i] == nil {
			continue
		}
		if m.Choice[i].CardID == cardID {
			keep = m.Choice[i]
		} else {
			drop = m.Choice[i]
		}
	}
	if keep == nil {
		return fmt.Errorf("That prompt is not one of the choices.")
	}
	if drop != nil {
		m.Prompts = append(m.Prompts, *drop)
	}
	m.Choice = [2]*playPrompt{}
	m.LivePrompt = keep
	g.enterSubmitLocked(m)
	return nil
}

func (g *Game) resetSubmitLocked(m *matchState) {
	for _, a := range m.Actors {
		a.Holes = nil
		a.Discard = nil
		a.Locked = false
		if a.Bot {
			a.Hand = nil
		}
	}
	m.Packets = nil
}

func (g *Game) enterSubmitLocked(m *matchState) {
	m.Phase = phaseSubmit
	live := g.seatedLiveLocked(g.helper)
	for _, a := range m.Actors {
		a.Holes = nil
		a.Discard = nil
		a.Locked = false
		if a.Bot {
			g.dealBotSubmitLocked(m, a)
			continue
		}
		if _, ok := live[a.ID]; !ok {
			a.Locked = true
			continue
		}
		if m.SuddenDeath && !slices.Contains(m.TieIDs, a.ID) && a.ID != m.JudgeID {
			a.Locked = true
			continue
		}
		g.dealHumanLocked(m, a, m.Settings.HandSize)
		a.Holes = make([]*playCard, promptPick(m.LivePrompt))
	}
	g.armTimerLocked(m, "submit", m.Settings.SubmitSec)
	g.maybeRevealLocked(m)
}

func (g *Game) dealBotSubmitLocked(m *matchState, a *actor) {
	a.Hand = nil
	a.Blank = nil
	pick := promptPick(m.LivePrompt)
	a.Holes = make([]*playCard, pick)
	for i := 0; i < pick; i++ {
		card, ok := g.popAnswerLocked(m)
		if !ok {
			break
		}
		c := card
		a.Holes[i] = &c
	}
	a.Locked = filledCount(a.Holes) == pick
	if a.Locked {
		g.recordPacketLocked(m, a)
	}
}

func (g *Game) canSubmit(m *matchState, id string) bool {
	if m == nil || m.Phase != phaseSubmit || m.LivePrompt == nil {
		return false
	}
	a := m.Actors[id]
	if a == nil || a.Bot || a.Locked {
		return false
	}
	if id == m.JudgeID && !(m.SuddenDeath && slices.Contains(m.TieIDs, id)) {
		return false
	}
	if m.SuddenDeath && !slices.Contains(m.TieIDs, id) {
		return false
	}
	return true
}

func (g *Game) slotLocked(m *matchState, id, cardID string) error {
	a := m.Actors[id]
	if a == nil || !g.canSubmit(m, id) {
		return fmt.Errorf("You cannot play a card now.")
	}
	pick := promptPick(m.LivePrompt)
	if len(a.Holes) != pick {
		a.Holes = make([]*playCard, pick)
	}
	slot := firstEmptyHole(a.Holes)
	if slot < 0 {
		return fmt.Errorf("Every hole is filled.")
	}
	card, handIdx, ok := takeCard(a, cardID)
	if !ok {
		return fmt.Errorf("That card is not in your hand.")
	}
	if card.Blank {
		if strings.TrimSpace(a.Draft) == "" {
			putCardBack(a, handIdx, card)
			return fmt.Errorf("Type an answer")
		}
		if err := g.wildcardCheckLocked(m, a.Draft); err != nil {
			putCardBack(a, handIdx, card)
			return err
		}
		card.Text = strings.TrimSpace(a.Draft)
		card.Wildcard = true
		card.Blank = false
		card.CardID = wildcardID(card.Text)
		a.Draft = ""
	}
	c := card
	a.Holes[slot] = &c
	return nil
}

func takeCard(a *actor, cardID string) (playCard, int, bool) {
	for i, c := range a.Hand {
		if c.CardID == cardID {
			a.Hand = slices.Delete(a.Hand, i, i+1)
			return c, i, true
		}
	}
	if a.Blank != nil && (cardID == blankCardID || a.Blank.CardID == cardID) {
		c := *a.Blank
		a.Blank = nil
		return c, -1, true
	}
	return playCard{}, -1, false
}

func putCardBack(a *actor, handIdx int, card playCard) {
	if card.Blank {
		a.Blank = &card
		return
	}
	if handIdx < 0 || handIdx > len(a.Hand) {
		a.Hand = append(a.Hand, card)
		return
	}
	a.Hand = slices.Insert(a.Hand, handIdx, card)
}

func (g *Game) unslotLocked(m *matchState, id string, hole int) error {
	a := m.Actors[id]
	if a == nil || !g.canSubmit(m, id) {
		return fmt.Errorf("You cannot play a card now.")
	}
	if hole < 0 || hole >= len(a.Holes) || a.Holes[hole] == nil {
		return fmt.Errorf("That hole is empty.")
	}
	card := *a.Holes[hole]
	a.Holes[hole] = nil
	if card.Wildcard {
		a.Draft = card.Text
		blank := playCard{LibraryID: wildcardLibraryID, CardID: blankCardID, Blank: true, Wildcard: true}
		a.Blank = &blank
		return nil
	}
	a.Hand = append(a.Hand, card)
	return nil
}

func (g *Game) markDiscardLocked(m *matchState, id, cardID string) error {
	a := m.Actors[id]
	if a == nil || !g.canSubmit(m, id) {
		return fmt.Errorf("You cannot discard now.")
	}
	hasWild := false
	for _, c := range a.Holes {
		if c != nil && c.Wildcard {
			hasWild = true
			break
		}
	}
	if !hasWild {
		return fmt.Errorf("Discard is only for leftover printed cards.")
	}
	if a.Discard != nil && a.Discard.CardID == cardID {
		a.Hand = append(a.Hand, *a.Discard)
		a.Discard = nil
		return nil
	}
	for i, c := range a.Hand {
		if c.CardID == cardID && !c.Wildcard {
			if a.Discard != nil {
				a.Hand = append(a.Hand, *a.Discard)
			}
			a.Hand = slices.Delete(a.Hand, i, i+1)
			cp := c
			a.Discard = &cp
			return nil
		}
	}
	return fmt.Errorf("That card is not in your hand.")
}
