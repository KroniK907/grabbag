package borrowedtruths

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math/rand"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/KroniK907/grabbag/internal/games"
)

// timEngine is n players with facts and photos in, after the facts close.
// tweak adjusts settings first.
func timEngine(t *testing.T, n, photos int, tweak func(*matchSettings)) *engine {
	t.Helper()
	s := factorySettings()
	s.LieSource = lieSourcePlayers
	if tweak != nil {
		tweak(&s)
	}
	e := newEngine(s, testRoster(n), nil, rand.New(rand.NewSource(5)), t0)
	for i, p := range e.Players {
		e.SubmitFacts(p.ID, []string{p.ID + " a", p.ID + " b"}, []string{p.ID + " lie"})
		if i < photos {
			e.SetPhoto(p.ID, "photo-"+p.ID)
		}
	}
	e.Continue(t0)
	return e
}

// toTIM continues until the first This Is My look.
func toTIM(t *testing.T, e *engine) {
	t.Helper()
	for i := 0; i < 500 && e.Phase != phaseLook; i++ {
		if e.Phase == phaseFinal {
			t.Fatal("reached final with no This Is My round")
		}
		e.Continue(t0)
	}
}

func timSlots(order []slot) string {
	var b strings.Builder
	for _, s := range order {
		if s.TIM {
			b.WriteByte('M')
		} else {
			b.WriteByte('t')
		}
	}
	return b.String()
}

func TestMatchOrderPlacement(t *testing.T) {
	tellers := make([]string, 8)
	for i := range tellers {
		tellers[i] = fmt.Sprint(i)
	}
	for _, tt := range []struct {
		k         int
		placement string
		want      string
	}{
		{2, placeMiddle, "ttttMMtttt"},
		{0, placeMiddle, "tttttttt"},
		{1, placeAlternate, "ttttMtttt"},
		{3, placeAlternate, "ttMttMttMtt"},
		{7, placeAlternate, "tMtMtMtMtMtMtMt"},
		{9, placeAlternate, "tMtMtMtMtMtMtMt"},
	} {
		if got := timSlots(matchOrder(tellers, tt.k, tt.placement)); got != tt.want {
			t.Errorf("k=%d %s: %s, want %s", tt.k, tt.placement, got, tt.want)
		}
	}
}

func TestTIMCountAutoAndCaps(t *testing.T) {
	for _, tt := range []struct {
		n, photos, want int
		rounds          string
	}{
		{4, 4, 0, timAuto},
		{5, 5, 1, timAuto},
		{8, 8, 2, timAuto},
		{13, 13, 3, timAuto},
		{8, 1, 1, timAuto},
		{6, 6, 4, "4"},
		{4, 4, 2, "2"},
	} {
		e := timEngine(t, tt.n, tt.photos, func(s *matchSettings) { s.TIMRounds = tt.rounds })
		got := 0
		for _, s := range e.Order {
			if s.TIM {
				got++
			}
		}
		if got != tt.want {
			t.Errorf("n=%d photos=%d rounds=%s: %d This Is My rounds, want %d", tt.n, tt.photos, tt.rounds, got, tt.want)
		}
	}
}

func TestFactsTimerLongerWithThisIsMy(t *testing.T) {
	s := factorySettings()
	if e := newEngine(s, testRoster(6), nil, rand.New(rand.NewSource(1)), t0); e.phaseSeconds(phaseFacts) != 240 {
		t.Fatalf("facts timer with This Is My = %d, want 240", e.phaseSeconds(phaseFacts))
	}
	if e := newEngine(s, testRoster(4), nil, rand.New(rand.NewSource(1)), t0); e.phaseSeconds(phaseFacts) != 180 {
		t.Fatal("facts timer without This Is My should stay 180")
	}
}

