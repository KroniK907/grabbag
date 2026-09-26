package apples

import (
	"fmt"
	"slices"
)

// Reshuffle overlay kinds: the deal that ran dry and waits on the host.
const (
	pendingStart   = "start"
	pendingDraw    = "draw"
	pendingSkip    = "skip"
	pendingMulti   = "multi"
	pendingBotFill = "bot-fill"
)

func (e *engine) draw() error {
	if e.Phase != phaseDrawWait && e.Phase != phaseSudden {
		return fmt.Errorf("Draw waits until this round is over.")
	}
	e.refill()
	needP, needA := e.drawNeed()
	kind := pendingDraw
	if e.Settings.PromptMode == modeMulti {
		kind = pendingMulti
	}
	if e.shortage(kind, needP, needA) {
		return nil
	}
	e.WinnerID = ""
	e.NamesShown = false
	e.Packets = nil
	e.Votes = map[string]string{}
	e.PhoneErr = map[string]string{}
	e.LivePrompt = nil
	e.Choice = [2]*playPrompt{}
	if e.Settings.PromptMode == modeMulti {
		p1, ok1 := e.popDealablePrompt()
		p2, ok2 := e.popDealablePrompt()
		if !ok1 || !ok2 {
			return fmt.Errorf("%s", startRefuseMsg)
		}
		e.Choice[0], e.Choice[1] = &p1, &p2
		e.Phase = phaseChoose
		e.clearTimer()
		return nil
	}
	p, ok := e.popDealablePrompt()
	if !ok {
		return fmt.Errorf("%s", startRefuseMsg)
	}
	e.LivePrompt = &p
	e.played(p.LibraryID, p.CardID)
	if e.Settings.PromptMode == modeSkip {
		e.Phase = phaseHold
		e.clearTimer()
		return nil
	}
	e.enterSubmit()
	return nil
}

func (e *engine) skip() error {
	if e.Settings.PromptMode != modeSkip || e.LivePrompt == nil || e.Phase != phaseHold {
		return fmt.Errorf("Skip is not available.")
	}
	if e.shortage(pendingSkip, 1, 0) {
		return nil
	}
	return e.skipAfterReshuffle()
}

func (e *engine) skipAfterReshuffle() error {
	if e.LivePrompt == nil {
		return fmt.Errorf("Skip is not available.")
	}
	old := *e.LivePrompt
	p, ok := e.popDealablePrompt()
	if !ok {
		return fmt.Errorf("%s", startRefuseMsg)
	}
	e.played(old.LibraryID, old.CardID)
	e.LivePrompt = &p
	e.played(p.LibraryID, p.CardID)
	if e.Phase == phaseSubmit {
		e.resetSubmit()
		e.enterSubmit()
		return nil
	}
	e.Phase = phaseHold
	e.clearTimer()
	return nil
}

func (e *engine) keepPrompt() error {
	if e.Settings.PromptMode != modeSkip || e.Phase != phaseHold || e.LivePrompt == nil {
		return fmt.Errorf("There is no prompt to lock in.")
	}
	e.enterSubmit()
	return nil
}

func (e *engine) choosePrompt(cardID string) error {
	if e.Phase != phaseChoose {
		return fmt.Errorf("No prompt to choose.")
	}
	var keep, drop *playPrompt
	for _, c := range e.Choice {
		if c == nil {
			continue
		}
		if c.CardID == cardID {
			keep = c
		} else {
			drop = c
		}
	}
	if keep == nil {
		return fmt.Errorf("That prompt is not one of the choices.")
	}
	if drop != nil {
		e.Prompts = append(e.Prompts, *drop)
	}
	e.Choice = [2]*playPrompt{}
	e.LivePrompt = keep
	e.played(keep.LibraryID, keep.CardID)
	e.enterSubmit()
	return nil
}

