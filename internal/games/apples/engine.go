package apples

import (
	"errors"
	"math/rand"
	"slices"
	"time"
)

const eventMatch = "apples"

type phase string

const (
	phaseDrawWait phase = "draw-wait"
	phaseChoose   phase = "choose"
	phaseHold     phase = "hold"
	phaseSubmit   phase = "submit"
	phaseReveal   phase = "reveal"
	phaseSudden   phase = "sudden-death"
	phaseOver     phase = "over"
)

const (
	timerAutoDraw     = "auto-draw"
	timerSubmit       = "submit"
	timerBetween      = "between-reveals"
	timerFavoriteVote = "favorite-vote"
	timerJudgePick    = "judge-pick"
	timerFinish       = "finish"
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

type commandKind int

const (
	cmdDraw commandKind = iota
	cmdSkip
	cmdKeepPrompt
	cmdChoosePrompt
	cmdSlot
	cmdUnslot
	cmdDiscard
	cmdDraft
	cmdLock
	cmdReveal
	cmdVote
	cmdConfirm
)

// command is one player action. Card holds a card, prompt, vote target, or
// winner id depending on Kind.
type command struct {
	Kind  commandKind
	Actor string
	Card  string
	Hole  int
	Text  string
}

// outcome is what the Game must do after the engine changes state. The
// engine never touches the helper, journals, or disk.
type outcome struct {
	Err error
	// Played cards go to the discard journal.
	Played []playedRow
	// FlushDiscard asks the Game to write the discard journal before replying.
	FlushDiscard bool
	// Wildcards are round wildcards that the save policy keeps.
	Wildcards []playCard
	// Shortage is the reshuffle overlay kind when the piles ran dry but the
	// discard journal would cover the deal.
	Shortage string
	// TooSmall means a reshuffle still left the piles short.
	TooSmall   bool
	BurnDrawer bool
	Publish    bool
	Pause      bool
	Finish     bool
}

// cardSupply is the engine's view of cards outside the match: the enabled
// library on disk, burns, and the discard journal.
type cardSupply interface {
	// unplayed returns enabled cards that are neither burned nor played.
	unplayed(s matchSettings) enabledPiles
	// recycleCovers reports whether reshuffling the discard journal would
	// leave needP dealable prompts and needA answers outside inHand.
	recycleCovers(s matchSettings, inHand map[string]bool, needP, needA int) bool
	// wildcardTexts is every saved wildcard answer.
	wildcardTexts() []string
}

type rosterRow struct {
	ID   string
	Name string
}

// engine is one Apples match: rounds, hands, judging, scoring, and timers.
type engine struct {
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
	SeatOrder       []string
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
	TimerTotal      time.Duration
	FrozenLeft      time.Duration
	PhoneErr        map[string]string
	// Live is the seated Lobby roster. Seated players missing from it sit out.
	Live   map[string]bool
	Paused bool

	supply cardSupply
	rng    *rand.Rand
	at     time.Time
	out    outcome
}

func (e *engine) begin(now time.Time) {
	e.at = now
	e.out = outcome{}
}

func (e *engine) done() outcome {
	out := e.out
	e.out = outcome{}
	return out
}

func (e *engine) fail(err error) outcome {
	e.out.Err = err
	return e.done()
}

func (e *engine) played(libraryID, cardID string) {
	e.out.Played = append(e.out.Played, playedRow{LibraryID: libraryID, CardID: cardID})
}

func (e *engine) shuffle(n int, swap func(i, j int)) {
	if e.rng == nil {
		e.rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	e.rng.Shuffle(n, swap)
}

func (e *engine) intn(n int) int {
	if e.rng == nil {
		e.rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	return e.rng.Intn(n)
}

// Begin seats the roster, fills bots, and deals opening hands from deal.
// Shortage is pendingStart when the piles are short but a reshuffle would
// cover them.
func (e *engine) Begin(settings matchSettings, roster []rosterRow, deal enabledPiles, supply cardSupply, rng *rand.Rand, now time.Time) outcome {
	*e = engine{
		Settings:        settings,
		WildcardAtStart: settings.wildcardLibraryOn(),
		Phase:           phaseDrawWait,
		Actors:          map[string]*actor{},
		Votes:           map[string]string{},
		PhoneErr:        map[string]string{},
		Prompts:         append([]playPrompt(nil), deal.Prompts...),
		Answers:         append([]playCard(nil), deal.Answers...),
		NextBot:         1,
		supply:          supply,
		rng:             rng,
	}
	e.begin(now)
	e.shuffle(len(e.Prompts), func(i, j int) { e.Prompts[i], e.Prompts[j] = e.Prompts[j], e.Prompts[i] })
	e.shuffle(len(e.Answers), func(i, j int) { e.Answers[i], e.Answers[j] = e.Answers[j], e.Answers[i] })
	e.SetSeated(roster)
	ids := make([]string, 0, len(roster))
	for _, row := range roster {
		ids = append(ids, row.ID)
		e.Actors[row.ID] = &actor{ID: row.ID, Name: row.Name}
		e.SeatOrder = append(e.SeatOrder, row.ID)
	}
	e.shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })
	e.JudgeCycle = ids
	if len(ids) > 0 {
		e.JudgeID = ids[0]
	}
	needBots := settings.BotCount
	if len(roster)+needBots < 3 {
		needBots = 3 - len(roster)
	}
	for i := 0; i < needBots; i++ {
		e.spawnBot()
	}
	needP, needA := openingNeeds(settings, len(roster))
	if e.shortage(pendingStart, needP, needA) {
		return e.done()
	}
	for _, row := range roster {
		e.dealHuman(e.Actors[row.ID])
	}
	e.armTimer(timerAutoDraw, settings.AutoDrawSec)
	return e.done()
}

// SetSeated replaces the live seated roster without dealing or bot fill.
func (e *engine) SetSeated(roster []rosterRow) {
	e.Live = map[string]bool{}
	for _, row := range roster {
		e.Live[row.ID] = true
	}
}

// Do applies one player command.
func (e *engine) Do(cmd command, now time.Time) outcome {
	e.begin(now)
	var err error
	switch cmd.Kind {
	case cmdDraw:
		err = e.judgeOnly(cmd, "Only the judge can draw.", e.draw)
	case cmdSkip:
		err = e.judgeOnly(cmd, "Only the judge can skip.", e.skip)
	case cmdKeepPrompt:
		err = e.judgeOnly(cmd, "Only the judge can lock in the prompt.", e.keepPrompt)
	case cmdChoosePrompt:
		err = e.judgeOnly(cmd, "Only the judge can choose.", func() error { return e.choosePrompt(cmd.Card) })
	case cmdSlot:
		err = e.slot(cmd.Actor, cmd.Card)
	case cmdUnslot:
		err = e.unslot(cmd.Actor, cmd.Hole)
	case cmdDiscard:
		err = e.markDiscard(cmd.Actor, cmd.Card)
	case cmdDraft:
		err = e.setDraft(cmd.Actor, cmd.Text)
	case cmdLock:
		err = e.lock(cmd.Actor)
	case cmdReveal:
		err = e.judgeOnly(cmd, "Only the judge can reveal.", e.revealNext)
	case cmdVote:
		err = e.vote(cmd.Actor, cmd.Card)
	case cmdConfirm:
		err = e.judgeOnly(cmd, "Only the judge can confirm.", func() error { return e.confirm(cmd.Card) })
	}
	if err != nil {
		return e.fail(err)
	}
	return e.done()
}

func (e *engine) judgeOnly(cmd command, refuse string, fn func() error) error {
	if cmd.Actor != e.JudgeID {
		return errors.New(refuse)
	}
	return fn()
}

// Advance fires the running timer once it has expired.
func (e *engine) Advance(now time.Time) outcome {
	e.begin(now)
	if e.Paused || e.TimerKind == "" || e.TimerEnd.IsZero() || e.at.Before(e.TimerEnd) {
		return e.done()
	}
	kind := e.TimerKind
	e.clearTimer()
	switch kind {
	case timerAutoDraw:
		_ = e.draw()
	case timerSubmit:
		e.timerSubmit()
	case timerBetween:
		_ = e.revealNext()
	case timerFavoriteVote:
		e.armTimer(timerJudgePick, e.Settings.JudgePickSec)
	case timerJudgePick:
		e.autoPick()
	case timerFinish:
		e.out.Finish = true
	}
	e.out.Publish = true
	return e.done()
}

// SetPaused freezes or thaws the running timer. The Game holds the engine
// paused while the host is paused or the reshuffle overlay is up.
func (e *engine) SetPaused(paused bool, now time.Time) {
	e.at = now
	e.Paused = paused
	if paused {
		e.freezeTimer()
		return
	}
	e.thawTimer()
}

// SyncRoster adds late sitters, benches leavers, tops up bots to three
// players, and moves the judge off a missing player.
func (e *engine) SyncRoster(roster []rosterRow, now time.Time) outcome {
	e.begin(now)
	e.SetSeated(roster)
	for _, row := range roster {
		if a, ok := e.Actors[row.ID]; ok {
			a.Name = row.Name
			continue
		}
		e.Actors[row.ID] = &actor{ID: row.ID, Name: row.Name}
		e.JudgeCycle = append(e.JudgeCycle, row.ID)
		e.appendSeat(row.ID)
		if e.Phase == phaseSubmit {
			e.dealHuman(e.Actors[row.ID])
		}
	}
	humans := 0
	bots := 0
	for id, a := range e.Actors {
		if a.Bot {
			bots++
			continue
		}
		if !e.Live[id] {
			a.Hand = nil
			a.Blank = nil
			a.Holes = nil
			a.Locked = true
			continue
		}
		humans++
	}
	if humans == 0 {
		e.out.Pause = true
		return e.done()
	}
	for humans+bots < 3 {
		if e.Phase == phaseSubmit && e.LivePrompt != nil {
			if e.shortage(pendingBotFill, 0, promptPick(e.LivePrompt)) {
				break
			}
		}
		bot := e.spawnBot()
		bots++
		if e.Phase == phaseSubmit && e.LivePrompt != nil {
			e.dealBotSubmit(bot)
		}
		e.out.Publish = true
	}
	if !e.Live[e.JudgeID] && e.JudgeID != "" && !e.SuddenDeath {
		e.advanceJudge()
	}
	return e.done()
}

// Reshuffled retries the deal that raised the overlay of kind. The Game has
// already emptied the discard journal.
func (e *engine) Reshuffled(kind string, now time.Time) outcome {
	e.begin(now)
	e.refill()
	switch kind {
	case pendingStart:
		for _, a := range e.Actors {
			if !a.Bot {
				e.dealHuman(a)
			}
		}
		if e.humanHandsShort() {
			e.out.TooSmall = true
		}
	case pendingDraw, pendingMulti:
		if err := e.draw(); err != nil {
			e.out.TooSmall = true
		}
	case pendingSkip:
		if err := e.skipAfterReshuffle(); err != nil {
			e.out.TooSmall = true
		}
	case pendingBotFill:
		e.fillWaitingBots()
	}
	return e.done()
}

// StripBurned drops burned cards from the draw piles.
func (e *engine) StripBurned(burned func(kind, text string) bool) {
	e.Prompts = slices.DeleteFunc(e.Prompts, func(p playPrompt) bool { return burned(kindPrompt, p.Text) })
	e.Answers = slices.DeleteFunc(e.Answers, func(c playCard) bool { return burned(kindAnswer, c.Text) })
}

// Refill rebuilds both draw piles from the supply, as after an unburn.
func (e *engine) Refill() {
	e.refill()
}

// AddAnswers puts newly saved wildcards into the answer pile.
func (e *engine) AddAnswers(cards []playCard) {
	e.Answers = append(e.Answers, cards...)
}