func TestTIMDealOwnerIsAClaimant(t *testing.T) {
	e := timEngine(t, 6, 6, func(s *matchSettings) { s.NotTheirsPct = 0 })
	toTIM(t, e)
	r := e.Round
	if len(r.Claimants) != 3 || r.NotTheirs || !e.isClaimant(e.Photos[r.Photo].Owner) {
		t.Fatalf("round = %+v", r)
	}
	if e.insider() != "" {
		t.Fatal("an owner who claims is not an insider")
	}
	for _, p := range e.voters() {
		if e.isClaimant(p.ID) {
			t.Fatal("claimant is a voter")
		}
	}
}

func TestTIMNotTheirsHidesTheOwner(t *testing.T) {
	e := timEngine(t, 6, 6, func(s *matchSettings) { s.NotTheirsPct = 100 })
	toTIM(t, e)
	owner := e.Photos[e.Round.Photo].Owner
	if !e.Round.NotTheirs || e.isClaimant(owner) || e.insider() != owner {
		t.Fatalf("round = %+v owner %s", e.Round, owner)
	}
	// At 4 seated nobody would be left to vote, so the owner always claims.
	e = timEngine(t, 4, 4, func(s *matchSettings) { s.NotTheirsPct = 100; s.TIMRounds = "1" })
	toTIM(t, e)
	if e.Round.NotTheirs {
		t.Fatal("Not theirs at 4 seated")
	}
}

func TestTIMStopsWithFewerThanThreePhotos(t *testing.T) {
	e := timEngine(t, 8, 4, nil)
	toTIM(t, e)
	e.Void(t0)
	if e.Phase != phaseLook {
		t.Fatalf("3 photos left, phase %s, want a new look", e.Phase)
	}
	e.Void(t0)
	if e.Phase == phaseLook || e.Round != nil {
		t.Fatal("dealt a This Is My round with 2 photos left")
	}
}

func TestTIMLookIsAlwaysTimedAndClaimsRunInOrder(t *testing.T) {
	e := timEngine(t, 6, 6, func(s *matchSettings) { s.NotTheirsPct = 0 })
	toTIM(t, e)
	if e.TimerEnd.IsZero() || e.TimerEnd.Sub(e.PhaseStart) != 20*time.Second {
		t.Fatal("look is not timed with timers off")
	}
	if _, changed := e.Advance(e.PhaseStart.Add(21 * time.Second)); !changed || e.Phase != phaseClaims {
		t.Fatalf("phase after the look = %s", e.Phase)
	}
	if !e.TimerEnd.IsZero() {
		t.Fatal("claims are untimed with timers off")
	}
	for i := 0; i < 3; i++ {
		if e.Phase != phaseClaims || e.Round.Speaker != i {
			t.Fatalf("claim %d: phase %s speaker %d", i, e.Phase, e.Round.Speaker)
		}
		e.Continue(t0)
	}
	if e.Phase != phaseTIMQuestion {
		t.Fatalf("phase after claims = %s", e.Phase)
	}
}

func TestTIMScoringRightClaimant(t *testing.T) {
	e := timEngine(t, 6, 6, func(s *matchSettings) { s.NotTheirsPct = 0 })
	toTIM(t, e)
	e.Continue(t0)
	for e.Phase != phaseTIMVote {
		e.Continue(t0)
	}
	owner := e.Photos[e.Round.Photo].Owner
	var liar string
	for _, c := range e.Round.Claimants {
		if c != owner {
			liar = c
		}
	}
	v := e.voters()
	e.Vote(v[0].ID, owner, true)
	e.Vote(v[1].ID, liar, true)
	if msg := e.Vote(v[2].ID, pickNone, true); msg != "" {
		t.Fatal(msg)
	}
	if msg := e.Vote(owner, owner, true); msg == "" {
		t.Fatal("claimant voted")
	}
	if !e.VoteClosed {
		t.Fatal("vote did not close")
	}
	e.Continue(t0)
	if e.Phase != phaseTIMReveal {
		t.Fatalf("phase = %s", e.Phase)
	}
	if v[0].Score != 2 || v[1].Score != 0 || v[2].Score != 0 {
		t.Fatalf("voters = %d %d %d", v[0].Score, v[1].Score, v[2].Score)
	}
	if e.player(owner).Score != 1 || e.player(liar).Score != 1 {
		t.Fatalf("claimants owner %d liar %d, want 1 each", e.player(owner).Score, e.player(liar).Score)
	}
	e.Continue(t0)
	if e.Phase != phaseStandings {
		t.Fatalf("phase after reveal = %s", e.Phase)
	}
}

