package borrowedtruths

import (
	"bytes"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"
)

var t0 = time.Date(2026, 9, 27, 20, 0, 0, 0, time.UTC)

func testRoster(n int) []rosterRow {
	rows := make([]rosterRow, n)
	for i := range rows {
		rows[i] = rosterRow{ID: fmt.Sprintf("p%d", i+1), Name: fmt.Sprintf("Player%d", i+1)}
	}
	return rows
}

// testEngine is n players with facts in, sitting in the private read of the
// first tell. mix is Yours:Borrowed:Lie.
func testEngine(t *testing.T, n int, mix [3]int) *engine {
	t.Helper()
	s := factorySettings()
	s.MixYours, s.MixBorrowed, s.MixLie = mix[0], mix[1], mix[2]
	s.LieSource = lieSourcePlayers
	e := newEngine(s, testRoster(n), nil, rand.New(rand.NewSource(1)), t0)
	for _, p := range e.Players {
		if msg := e.SubmitFacts(p.ID, []string{p.ID + " truth A", p.ID + " truth B"}, []string{p.ID + " lie"}); msg != "" {
			t.Fatal(msg)
		}
	}
	if !e.FactsReady() {
		t.Fatal("facts not ready")
	}
	e.Continue(t0)
	if e.Phase != phasePrivate {
		t.Fatalf("phase = %s, want private read", e.Phase)
	}
	return e
}

// toVote walks the current tell from the private read to an open vote.
func toVote(e *engine) {
	e.LockIn(e.teller().ID, t0)
	e.Continue(t0)
	e.Continue(t0)
}

func TestTellOrderEveryoneOnceBeforeTwice(t *testing.T) {
	s := factorySettings()
	for _, n := range []int{4, 7, 8, 12} {
		e := newEngine(s, testRoster(n), nil, rand.New(rand.NewSource(int64(n))), t0)
		order := tellOrder(s, e.Players, e.rng)
		want := 2
		if n >= 8 {
			want = 1
		}
		if len(order) != n*want {
			t.Fatalf("n=%d: %d tells, want %d", n, len(order), n*want)
		}
		seen := map[string]int{}
		for i, id := range order {
			seen[id]++
			if seen[id] > i/n+1 {
				t.Fatalf("n=%d: %s tells twice before everyone told once: %v", n, id, order)
			}
			if i > 0 && order[i-1] == id {
				t.Fatalf("n=%d: %s tells twice in a row", n, id)
			}
		}
	}
	e := newEngine(s, testRoster(16), nil, rand.New(rand.NewSource(3)), t0)
	if got := len(tellOrder(s, e.Players, e.rng)); got != 12 {
		t.Fatalf("16 seated: %d tells, want the max of 12", got)
	}
}

func TestBorrowedSpreadsOwnersAndNeverRepeats(t *testing.T) {
	e := testEngine(t, 5, [3]int{0, 1, 0})
	teller := e.teller().ID
	owners := map[string]int{}
	last := ""
	for i := 0; i < 4; i++ {
		c := e.deal(teller)
		if c == nil || c.Kind != kindBorrowed {
			t.Fatalf("deal %d = %+v", i, c)
		}
		owner := e.Facts[c.Fact].Owner
		if owner == teller || owner == last {
			t.Fatalf("deal %d: owner %s (teller %s, last %s)", i, owner, teller, last)
		}
		e.Facts[c.Fact].Used = true
		e.LastOwner, last = owner, owner
		owners[owner]++
	}
	for id, n := range owners {
		if n != 1 {
			t.Fatalf("%s borrowed %d times before everyone was borrowed once: %v", id, n, owners)
		}
	}
}

func TestDealFallsBackWhenTypeIsEmpty(t *testing.T) {
	e := testEngine(t, 4, [3]int{1, 0, 1})
	teller := e.teller().ID
	for i, f := range e.Facts {
		if f.Lie {
			e.Facts[i].Used = true
		}
	}
	c := e.deal(teller)
	if c == nil || c.Kind != kindYours || e.Facts[c.Fact].Owner != teller {
		t.Fatalf("deal = %+v, want the teller's own truth", c)
	}
}

