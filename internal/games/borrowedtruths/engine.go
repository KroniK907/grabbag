package borrowedtruths

import (
	"math/rand"
	"strings"
	"time"
	"unicode/utf8"
)

type phase string

const (
	phaseFacts       phase = "facts"
	phasePrivate     phase = "private-read"
	phasePublic      phase = "public-read"
	phaseQuestion    phase = "questioning"
	phaseVote        phase = "vote"
	phaseReveal      phase = "reveal"
	phaseOwnerVote   phase = "owner-vote"
	phaseOwnerReveal phase = "owner-reveal"
	phaseStandings   phase = "standings"
	phaseFinal       phase = "final"

	phaseLook        phase = "tim-look"
	phaseClaims      phase = "tim-claims"
	phaseTIMQuestion phase = "tim-questioning"
	phaseTIMVote     phase = "tim-vote"
	phaseTIMReveal   phase = "tim-reveal"
)

// isTIM reports a This Is My phase.
func (p phase) isTIM() bool {
	switch p {
	case phaseLook, phaseClaims, phaseTIMQuestion, phaseTIMVote, phaseTIMReveal:
		return true
	}
	return false
}

type cardKind string

const (
	kindYours    cardKind = "yours"
	kindBorrowed cardKind = "borrowed"
	kindLie      cardKind = "lie"
)

// Vote and I knew it picks. Both Lie picks count as Lie.
const (
	pickTrue     = "true"
	pickLie      = "lie"
	pickBorrowed = "borrowed"
)

const maxFactRunes = 120

// fact is one truth or lie in the match pool. Owner is the player who wrote
// it, or empty for a bank lie.
type fact struct {
	Owner string
	Text  string
	Lie   bool
	Used  bool
}

// card is the dealt fact and its type for the current tell.
type card struct {
	Fact int
	Kind cardKind
}

type player struct {
	ID       string
	Name     string
	Seed     string
	Done     bool
	Score    int
	Fooled   int
	Calls    int
	Straight int
}

type knewPick struct {
	Pick  string
	Owner string
}

// award is one line of points from the last reveal.
type award struct {
	ID     string
	Points int
	Why    string
}

// engine is one Borrowed Truths match. It has no HTTP or Helper and takes
// the clock as an argument so tests and previews drive it directly.
type engine struct {
	Settings matchSettings
	Phase    phase
	Players  []*player
	byID     map[string]*player
	Facts    []fact
	Order    []slot
	Tell     int
	Card     *card
	Skips    int
	Skipped  map[int]bool

	Photos     []photo
	Claims     map[string]int
	Round      *timRound
	TIMPlanned bool

	LastOwner  string
	Knew       map[string]knewPick
	Votes      map[string]string
	Crowd      map[string]string
	OwnerVotes map[string]string
	VoteClosed bool
	Awards     []award
	Notice     string

	PhaseStart time.Time
	TimerEnd   time.Time
	TimerLeft  time.Duration
	Extended   bool
	Paused     bool

	rng *rand.Rand
}

// slot is one turn in the match order: a tell by Teller, or a This Is My round.
type slot struct {
	TIM    bool
	Teller string
}

type rosterRow struct {
	ID   string
	Name string
	Seed string
}

func newEngine(s matchSettings, roster []rosterRow, bank []string, rng *rand.Rand, now time.Time) *engine {
	e := &engine{Settings: s, byID: map[string]*player{}, Claims: map[string]int{}, rng: rng}
	e.TIMPlanned = s.timRounds(len(roster)) > 0
	for _, row := range roster {
		p := &player{ID: row.ID, Name: row.Name, Seed: row.Seed}
		e.Players = append(e.Players, p)
		e.byID[row.ID] = p
	}
	if s.lieSource() != lieSourcePlayers {
		for _, text := range bank {
			e.Facts = append(e.Facts, fact{Text: text, Lie: true})
		}
	}
	e.resetCard()
	e.enter(phaseFacts, now)
	return e
}