// drawNeed is the cards the next draw deals: prompts for the judge, refills
// for live human hands, and a full packet per bot.
func (e *engine) drawNeed() (needP, needA int) {
	needP = 1
	if e.Settings.PromptMode == modeMulti {
		needP = 2
	}
	maxPick := 1
	for _, p := range e.Prompts {
		if p.dealable(e.Settings.HandSize) && p.Pick > maxPick {
			maxPick = p.Pick
		}
	}
	for _, a := range e.Actors {
		if a.Bot {
			needA += maxPick
			continue
		}
		if !e.Live[a.ID] {
			continue
		}
		if n := e.Settings.HandSize - len(a.Hand); n > 0 {
			needA += n
		}
	}
	return
}

// shortage raises the reshuffle overlay of kind when the piles cannot cover
// the deal but the discard journal can.
func (e *engine) shortage(kind string, needP, needA int) bool {
	haveP := dealablePromptCount(enabledPiles{Prompts: e.Prompts}, e.Settings.HandSize)
	if haveP >= needP && len(e.Answers) >= needA {
		return false
	}
	if e.supply == nil || !e.supply.recycleCovers(e.Settings, e.inHandKeys(), needP, needA) {
		return false
	}
	e.out.Shortage = kind
	return true
}

// refill rebuilds both piles from the supply, minus cards already in play.
func (e *engine) refill() {
	if e.supply == nil {
		return
	}
	piles := e.supply.unplayed(e.Settings)
	hands := e.inHandKeys()
	var prompts []playPrompt
	var answers []playCard
	for _, p := range piles.Prompts {
		if !hands[p.key()] {
			prompts = append(prompts, p)
		}
	}
	for _, a := range piles.Answers {
		if !hands[a.key()] {
			answers = append(answers, a)
		}
	}
	e.shuffle(len(prompts), func(i, j int) { prompts[i], prompts[j] = prompts[j], prompts[i] })
	e.shuffle(len(answers), func(i, j int) { answers[i], answers[j] = answers[j], answers[i] })
	e.Prompts = prompts
	e.Answers = answers
}

func (e *engine) inHandKeys() map[string]bool {
	out := map[string]bool{}
	for _, a := range e.Actors {
		for _, c := range a.Hand {
			out[c.key()] = true
		}
		for _, h := range a.Holes {
			if h != nil {
				out[h.key()] = true
			}
		}
		if a.Discard != nil {
			out[a.Discard.key()] = true
		}
	}
	return out
}

func (e *engine) popAnswer() (playCard, bool) {
	if len(e.Answers) == 0 {
		return playCard{}, false
	}
	n := len(e.Answers) - 1
	card := e.Answers[n]
	e.Answers = e.Answers[:n]
	return card, true
}

func (e *engine) popDealablePrompt() (playPrompt, bool) {
	for i, p := range e.Prompts {
		if p.dealable(e.Settings.HandSize) {
			e.Prompts = slices.Delete(e.Prompts, i, i+1)
			return p, true
		}
	}
	return playPrompt{}, false
}

func (e *engine) dealHuman(a *actor) {
	for len(a.Hand) < e.Settings.HandSize {
		card, ok := e.popAnswer()
		if !ok {
			break
		}
		a.Hand = append(a.Hand, card)
	}
	if e.WildcardAtStart && a.Blank == nil {
		a.Blank = blankCard()
	}
}

func (e *engine) humanHandsShort() bool {
	for _, a := range e.Actors {
		if !a.Bot && len(a.Hand) < e.Settings.HandSize {
			return true
		}
	}
	return false
}

func (e *engine) spawnBot() *actor {
	n := e.NextBot
	e.NextBot++
	id := fmt.Sprintf("bot:%d", n)
	a := &actor{ID: id, Name: fmt.Sprintf("Bot %d", n), Bot: true, BotNum: n}
	e.Actors[id] = a
	e.appendSeat(id)
	return a
}

func (e *engine) appendSeat(id string) {
	if !slices.Contains(e.SeatOrder, id) {
		e.SeatOrder = append(e.SeatOrder, id)
	}
}

func blankCard() *playCard {
	return &playCard{LibraryID: wildcardLibraryID, CardID: blankCardID, Blank: true, Wildcard: true}
}
