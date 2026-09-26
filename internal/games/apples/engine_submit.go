package apples

import (
	"fmt"
	"slices"
	"strings"
)

func (e *engine) resetSubmit() {
	for _, a := range e.Actors {
		a.Holes = nil
		a.Discard = nil
		a.Locked = false
		if a.Bot {
			a.Hand = nil
		}
	}
	e.Packets = nil
}

// enterSubmit opens the submit phase: bots play at once, benched and
// non-tied players sit out, and everyone else gets a full hand and empty holes.
func (e *engine) enterSubmit() {
	e.Phase = phaseSubmit
	for _, a := range e.Actors {
		a.Holes = nil
		a.Discard = nil
		a.Locked = false
		if a.Bot {
			e.dealBotSubmit(a)
			continue
		}
		if !e.Live[a.ID] {
			a.Locked = true
			continue
		}
		if e.SuddenDeath && !slices.Contains(e.TieIDs, a.ID) && a.ID != e.JudgeID {
			a.Locked = true
			continue
		}
		e.dealHuman(a)
		a.Holes = make([]*playCard, promptPick(e.LivePrompt))
	}
	e.armTimer(timerSubmit, e.Settings.SubmitSec)
	e.maybeReveal()
}

func (e *engine) dealBotSubmit(a *actor) {
	a.Hand = nil
	a.Blank = nil
	pick := promptPick(e.LivePrompt)
	a.Holes = make([]*playCard, pick)
	for i := 0; i < pick; i++ {
		card, ok := e.popAnswer()
		if !ok {
			break
		}
		a.Holes[i] = &card
	}
	a.Locked = filledCount(a.Holes) == pick
	if a.Locked {
		e.recordPacket(a)
	}
}

func (e *engine) fillWaitingBots() {
	if e.Phase != phaseSubmit || e.LivePrompt == nil {
		return
	}
	for _, a := range e.Actors {
		if a.Bot && !a.Locked && filledCount(a.Holes) == 0 {
			e.dealBotSubmit(a)
		}
	}
	e.maybeReveal()
}

func (e *engine) canSubmit(id string) bool {
	if e.Phase != phaseSubmit || e.LivePrompt == nil {
		return false
	}
	a := e.Actors[id]
	if a == nil || a.Bot || a.Locked {
		return false
	}
	tied := slices.Contains(e.TieIDs, id)
	if id == e.JudgeID && !(e.SuddenDeath && tied) {
		return false
	}
	if e.SuddenDeath && !tied {
		return false
	}
	return true
}

func (e *engine) slot(id, cardID string) error {
	a := e.Actors[id]
	if a == nil || !e.canSubmit(id) {
		return fmt.Errorf("You cannot play a card now.")
	}
	pick := promptPick(e.LivePrompt)
	if len(a.Holes) != pick {
		a.Holes = make([]*playCard, pick)
	}
	slot := firstEmptyHole(a.Holes)
	swapLast := slot < 0
	if swapLast {
		if len(a.Holes) == 0 {
			return fmt.Errorf("Every hole is filled.")
		}
		slot = len(a.Holes) - 1
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
		if err := wildcardReject(a.Draft, e.Settings, e.wildcardTexts()); err != nil {
			putCardBack(a, handIdx, card)
			return err
		}
		card.Text = strings.TrimSpace(a.Draft)
		card.Wildcard = true
		card.Blank = false
		card.CardID = wildcardID(card.Text)
		a.Draft = ""
	}
	if swapLast {
		returnHole(a, slot)
	}
	a.Holes[slot] = &card
	restoreOrphanDiscard(a)
	return nil
}

func (e *engine) wildcardTexts() []string {
	if e.supply == nil {
		return nil
	}
	return e.supply.wildcardTexts()
}

func (e *engine) unslot(id string, hole int) error {
	a := e.Actors[id]
	if a == nil || !e.canSubmit(id) {
		return fmt.Errorf("You cannot play a card now.")
	}
	if hole < 0 || hole >= len(a.Holes) || a.Holes[hole] == nil {
		return fmt.Errorf("That hole is empty.")
	}
	returnHole(a, hole)
	restoreOrphanDiscard(a)
	return nil
}

func (e *engine) setDraft(id, text string) error {
	a := e.Actors[id]
	if a == nil || !e.canSubmit(id) {
		return fmt.Errorf("You cannot type a wildcard now.")
	}
	if cap := e.Settings.WildcardCap; cap > 0 && len([]rune(text)) > cap {
		text = string([]rune(text)[:cap])
	}
	a.Draft = text
	return e.slot(id, blankCardID)
}