func (e *engine) player(id string) *player { return e.byID[id] }

func (e *engine) teller() *player {
	if e.Tell < len(e.Order) && !e.Order[e.Tell].TIM {
		return e.byID[e.Order[e.Tell].Teller]
	}
	return nil
}

// insider is the owner of a borrowed truth, the author of a player lie, or
// the hidden owner of a Not theirs photo.
func (e *engine) insider() string {
	if e.Round != nil {
		if e.Round.NotTheirs {
			return e.Photos[e.Round.Photo].Owner
		}
		return ""
	}
	if e.Card == nil || e.Card.Kind == kindYours {
		return ""
	}
	return e.Facts[e.Card.Fact].Owner
}

func (e *engine) resetCard() {
	e.Knew = map[string]knewPick{}
	e.Votes = map[string]string{}
	e.Crowd = map[string]string{}
	e.OwnerVotes = map[string]string{}
	e.VoteClosed = false
	e.Awards = nil
}

// phaseSeconds is the timer length for p when timers are on. 0 is untimed.
func (e *engine) phaseSeconds(p phase) int {
	s := e.Settings
	switch p {
	case phaseFacts:
		if e.TIMPlanned && s.FactsSec == factorySettings().FactsSec {
			return 240
		}
		return s.FactsSec
	case phaseLook:
		return s.LookSec
	case phaseClaims:
		return s.ClaimSec
	case phaseTIMQuestion:
		return s.TIMQuestionSec
	case phaseTIMVote:
		return s.VoteSec
	case phasePrivate:
		return s.PrivateSec
	case phaseQuestion:
		return s.QuestionSec
	case phaseVote:
		return s.VoteSec
	case phaseOwnerVote:
		return s.OwnerVoteSec
	}
	return 0
}

func (e *engine) enter(p phase, now time.Time) {
	e.Phase = p
	e.PhaseStart = now
	e.Extended = false
	e.clearTimer()
	// The look is always timed, even with timers off.
	if sec := e.phaseSeconds(p); sec > 0 && (e.Settings.Timers || p == phaseLook) {
		e.TimerEnd = now.Add(time.Duration(sec) * time.Second)
		if e.Paused {
			e.TimerLeft = time.Duration(sec) * time.Second
		}
	}
}

func (e *engine) clearTimer() {
	e.TimerEnd = time.Time{}
	e.TimerLeft = 0
}

// SetPaused freezes or thaws the phase timer.
func (e *engine) SetPaused(paused bool, now time.Time) {
	if paused == e.Paused {
		return
	}
	e.Paused = paused
	if e.TimerEnd.IsZero() {
		return
	}
	if paused {
		e.TimerLeft = max(0, e.TimerEnd.Sub(now))
		return
	}
	e.TimerEnd = now.Add(e.TimerLeft)
	e.TimerLeft = 0
}

// Advance runs the phase timer. It reports whether the phase moved.
func (e *engine) Advance(now time.Time) (finish, changed bool) {
	if e.Paused || e.TimerEnd.IsZero() || now.Before(e.TimerEnd) {
		return false, false
	}
	e.clearTimer()
	return e.Continue(now)
}

// Extend adds 30 seconds to a running timer, once per phase.
func (e *engine) Extend(now time.Time) string {
	if e.TimerEnd.IsZero() || e.Extended {
		return "No time to add."
	}
	e.Extended = true
	if e.Paused {
		e.TimerLeft += 30 * time.Second
	} else {
		e.TimerEnd = e.TimerEnd.Add(30 * time.Second)
	}
	return ""
}

