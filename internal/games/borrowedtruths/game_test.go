package borrowedtruths

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/KroniK907/grabbag/internal/games"
)

type fakeHelper struct {
	mu        sync.Mutex
	seated    []games.Player
	kv        map[string][]byte
	published []string
	finished  bool
	paused    bool
	game      *Game
	dir       string
}

func newFakeHelper(n int) *fakeHelper {
	h := &fakeHelper{kv: map[string][]byte{}}
	for i := 1; i <= n; i++ {
		h.seated = append(h.seated, games.Player{ID: fmt.Sprintf("p%d", i), DisplayName: fmt.Sprintf("Player%d", i), Seated: true, ClaimedHost: i == 1})
	}
	return h
}

func (h *fakeHelper) Seated() []games.Player   { return h.seated }
func (h *fakeHelper) Waiting() []games.Player  { return nil }
func (h *fakeHelper) Audience() []games.Player { return nil }
func (h *fakeHelper) Player(id string) (games.Player, bool) {
	for _, p := range h.seated {
		if p.ID == id {
			return p, true
		}
	}
	return games.Player{}, false
}
func (h *fakeHelper) PlayerFromRequest(r *http.Request) (games.Player, bool, error) {
	id := r.Header.Get("X-Player")
	if p, ok := h.Player(id); ok {
		return p, true, nil
	}
	if id != "" {
		return games.Player{ID: id, DisplayName: id, Audience: true}, true, nil
	}
	return games.Player{}, false, nil
}
func (h *fakeHelper) DataDir() string                        { return h.dir }
func (h *fakeHelper) KVGet(key string) ([]byte, bool, error) { v, ok := h.kv[key]; return v, ok, nil }
func (h *fakeHelper) KVSet(key string, value []byte) error   { h.kv[key] = value; return nil }
func (h *fakeHelper) Finish()                                { h.finished = true; _ = h.game.Stop() }
func (h *fakeHelper) Pause()                                 { h.paused = true; _ = h.game.Pause() }
func (h *fakeHelper) Resume()                                { h.paused = false; _ = h.game.Resume() }
func (h *fakeHelper) Notify(string, string, string, int)     {}
func (h *fakeHelper) Log(string)                             {}
func (h *fakeHelper) Theme() string                          { return "neon-dark" }
func (h *fakeHelper) HasAdmin(*http.Request) bool            { return false }
func (h *fakeHelper) Publish(name string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.published = append(h.published, name)
}

func (h *fakeHelper) lastEvent() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.published) == 0 {
		return ""
	}
	return h.published[len(h.published)-1]
}

func startGame(t *testing.T, n int) (*Game, *fakeHelper, http.Handler) {
	t.Helper()
	g := New()
	h := newFakeHelper(n)
	h.game = g
	h.dir = t.TempDir()
	if err := g.Load(h); err != nil {
		t.Fatal(err)
	}
	if err := g.Start(h); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = g.Shutdown() })
	return g, h, g.Play()
}

func post(t *testing.T, mux http.Handler, player, path string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Player", player)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestCatalogRegistersBorrowedTruths(t *testing.T) {
	for _, f := range games.Catalog() {
		if f.ID == id {
			g := f.New()
			if g.Name() != "Borrowed Truths" || g.MinPlayers() != 4 || g.MaxPlayers() != 20 {
				t.Fatalf("name=%s min=%d max=%d", g.Name(), g.MinPlayers(), g.MaxPlayers())
			}
			return
		}
	}
	t.Fatal("borrowedtruths is not in the catalog")
}

func TestStartNeedsFourSeated(t *testing.T) {
	g := New()
	h := newFakeHelper(3)
	h.game = g
	_ = g.Load(h)
	if err := g.Start(h); err == nil {
		t.Fatal("Start with 3 seated should fail")
	}
}