// markDiscard toggles the one printed card a wildcard player throws away.
func (e *engine) markDiscard(id, cardID string) error {
	a := e.Actors[id]
	if a == nil || !e.canSubmit(id) {
		return fmt.Errorf("You cannot discard now.")
	}
	if !holeHasWildcard(a) {
		return fmt.Errorf("Discard is only for leftover printed cards.")
	}
	if a.Discard != nil && a.Discard.CardID == cardID {
		a.Hand = append(a.Hand, *a.Discard)
		a.Discard = nil
		return nil
	}
	if a.Discard != nil {
		return fmt.Errorf("You already discarded a leftover card.")
	}
	for i, c := range a.Hand {
		if c.CardID == cardID && !c.Wildcard {
			a.Hand = slices.Delete(a.Hand, i, i+1)
			a.Discard = &c
			return nil
		}
	}
	return fmt.Errorf("That card is not in your hand.")
}

func (e *engine) lock(id string) error {
	a := e.Actors[id]
	if a == nil || !e.canSubmit(id) {
		return fmt.Errorf("You cannot lock now.")
	}
	pick := promptPick(e.LivePrompt)
	if filledCount(a.Holes) != pick {
		return fmt.Errorf("%d of %d", filledCount(a.Holes), pick)
	}
	a.Locked = true
	if a.Discard != nil && !a.Discard.Blank {
		e.played(a.Discard.LibraryID, a.Discard.CardID)
	}
	for _, c := range holeCards(a.Holes) {
		if c.Blank || c.Wildcard {
			continue
		}
		e.played(c.LibraryID, c.CardID)
	}
	a.Discard = nil
	e.recordPacket(a)
	e.maybeReveal()
	return nil
}

// timerSubmit fills open holes from each hand with printed cards and locks
// every player who ends up full.
func (e *engine) timerSubmit() {
	pick := promptPick(e.LivePrompt)
	for _, a := range e.Actors {
		if !e.canSubmit(a.ID) {
			continue
		}
		for filledCount(a.Holes) < pick {
			idx := slices.IndexFunc(a.Hand, func(c playCard) bool { return !c.Wildcard && !c.Blank })
			slot := firstEmptyHole(a.Holes)
			if idx < 0 || slot < 0 {
				break
			}
			card := a.Hand[idx]
			a.Hand = slices.Delete(a.Hand, idx, idx+1)
			a.Holes[slot] = &card
		}
		if filledCount(a.Holes) == pick {
			_ = e.lock(a.ID)
		}
	}
}

func (e *engine) recordPacket(a *actor) {
	for _, p := range e.Packets {
		if p.ActorID == a.ID {
			return
		}
	}
	e.Packets = append(e.Packets, packet{ActorID: a.ID, Cards: holeCards(a.Holes)})
}

// maybeReveal moves to reveal once every submitter is locked.
func (e *engine) maybeReveal() {
	if e.Phase != phaseSubmit {
		return
	}
	for _, a := range e.Actors {
		if a.ID == e.JudgeID && !(e.SuddenDeath && slices.Contains(e.TieIDs, a.ID)) {
			continue
		}
		if !a.Locked {
			return
		}
	}
	e.shuffle(len(e.Packets), func(i, j int) { e.Packets[i], e.Packets[j] = e.Packets[j], e.Packets[i] })
	e.Phase = phaseReveal
	if e.SuddenDeath && e.JudgeID == "" {
		for i := range e.Packets {
			e.Packets[i].Revealed = true
		}
		e.autoPick()
		return
	}
	e.armTimer(timerBetween, e.Settings.BetweenRevealSec)
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

func returnHole(a *actor, hole int) {
	if hole < 0 || hole >= len(a.Holes) || a.Holes[hole] == nil {
		return
	}
	card := *a.Holes[hole]
	a.Holes[hole] = nil
	if card.Wildcard {
		a.Draft = card.Text
		a.Blank = blankCard()
		return
	}
	a.Hand = append(a.Hand, card)
}

func holeHasWildcard(a *actor) bool {
	for _, c := range a.Holes {
		if c != nil && c.Wildcard {
			return true
		}
	}
	return false
}

func restoreOrphanDiscard(a *actor) {
	if a.Discard == nil || holeHasWildcard(a) {
		return
	}
	a.Hand = append(a.Hand, *a.Discard)
	a.Discard = nil
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