// Continue is the host's Continue. finish asks the Game to call Finish.
func (e *engine) Continue(now time.Time) (finish, changed bool) {
	switch e.Phase {
	case phaseFacts:
		e.closeFacts(now)
	case phasePrivate:
		e.Notice = ""
		e.enter(phasePublic, now)
	case phasePublic:
		e.enter(phaseQuestion, now)
	case phaseQuestion:
		e.enter(phaseVote, now)
	case phaseVote:
		if !e.VoteClosed {
			e.VoteClosed = true
			e.clearTimer()
			return false, true
		}
		e.reveal(now)
	case phaseReveal:
		if e.Card.Kind == kindBorrowed {
			e.enter(phaseOwnerVote, now)
		} else {
			e.enter(phaseStandings, now)
		}
	case phaseOwnerVote:
		e.ownerReveal(now)
	case phaseOwnerReveal:
		e.enter(phaseStandings, now)
	case phaseStandings:
		e.Tell++
		e.startTell(now)
	case phaseLook:
		e.Round.Speaker = 0
		e.enter(phaseClaims, now)
	case phaseClaims:
		if e.Round.Speaker+1 < len(e.Round.Claimants) {
			e.Round.Speaker++
			e.enter(phaseClaims, now)
		} else {
			e.enter(phaseTIMQuestion, now)
		}
	case phaseTIMQuestion:
		e.enter(phaseTIMVote, now)
	case phaseTIMVote:
		if !e.VoteClosed {
			e.VoteClosed = true
			e.clearTimer()
			return false, true
		}
		e.scoreTIM()
		e.enter(phaseTIMReveal, now)
	case phaseTIMReveal:
		e.enter(phaseStandings, now)
	case phaseFinal:
		return true, false
	default:
		return false, false
	}
	return false, true
}

// SubmitFacts stores one seated player's truths and lies.
func (e *engine) SubmitFacts(id string, truths, lies []string) string {
	p := e.player(id)
	if p == nil {
		return "You are not in this match."
	}
	if e.Phase != phaseFacts {
		return "Facts are closed."
	}
	if p.Done {
		return "Your facts are already in."
	}
	if len(truths) != e.Settings.TruthsPerPlayer || len(lies) != e.Settings.LiesPerPlayer {
		return "Fill in every fact."
	}
	for _, text := range append(append([]string(nil), truths...), lies...) {
		text = strings.TrimSpace(text)
		if text == "" {
			return "Fill in every fact."
		}
		if utf8.RuneCountInString(text) > maxFactRunes {
			return "Keep each fact to 120 characters."
		}
	}
	for _, text := range truths {
		e.Facts = append(e.Facts, fact{Owner: id, Text: strings.TrimSpace(text)})
	}
	if e.Settings.lieSource() != lieSourceBank {
		for _, text := range lies {
			e.Facts = append(e.Facts, fact{Owner: id, Text: strings.TrimSpace(text), Lie: true})
		}
	}
	p.Done = true
	return ""
}

// FactsReady reports that every seated player has written their facts.
func (e *engine) FactsReady() bool {
	for _, p := range e.Players {
		if !p.Done {
			return false
		}
	}
	return true
}

func (e *engine) closeFacts(now time.Time) {
	tellers := tellOrder(e.Settings, e.Players, e.rng)
	e.Order = matchOrder(tellers, e.timCount(len(tellers)), e.Settings.TIMPlacement)
	e.Tell = 0
	e.startTell(now)
}

// startTell deals the slot at e.Tell: a card for a teller, or a photo for
// This Is My. A slot with nothing to deal is skipped. Past the last slot the
// match goes to final.
func (e *engine) startTell(now time.Time) {
	e.Skipped = map[int]bool{}
	e.Skips = e.Settings.Skips
	for ; e.Tell < len(e.Order); e.Tell++ {
		if e.redeal(now) {
			return
		}
	}
	e.Card = nil
	e.Round = nil
	e.enter(phaseFinal, now)
}

// redeal deals a fresh card or photo for the current slot.
func (e *engine) redeal(now time.Time) bool {
	if e.Order[e.Tell].TIM {
		return e.dealTIM(now)
	}
	return e.dealFresh(now)
}