func TestFactsOverHTTPThenHostContinue(t *testing.T) {
	g, h, mux := startGame(t, 4)
	facts := url.Values{"truth": {"one", "two"}, "lie": {"a lie"}}
	rec := post(t, mux, "p2", "/facts", facts)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Locked in.") {
		t.Fatalf("facts post: %d %s", rec.Code, rec.Body.String())
	}
	if h.lastEvent() != eventFacts {
		t.Fatalf("facts tick published %q, want only the board event", h.lastEvent())
	}
	if rec := post(t, mux, "p2", "/facts", url.Values{"truth": {"x"}, "lie": {"y"}}); !strings.Contains(rec.Body.String(), "already in") {
		t.Fatal("second facts post was accepted")
	}
	if rec := post(t, mux, "p3", "/host/continue", nil); rec.Code != http.StatusForbidden {
		t.Fatalf("non-host continue = %d", rec.Code)
	}
	if rec := post(t, mux, "p1", "/host/continue", nil); rec.Code != http.StatusOK {
		t.Fatalf("host continue = %d", rec.Code)
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.engine.Phase != phasePrivate || h.lastEvent() != event {
		t.Fatalf("phase %s event %q", g.engine.Phase, h.lastEvent())
	}
	if c := g.engine.Card; c == nil || g.engine.Facts[c.Fact].Owner == "p1" {
		t.Fatal("dealt a card from a player with no facts")
	}
}

func TestTellerPhoneIsPrivateAndVoterPhoneIsNot(t *testing.T) {
	g, _, mux := startGame(t, 4)
	for _, p := range []string{"p1", "p2", "p3", "p4"} {
		post(t, mux, p, "/facts", url.Values{"truth": {p + " t1", p + " t2"}, "lie": {p + " lie"}})
	}
	g.mu.Lock()
	teller := g.engine.teller().ID
	text := g.engine.Facts[g.engine.Card.Fact].Text
	var voter string
	for _, p := range g.engine.voters() {
		voter = p.ID
	}
	g.mu.Unlock()

	get := func(player string) string {
		req := httptest.NewRequest(http.MethodGet, "/partials/phone", nil)
		req.Header.Set("X-Player", player)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec.Body.String()
	}
	if body := get(teller); !strings.Contains(body, text) || !strings.Contains(body, "HELLO") {
		t.Fatal("teller phone is missing the card or its type")
	}
	if body := get(voter); strings.Contains(body, text) {
		t.Fatal("voter phone shows the card during the private read")
	}
	post(t, mux, teller, "/lock-in", nil)
	if body := get(voter); !strings.Contains(body, text) {
		t.Fatal("voter phone is missing the card in the public read")
	}
}

func TestHostEndGameFinishes(t *testing.T) {
	g, h, mux := startGame(t, 4)
	g.mu.Lock()
	g.engine.enter(phaseFinal, t0)
	g.mu.Unlock()
	post(t, mux, "p1", "/host/continue", nil)
	if !h.finished {
		t.Fatal("End game did not call Finish")
	}
}

func TestSettingsSaveAndLockDuringMatch(t *testing.T) {
	g := New()
	h := newFakeHelper(4)
	h.game = g
	_ = g.Load(h)
	mux := g.Settings()
	form := url.Values{
		"truths": {"3"}, "lies": {"0"}, "lie_source": {"bank"}, "mix_yours": {"1"}, "mix_borrowed": {"2"}, "mix_lie": {"1"},
		"tells": {"0"}, "max_tells": {"12"}, "facts_sec": {"180"}, "private_sec": {"30"}, "question_sec": {"90"},
		"vote_sec": {"20"}, "owner_vote_sec": {"15"}, "knew_void": {"third"}, "skips": {"2"}, "timers": {"1"},
		"tim_rounds": {"auto"}, "tim_placement": {"alternate"}, "not_theirs": {"20"}, "look_sec": {"20"},
		"claim_sec": {"30"}, "tim_question_sec": {"180"},
	}
	rec := post(t, mux, "", "/match", form)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("save = %d %s", rec.Code, rec.Body.String())
	}
	s := g.loadSettings(h)
	if s.TruthsPerPlayer != 3 || s.LiesPerPlayer != 0 || !s.Timers || s.KnewIt || s.MixBorrowed != 2 || s.KnewVoid != voidThird ||
		s.TIMPlacement != placeAlternate || s.NotTheirsPct != 20 {
		t.Fatalf("saved %+v", s)
	}
	form.Set("facts_sec", "5")
	if rec := post(t, mux, "", "/match", form); !strings.Contains(rec.Body.String(), "Facts timer is 60 to 300.") {
		t.Fatal("out of range value was not refused")
	}
	_ = g.Start(h)
	defer g.Shutdown()
	if rec := post(t, mux, "", "/match", form); !strings.Contains(rec.Body.String(), "Setup is locked") {
		t.Fatal("settings changed during a match")
	}
}