func TestNonFinisherIsNeverBorrowedFrom(t *testing.T) {
	s := factorySettings()
	s.MixYours, s.MixBorrowed, s.MixLie = 0, 1, 0
	e := newEngine(s, testRoster(4), nil, rand.New(rand.NewSource(2)), t0)
	for _, p := range e.Players[1:] {
		e.SubmitFacts(p.ID, []string{"a " + p.ID, "b " + p.ID}, []string{"lie " + p.ID})
	}
	e.Continue(t0)
	for i := 0; i < 20 && e.Phase != phaseFinal; i++ {
		if owner := e.Facts[e.Card.Fact].Owner; owner == "p1" {
			t.Fatal("borrowed from a player with no facts")
		}
		e.Void(t0)
	}
}

func TestYoursScoring(t *testing.T) {
	e := testEngine(t, 4, [3]int{1, 0, 0})
	teller := e.teller()
	toVote(e)
	voters := e.voters()
	e.Vote(voters[0].ID, pickTrue, true)
	e.Vote(voters[1].ID, pickLie, true)
	e.Vote(voters[2].ID, pickBorrowed, true)
	if !e.VoteClosed {
		t.Fatal("vote did not close when everyone locked")
	}
	if msg := e.Reveal(teller.ID, t0); msg != "" {
		t.Fatal(msg)
	}
	if voters[0].Score != 2 || voters[1].Score != 0 || voters[2].Score != 0 {
		t.Fatalf("voter scores = %d %d %d", voters[0].Score, voters[1].Score, voters[2].Score)
	}
	if teller.Score != 2 || teller.Fooled != 2 {
		t.Fatalf("teller score %d fooled %d, want 2 and 2", teller.Score, teller.Fooled)
	}
	e.Continue(t0)
	if e.Phase != phaseStandings {
		t.Fatalf("phase after a Yours reveal = %s", e.Phase)
	}
}

func TestBorrowedScoringAndOwnerVote(t *testing.T) {
	e := testEngine(t, 5, [3]int{0, 1, 0})
	teller := e.teller()
	owner := e.player(e.insider())
	toVote(e)
	var others []*player
	for _, p := range e.voters() {
		if p != owner {
			others = append(others, p)
		}
	}
	e.Vote(others[0].ID, pickBorrowed, true)
	e.Vote(others[1].ID, pickLie, true)
	e.Vote(others[2].ID, pickTrue, true)
	e.Vote(owner.ID, pickTrue, true)
	e.Continue(t0)
	if e.Phase != phaseReveal {
		t.Fatalf("phase = %s", e.Phase)
	}
	if others[0].Score != 3 || others[1].Score != 2 || others[2].Score != 0 || owner.Score != 0 {
		t.Fatalf("scores = %d %d %d owner %d", others[0].Score, others[1].Score, others[2].Score, owner.Score)
	}
	if teller.Score != 1 {
		t.Fatalf("teller = %d, want 1 for fooling one counted voter", teller.Score)
	}
	e.Continue(t0)
	if e.Phase != phaseOwnerVote {
		t.Fatalf("phase = %s, want owner vote", e.Phase)
	}
	if msg := e.OwnerVote(others[0].ID, teller.ID, t0); msg == "" {
		t.Fatal("owner vote for the teller was allowed")
	}
	e.OwnerVote(others[0].ID, owner.ID, t0)
	e.OwnerVote(others[1].ID, others[2].ID, t0)
	e.OwnerVote(others[2].ID, others[1].ID, t0)
	e.OwnerVote(owner.ID, others[0].ID, t0)
	if e.Phase != phaseOwnerReveal {
		t.Fatalf("phase = %s, want owner reveal once everyone picked", e.Phase)
	}
	if others[0].Score != 5 {
		t.Fatalf("owner namer = %d, want 3 + 2", others[0].Score)
	}
	if owner.Score != 2 || owner.Straight != 1 {
		t.Fatalf("owner = %d straight %d, want the 2 point bonus", owner.Score, owner.Straight)
	}
}

func TestPlayerLieAuthorBonus(t *testing.T) {
	e := testEngine(t, 4, [3]int{0, 0, 1})
	author := e.player(e.insider())
	if author == nil || author == e.teller() {
		t.Fatal("player lie has no author")
	}
	toVote(e)
	for _, p := range e.voters() {
		e.Vote(p.ID, pickTrue, true)
	}
	e.Continue(t0)
	if author.Score != 2 {
		t.Fatalf("author = %d, want 2 when nobody called Lie", author.Score)
	}
	if e.teller().Score != 2 {
		t.Fatalf("teller = %d, want 2: the author's vote is ignored", e.teller().Score)
	}
}