// nextSlot moves on when the current slot has nothing left to deal.
func (e *engine) nextSlot(now time.Time) {
	e.Tell++
	e.startTell(now)
}

// dealFresh deals a new card to the current teller and opens the private read.
func (e *engine) dealFresh(now time.Time) bool {
	e.resetCard()
	e.Round = nil
	c := e.deal(e.Order[e.Tell].Teller)
	if c == nil {
		return false
	}
	e.Card = c
	e.Facts[c.Fact].Used = true
	if c.Kind == kindBorrowed {
		e.LastOwner = e.Facts[c.Fact].Owner
	}
	e.enter(phasePrivate, now)
	return true
}

// Skip puts a truth back in the pool, discards a lie, and deals again.
func (e *engine) Skip(id string, now time.Time) string {
	if e.Phase != phasePrivate || e.teller() == nil || e.teller().ID != id {
		return "You cannot skip now."
	}
	if e.Skips <= 0 {
		return "No skips left."
	}
	e.Skips--
	f := &e.Facts[e.Card.Fact]
	e.Skipped[e.Card.Fact] = true
	if !f.Lie {
		f.Used = false
	}
	if !e.dealFresh(now) {
		e.nextSlot(now)
	}
	return ""
}

// LockIn ends the private read.
func (e *engine) LockIn(id string, now time.Time) string {
	if e.Phase != phasePrivate || e.teller() == nil || e.teller().ID != id {
		return "You cannot lock in now."
	}
	e.Notice = ""
	e.enter(phasePublic, now)
	return ""
}

// Void drops the card with no score and deals the teller a new one.
func (e *engine) Void(now time.Time) string {
	switch e.Phase {
	case phasePrivate, phasePublic, phaseQuestion, phaseVote:
		e.Notice = "Card voided"
	case phaseLook, phaseClaims, phaseTIMQuestion, phaseTIMVote:
		e.Notice = "Photo voided"
	default:
		return "Nothing to void now."
	}
	if !e.redeal(now) {
		e.nextSlot(now)
	}
	return ""
}

// voters are the seated players who may act on this card or photo: not the
// teller, and not a This Is My claimant.
func (e *engine) voters() []*player {
	t := e.teller()
	var out []*player
	for _, p := range e.Players {
		if (t != nil && p.ID == t.ID) || e.isClaimant(p.ID) {
			continue
		}
		out = append(out, p)
	}
	return out
}

// countedVoters are voters whose picks score: not the insider.
func (e *engine) countedVoters() []*player {
	in := e.insider()
	var out []*player
	for _, p := range e.voters() {
		if p.ID != in {
			out = append(out, p)
		}
	}
	return out
}

// CallKnew records an I knew it pick during the public read.
func (e *engine) CallKnew(id, pick, owner string, now time.Time) string {
	if e.Round != nil {
		return e.knewTIM(id, pick, now)
	}
	if !e.Settings.KnewIt || e.Phase != phasePublic {
		return "I knew it is closed."
	}
	if !e.isVoter(id) {
		return "You cannot call this one."
	}
	if _, done := e.Knew[id]; done {
		return "You already called it."
	}
	switch pick {
	case pickTrue, pickLie:
		owner = ""
	case pickBorrowed:
		if p := e.player(owner); p == nil || p.ID == id || p.ID == e.teller().ID {
			return "Pick whose truth it is."
		}
	default:
		return "Pick an answer."
	}
	e.Knew[id] = knewPick{Pick: pick, Owner: owner}
	e.checkKnewVoid(now)
	return ""
}

func (e *engine) checkKnewVoid(now time.Time) {
	counted := e.countedVoters()
	knew := 0
	for _, p := range counted {
		if _, ok := e.Knew[p.ID]; ok {
			knew++
		}
	}
	num, den := e.Settings.voidShare()
	if len(counted) == 0 || knew*den < num*len(counted) {
		return
	}
	e.Awards = nil
	if e.Round != nil {
		e.scoreTIMKnew()
	} else {
		e.scoreKnew()
	}
	e.Notice = "Too well known"
	if !e.redeal(now) {
		e.nextSlot(now)
	}
}