func TestTIMScoringNotTheirs(t *testing.T) {
	e := timEngine(t, 7, 7, func(s *matchSettings) { s.NotTheirsPct = 100 })
	toTIM(t, e)
	for e.Phase != phaseTIMVote {
		e.Continue(t0)
	}
	owner := e.player(e.insider())
	var counted []*player
	for _, p := range e.voters() {
		if p != owner {
			counted = append(counted, p)
		}
	}
	claimant := e.Round.Claimants[0]
	e.Vote(counted[0].ID, pickNone, true)
	e.Vote(counted[1].ID, claimant, true)
	e.Vote(counted[2].ID, claimant, true)
	e.Vote(owner.ID, pickNone, true)
	e.Continue(t0)
	if counted[0].Score != 3 {
		t.Fatalf("right on None = %d, want 3", counted[0].Score)
	}
	if e.player(claimant).Score != 2 {
		t.Fatalf("claimant = %d, want 2 for two votes; the owner's vote is ignored", e.player(claimant).Score)
	}
	if owner.Score != 2 || owner.Straight != 1 {
		t.Fatalf("hidden owner = %d, want 2 when fewer than half picked None", owner.Score)
	}
}

func TestTIMKnewItVoidsAndDealsAnotherPhoto(t *testing.T) {
	e := timEngine(t, 7, 7, func(s *matchSettings) { s.NotTheirsPct = 0 })
	toTIM(t, e)
	first := e.Round.Photo
	owner := e.Photos[first].Owner
	v := e.voters()
	if msg := e.CallKnew(v[0].ID, owner, "", t0); msg != "" {
		t.Fatal(msg)
	}
	e.CallKnew(v[1].ID, pickNone, "", t0)
	if e.Phase != phaseLook || e.Round.Photo == first || e.Notice != "Too well known" {
		t.Fatalf("half knew it: phase %s photo %d", e.Phase, e.Round.Photo)
	}
	if v[0].Score != 1 || v[1].Score != 0 {
		t.Fatalf("knew it = %d %d", v[0].Score, v[1].Score)
	}
}

func TestMatchWithThisIsMyRunsToFinal(t *testing.T) {
	e := timEngine(t, 8, 8, func(s *matchSettings) { s.TIMPlacement = placeAlternate; s.TIMRounds = "3" })
	rounds := 0
	for i := 0; i < 500 && e.Phase != phaseFinal; i++ {
		if e.Phase == phaseLook {
			rounds++
		}
		if (e.Phase == phaseVote || e.Phase == phaseTIMVote) && !e.VoteClosed {
			for _, p := range e.voters() {
				if e.Round != nil {
					e.Vote(p.ID, pickNone, true)
				} else {
					e.Vote(p.ID, pickTrue, true)
				}
			}
		}
		e.Continue(t0)
	}
	if e.Phase != phaseFinal || rounds != 3 {
		t.Fatalf("phase %s after %d This Is My rounds", e.Phase, rounds)
	}
	if e.TellNum() != 8 || e.TellCount() != 8 {
		t.Fatalf("tell %d of %d", e.TellNum(), e.TellCount())
	}
}