func TestKnewItScoresAndVoids(t *testing.T) {
	e := testEngine(t, 5, [3]int{1, 0, 0})
	e.LockIn(e.teller().ID, t0)
	voters := e.voters()
	first := e.Card.Fact
	if msg := e.CallKnew(voters[0].ID, pickTrue, "", t0); msg != "" {
		t.Fatal(msg)
	}
	if e.Phase != phasePublic {
		t.Fatalf("one of four knew it voided the card")
	}
	e.CallKnew(voters[1].ID, pickLie, "", t0)
	if e.Phase != phasePrivate || e.Notice != "Too well known" || e.Card.Fact == first {
		t.Fatalf("half knew it: phase %s notice %q", e.Phase, e.Notice)
	}
	if voters[0].Score != 1 || voters[1].Score != 0 {
		t.Fatalf("knew it scores = %d %d", voters[0].Score, voters[1].Score)
	}
}

func TestKnewItVoterSitsOutTheVote(t *testing.T) {
	e := testEngine(t, 6, [3]int{1, 0, 0})
	e.LockIn(e.teller().ID, t0)
	knower := e.voters()[0]
	e.CallKnew(knower.ID, pickTrue, "", t0)
	e.Continue(t0)
	e.Continue(t0)
	if msg := e.Vote(knower.ID, pickTrue, true); msg == "" {
		t.Fatal("I knew it voter could vote")
	}
	for _, p := range e.voters()[1:] {
		e.Vote(p.ID, pickTrue, true)
	}
	if !e.VoteClosed {
		t.Fatal("the vote waited on the I knew it voter")
	}
}

func TestSkipReturnsTruthAndDiscardsLie(t *testing.T) {
	e := testEngine(t, 4, [3]int{1, 0, 0})
	skipped := e.Card.Fact
	if msg := e.Skip(e.teller().ID, t0); msg != "" {
		t.Fatal(msg)
	}
	if e.Facts[skipped].Used || e.Card.Fact == skipped {
		t.Fatal("skipped truth should go back in the pool and not be dealt again this tell")
	}
	if msg := e.Skip(e.teller().ID, t0); msg == "" {
		t.Fatal("second skip allowed with 1 skip per turn")
	}
}

func TestHostVoidScoresNothing(t *testing.T) {
	e := testEngine(t, 4, [3]int{1, 0, 0})
	toVote(e)
	for _, p := range e.voters() {
		e.Vote(p.ID, pickTrue, true)
	}
	e.Void(t0)
	for _, p := range e.Players {
		if p.Score != 0 {
			t.Fatalf("%s scored %d on a void", p.ID, p.Score)
		}
	}
	if e.Phase != phasePrivate {
		t.Fatalf("phase after void = %s", e.Phase)
	}
}

func TestTimersAdvanceAndExtend(t *testing.T) {
	e := testEngine(t, 4, [3]int{1, 0, 0})
	if !e.TimerEnd.IsZero() {
		t.Fatal("timers are off by default")
	}
	e.Settings.Timers = true
	e.enter(phasePrivate, t0)
	if e.Extend(t0) != "" || e.Extend(t0) == "" {
		t.Fatal("+30s should work once per phase")
	}
	if _, changed := e.Advance(t0.Add(40 * time.Second)); changed {
		t.Fatal("advanced before 30 + 30 seconds")
	}
	e.SetPaused(true, t0.Add(40*time.Second))
	if _, changed := e.Advance(t0.Add(2 * time.Minute)); changed {
		t.Fatal("advanced while paused")
	}
	e.SetPaused(false, t0.Add(2*time.Minute))
	if _, changed := e.Advance(t0.Add(2*time.Minute + 21*time.Second)); !changed || e.Phase != phasePublic {
		t.Fatalf("phase = %s, want public read when the timer ran out", e.Phase)
	}
	if !e.TimerEnd.IsZero() {
		t.Fatal("public read is untimed")
	}
}

func TestMatchRunsToFinal(t *testing.T) {
	e := testEngine(t, 4, [3]int{1, 1, 1})
	for i := 0; i < 200 && e.Phase != phaseFinal; i++ {
		if e.Phase == phaseVote && !e.VoteClosed {
			for _, p := range e.voters() {
				e.Vote(p.ID, pickLie, true)
			}
		}
		e.Continue(t0)
	}
	if e.Phase != phaseFinal {
		t.Fatalf("match stuck in %s", e.Phase)
	}
	if finish, _ := e.Continue(t0); !finish {
		t.Fatal("Continue on final should finish")
	}
	if len(e.ribbons()) == 0 {
		t.Fatal("no ribbons after a full match")
	}
}