func (e *engine) isVoter(id string) bool {
	for _, p := range e.voters() {
		if p.ID == id {
			return true
		}
	}
	return false
}

// gate is every voter still expected to act: voters who did not tap I knew it.
func (e *engine) gate() []*player {
	var out []*player
	for _, p := range e.voters() {
		if _, ok := e.Knew[p.ID]; !ok {
			out = append(out, p)
		}
	}
	return out
}

// Vote locks a True or Lie pick. Seated voters count toward the gate. The
// audience goes to the crowd bar.
func (e *engine) Vote(id, pick string, seated bool) string {
	if (e.Phase != phaseVote && e.Phase != phaseTIMVote) || e.VoteClosed {
		return "Voting is closed."
	}
	if e.Round != nil {
		if pick != pickNone && !e.isClaimant(pick) {
			return "Pick a claimant or None of them."
		}
	} else if pick != pickTrue && pick != pickLie && pick != pickBorrowed {
		return "Pick True or Lie."
	}
	if !seated || e.player(id) == nil {
		if !e.Settings.AudienceVote {
			return "Audience voting is off."
		}
		if _, done := e.Crowd[id]; done {
			return "Your vote is in."
		}
		e.Crowd[id] = pick
		return ""
	}
	if !e.isVoter(id) {
		return "You cannot vote on this one."
	}
	if _, ok := e.Knew[id]; ok {
		return "You already called it."
	}
	if _, done := e.Votes[id]; done {
		return "Your vote is in."
	}
	e.Votes[id] = pick
	if e.allIn(e.Votes) {
		e.VoteClosed = true
		e.clearTimer()
	}
	return ""
}

func (e *engine) allIn(picks map[string]string) bool {
	for _, p := range e.gate() {
		if _, ok := picks[p.ID]; !ok {
			return false
		}
	}
	return true
}

// Reveal is the teller's Reveal button once the vote is closed.
func (e *engine) Reveal(id string, now time.Time) string {
	if e.Phase != phaseVote || !e.VoteClosed || e.teller() == nil || e.teller().ID != id {
		return "You cannot reveal now."
	}
	e.reveal(now)
	return ""
}

func (e *engine) reveal(now time.Time) {
	e.scoreVote()
	e.enter(phaseReveal, now)
}

// OwnerVote records whose truth a voter thinks it is.
func (e *engine) OwnerVote(id, target string, now time.Time) string {
	if e.Phase != phaseOwnerVote {
		return "The owner vote is closed."
	}
	if !e.isVoter(id) {
		return "You cannot vote on this one."
	}
	if _, ok := e.Knew[id]; ok {
		return "You already called it."
	}
	if _, done := e.OwnerVotes[id]; done {
		return "Your pick is in."
	}
	if t := e.player(target); t == nil || t.ID == id || t.ID == e.teller().ID {
		return "Pick a player."
	}
	e.OwnerVotes[id] = target
	if e.allIn(e.OwnerVotes) {
		e.ownerReveal(now)
	}
	return ""
}

func (e *engine) ownerReveal(now time.Time) {
	e.scoreOwnerVote()
	e.enter(phaseOwnerReveal, now)
}

// TellCount is the number of tells in the match once the order is set.
func (e *engine) TellCount() int {
	n := 0
	for _, s := range e.Order {
		if !s.TIM {
			n++
		}
	}
	return n
}

// TellNum is the 1-based number of the current or last tell.
func (e *engine) TellNum() int {
	n := 0
	for i := 0; i < len(e.Order) && i <= e.Tell; i++ {
		if !e.Order[i].TIM {
			n++
		}
	}
	return n
}