// The board must not learn the owner or Not theirs before the reveal, and no
// photo id may reach markup before its photo is on the board.
func TestTIMBoardNeverLeaks(t *testing.T) {
	s := factorySettings()
	s.NotTheirsPct = 100
	s.MixYours, s.MixBorrowed, s.MixLie = 1, 0, 0
	e := newEngine(s, testRoster(6), nil, rand.New(rand.NewSource(5)), t0)
	g := &Game{engine: e, started: true, now: func() time.Time { return t0 }}
	for _, p := range e.Players {
		e.SubmitFacts(p.ID, []string{"a", "b"}, []string{"c"})
		e.SetPhoto(p.ID, "secret-"+p.ID)
	}
	render := func(name string, view any) string {
		var buf bytes.Buffer
		if err := pages.ExecuteTemplate(&buf, name, view); err != nil {
			t.Fatal(err)
		}
		return buf.String()
	}
	if html := render("board-frame", g.boardViewLocked()); strings.Contains(html, "secret-") {
		t.Fatal("photo id on the board during facts")
	}
	e.Continue(t0)
	toTIM(t, e)
	owner := e.player(e.insider())
	voter := e.voters()[0]
	if voter == owner {
		voter = e.voters()[1]
	}
	for _, ph := range []phase{phaseLook, phaseClaims, phaseClaims, phaseClaims, phaseTIMQuestion, phaseTIMVote} {
		if e.Phase != ph {
			t.Fatalf("phase %s, want %s", e.Phase, ph)
		}
		view := g.boardViewLocked()
		if view.NotTheirs || view.Owner.ID != "" {
			t.Fatalf("%s board view leaks the owner", ph)
		}
		html := render("board-frame", view)
		for _, leak := range []string{"is-owner", owner.Name, "None of them"} {
			if strings.Contains(html, leak) && !(leak == owner.Name && ph == phaseTIMVote) {
				t.Fatalf("%s board html has %q", ph, leak)
			}
		}
		phone := render("phone-frame", g.phoneViewFor(gp(voter), t0))
		if strings.Contains(phone, "Not yours") || strings.Contains(phone, "Sell it") {
			t.Fatalf("%s voter phone leaks a private line", ph)
		}
		for _, p := range e.Photos {
			if !p.Shown && strings.Contains(html+phone, p.ID) {
				t.Fatalf("%s: unshown photo id in markup", ph)
			}
		}
		e.Continue(t0)
	}
}

func gp(p *player) games.Player { return games.Player{ID: p.ID, DisplayName: p.Name, Seated: true} }

func pngBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	img.Set(1, 1, color.RGBA{255, 0, 0, 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func upload(t *testing.T, mux http.Handler, player string, data []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("photo", "photo.jpg")
	_, _ = fw.Write(data)
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/photo", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("X-Player", player)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestPhotoUploadServeAndDelete(t *testing.T) {
	g, _, mux := startGame(t, 6)
	if rec := upload(t, mux, "p2", []byte("not an image")); rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("non-image = %d", rec.Code)
	}
	if rec := upload(t, mux, "p2", bytes.Repeat([]byte{0}, maxPhotoBytes+10)); rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversize = %d", rec.Code)
	}
	for _, p := range []string{"p2", "p2", "p3"} {
		if rec := upload(t, mux, p, pngBytes(t)); rec.Code != http.StatusNoContent {
			t.Fatalf("upload %s = %d %s", p, rec.Code, rec.Body.String())
		}
	}
	g.mu.Lock()
	dir := g.runDir
	id := g.engine.Photos[0].ID
	g.mu.Unlock()
	files, _ := os.ReadDir(dir)
	if len(files) != 2 {
		t.Fatalf("%d files, want the replaced photo deleted", len(files))
	}
	get := func() int {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/photo/"+id, nil))
		return rec.Code
	}
	if get() != http.StatusNotFound {
		t.Fatal("photo served before it was on the board")
	}
	g.mu.Lock()
	g.engine.Photos[0].Shown = true
	g.mu.Unlock()
	if get() != http.StatusOK {
		t.Fatal("shown photo not served")
	}
	_ = g.Stop()
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("Stop left the photo folder")
	}
}