// The board must not learn the card type, owner, or author before the reveal.
func TestBoardNeverLeaksBeforeReveal(t *testing.T) {
	e := testEngine(t, 5, [3]int{0, 1, 0})
	g := &Game{engine: e, started: true, now: func() time.Time { return t0 }}
	owner := e.player(e.insider())
	for _, ph := range []phase{phasePrivate, phasePublic, phaseQuestion, phaseVote} {
		if e.Phase != ph {
			t.Fatalf("phase = %s, want %s", e.Phase, ph)
		}
		view := g.boardViewLocked()
		if view.Answer != "" || view.Borrowed || view.Author != "" || view.Owner.ID != "" || view.OwnerSplit != nil {
			t.Fatalf("%s board view leaks: %+v", ph, view)
		}
		if ph == phasePrivate && view.CardText != "" {
			t.Fatal("card text on the board during the private read")
		}
		var buf bytes.Buffer
		if err := pages.ExecuteTemplate(&buf, "board-frame", view); err != nil {
			t.Fatal(err)
		}
		html := buf.String()
		for _, leak := range []string{"belongs to", "is-owner", "Written by", "true for someone"} {
			if strings.Contains(html, leak) {
				t.Fatalf("%s board html has %q", ph, leak)
			}
		}
		if ph == phaseVote {
			e.Vote(owner.ID, pickTrue, true)
			break
		}
		if ph == phasePrivate {
			e.LockIn(e.teller().ID, t0)
		} else {
			e.Continue(t0)
		}
	}
	for _, p := range e.voters() {
		e.Vote(p.ID, pickTrue, true)
	}
	e.Continue(t0)
	e.Continue(t0)
	if view := g.boardViewLocked(); view.Owner.ID != "" {
		t.Fatal("owner named during the owner vote")
	}
}

func TestPreviewStagesReachTheirPhase(t *testing.T) {
	want := map[string]phase{
		"facts": phaseFacts, "private-read": phasePrivate, "public-read": phasePublic,
		"vote": phaseVote, "vote-closed": phaseVote, "reveal": phaseReveal, "reveal-lie": phaseReveal,
		"owner-vote": phaseOwnerVote, "owner-reveal": phaseOwnerReveal, "standings": phaseStandings, "final": phaseFinal,
	}
	for _, st := range previewStages {
		for _, n := range []int{4, 12, 20} {
			g, _, err := previewMatch(st.name, st.kind, n)
			if err != nil {
				t.Fatal(err)
			}
			if g.engine.Phase != want[st.name] {
				t.Fatalf("%s n=%d: phase %s", st.name, n, g.engine.Phase)
			}
			if st.name == "vote-closed" && !g.engine.VoteClosed {
				t.Fatalf("vote-closed n=%d: vote still open", n)
			}
		}
	}
}

func TestPreviewTIMStagesReachTheirPhase(t *testing.T) {
	want := map[string]phase{
		"tim-look": phaseLook, "tim-claims": phaseClaims, "tim-questioning": phaseTIMQuestion,
		"tim-vote": phaseTIMVote, "tim-reveal": phaseTIMReveal, "tim-reveal-none": phaseTIMReveal,
	}
	for _, st := range previewTIMStages {
		for _, n := range []int{5, 12, 20} {
			g, cast, err := previewTIM(st, n)
			if err != nil {
				t.Fatal(err)
			}
			e := g.engine
			if e.Phase != want[st] {
				t.Fatalf("%s n=%d: phase %s", st, n, e.Phase)
			}
			if (st == "tim-reveal-none") != e.Round.NotTheirs {
				t.Fatalf("%s n=%d: NotTheirs %v", st, n, e.Round.NotTheirs)
			}
			if st == "tim-claims" && e.Round.Speaker != 1 {
				t.Fatalf("tim-claims speaker %d", e.Round.Speaker)
			}
			if cast["voter"].ID == "" || e.isClaimant(cast["voter"].ID) {
				t.Fatalf("%s n=%d: bad preview voter", st, n)
			}
		}
	}
}
