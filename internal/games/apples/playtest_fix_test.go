package apples

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestBoardRosterKeepsSeatOrder(t *testing.T) {
	t.Parallel()
	h, g := tinyGame(t, 6, 40, 1)
	postSettings(g, "/hand-size", url.Values{"hand_size": {"3"}})
	postSettings(g, "/voting", url.Values{"voting": {"off"}})
	postSettings(g, "/timer-submit", url.Values{"submit": {"0"}})
	h.sit("p1", "Pat")
	h.sit("p2", "Sam")
	seedRNG(g)
	if err := g.Start(h); err != nil {
		t.Fatal(err)
	}
	first := boardRosterNames(t, g)
	if len(first) < 3 || first[0] != "Pat" || first[1] != "Sam" {
		t.Fatalf("seat order = %v", first)
	}
	for i := 0; i < 8; i++ {
		got := boardRosterNames(t, g)
		if strings.Join(got, ",") != strings.Join(first, ",") {
			t.Fatalf("roster jumped from %v to %v", first, got)
		}
	}
}

func TestDrawWaitHidesOldPrompt(t *testing.T) {
	t.Parallel()
	h, g := tinyGame(t, 8, 40, 1)
	postSettings(g, "/hand-size", url.Values{"hand_size": {"3"}})
	postSettings(g, "/prompt-mode", url.Values{"prompt_mode": {"single"}})
	postSettings(g, "/voting", url.Values{"voting": {"off"}})
	postSettings(g, "/timer-submit", url.Values{"submit": {"0"}})
	postSettings(g, "/timer-judge-pick", url.Values{"judge_pick": {"0"}})
	h.sit("p1", "Pat")
	h.sit("p2", "Sam")
	seedRNG(g)
	if err := g.Start(h); err != nil {
		t.Fatal(err)
	}
	judge := currentJudge(g)
	other := "p1"
	if other == judge {
		other = "p2"
	}
	playPOST(g, "/draw", judge, nil)
	prompt := g.match.LivePrompt.Text
	playPOST(g, "/slot", other, url.Values{"card": {firstHandCard(g, other)}})
	playPOST(g, "/lock", other, nil)
	for !allRevealed(g) {
		playPOST(g, "/reveal", judge, nil)
	}
	playPOST(g, "/confirm", judge, url.Values{"winner": {firstPacket(g)}})
	if phaseOf(g) != phaseDrawWait {
		t.Fatalf("phase = %s", phaseOf(g))
	}
	board := httptest.NewRecorder()
	g.Board(board, httptest.NewRequest(http.MethodGet, "/board", nil))
	body := board.Body.String()
	if !strings.Contains(body, "Judge, draw") {
		t.Fatalf("draw board missing draw copy: %s", body)
	}
	if strings.Contains(body, expandBlanks(prompt)) {
		t.Fatalf("draw board still shows last prompt: %s", body)
	}
}

func TestPhoneShowsHandWhileJudgeChooses(t *testing.T) {
	t.Parallel()
	h, g := tinyGame(t, 8, 40, 1)
	postSettings(g, "/hand-size", url.Values{"hand_size": {"3"}})
	postSettings(g, "/prompt-mode", url.Values{"prompt_mode": {modeMulti}})
	postSettings(g, "/voting", url.Values{"voting": {"off"}})
	postSettings(g, "/timer-submit", url.Values{"submit": {"0"}})
	h.sit("p1", "Pat")
	h.sit("p2", "Sam")
	seedRNG(g)
	if err := g.Start(h); err != nil {
		t.Fatal(err)
	}
	judge := currentJudge(g)
	other := "p1"
	if other == judge {
		other = "p2"
	}
	playPOST(g, "/draw", judge, nil)
	if phaseOf(g) != phaseChoose {
		t.Fatalf("phase = %s", phaseOf(g))
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Player", other)
	rec := httptest.NewRecorder()
	g.Phone(rec, req)
	body := rec.Body.String()
	if !strings.Contains(body, `class="apples-prompt down"`) {
		t.Fatalf("missing blank prompt: %s", body)
	}
	card := firstHandCard(g, other)
	if !strings.Contains(body, g.match.Actors[other].Hand[0].Text) {
		t.Fatalf("missing hand card: %s", body)
	}
	if strings.Contains(body, `hx-post="/play/slot"`) {
		t.Fatalf("preview still slots cards: %s", body)
	}
	if card == "" {
		t.Fatal("empty hand")
	}
}

func TestPromptBlanksRenderAsFourUnderscores(t *testing.T) {
	t.Parallel()
	if got := expandBlanks("Hi _ there"); got != "Hi ____ there" {
		t.Fatalf("expand = %q", got)
	}
	if got := expandBlanks("keep ____"); got != "keep ____" {
		t.Fatalf("already expanded = %q", got)
	}
	h, g := tinyGame(t, 6, 40, 1)
	postSettings(g, "/hand-size", url.Values{"hand_size": {"3"}})
	postSettings(g, "/prompt-mode", url.Values{"prompt_mode": {"single"}})
	postSettings(g, "/voting", url.Values{"voting": {"off"}})
	postSettings(g, "/timer-submit", url.Values{"submit": {"0"}})
	h.sit("p1", "Pat")
	h.sit("p2", "Sam")
	seedRNG(g)
	if err := g.Start(h); err != nil {
		t.Fatal(err)
	}
	judge := currentJudge(g)
	playPOST(g, "/draw", judge, nil)
	board := httptest.NewRecorder()
	g.Board(board, httptest.NewRequest(http.MethodGet, "/board", nil))
	body := board.Body.String()
	if !strings.Contains(body, "____") {
		t.Fatalf("board missing four-underscore blank: %s", body)
	}
	if strings.Contains(body, expandBlanks(g.match.LivePrompt.Text)) {
		lib := g.match.LivePrompt.Text
		if strings.Contains(lib, "_") && strings.Contains(body, ">"+lib+"<") {
			t.Fatalf("board still shows library blank %q: %s", lib, body)
		}
	}
}

func TestFavoriteVoteBlocksJudgeConfirm(t *testing.T) {
	t.Parallel()
	h, g := tinyGame(t, 6, 40, 1)
	postSettings(g, "/hand-size", url.Values{"hand_size": {"3"}})
	postSettings(g, "/prompt-mode", url.Values{"prompt_mode": {"single"}})
	postSettings(g, "/timer-submit", url.Values{"submit": {"0"}})
	postSettings(g, "/timer-favorite-vote", url.Values{"favorite_vote": {"20"}})
	postSettings(g, "/timer-judge-pick", url.Values{"judge_pick": {"0"}})
	h.sit("p1", "Pat")
	h.sit("p2", "Sam")
	seedRNG(g)
	now := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	g.now = func() time.Time { return now }
	if err := g.Start(h); err != nil {
		t.Fatal(err)
	}
	judge := currentJudge(g)
	other := "p1"
	if other == judge {
		other = "p2"
	}
	playPOST(g, "/draw", judge, nil)
	playPOST(g, "/slot", other, url.Values{"card": {firstHandCard(g, other)}})
	playPOST(g, "/lock", other, nil)
	for !allRevealed(g) {
		playPOST(g, "/reveal", judge, nil)
	}
	if g.match.TimerKind != "favorite-vote" {
		t.Fatalf("timer = %s", g.match.TimerKind)
	}
	conf := playPOST(g, "/confirm", judge, url.Values{"winner": {firstPacket(g)}})
	if !strings.Contains(conf.Body.String(), "Favorites are still open.") {
		t.Fatalf("confirm during favorites = %s", conf.Body.String())
	}
	now = now.Add(21 * time.Second)
	g.mu.Lock()
	g.fireTimerLocked()
	g.mu.Unlock()
	if g.match.TimerKind != "" && g.match.TimerKind != "judge-pick" {
		t.Fatalf("after favorites timer = %s", g.match.TimerKind)
	}
	conf = playPOST(g, "/confirm", judge, url.Values{"winner": {firstPacket(g)}})
	if strings.Contains(conf.Body.String(), "Favorites are still open.") {
		t.Fatalf("confirm after favorites = %s", conf.Body.String())
	}
	if g.match.WinnerID == "" {
		t.Fatal("winner was not set")
	}
}

func TestWinnerHoldZeroWaitsForHost(t *testing.T) {
	t.Parallel()
	h := newFakeHelper(t.TempDir(), true)
	h.sitHost("p1", "Pat")
	g := loadedGame(t, h)
	m := &matchState{
		Settings: factorySettings(),
		Actors:   map[string]*actor{"p1": {ID: "p1", Name: "Pat"}},
		WinnerID: "p1",
		PhoneErr: map[string]string{},
	}
	g.mu.Lock()
	g.started = true
	g.match = m
	g.finishLocked(h, m)
	g.mu.Unlock()
	if m.TimerKind != "" {
		t.Fatalf("zero hold armed %s", m.TimerKind)
	}
	board := httptest.NewRecorder()
	g.Board(board, httptest.NewRequest(http.MethodGet, "/board", nil))
	body := board.Body.String()
	if !strings.Contains(body, ">Winner<") || !strings.Contains(body, `action="/play/finish"`) {
		t.Fatalf("winner page = %s", body)
	}
	g.mu.Lock()
	g.fireTimerLocked()
	g.mu.Unlock()
	if h.finishN != 0 {
		t.Fatal("zero hold auto-finished")
	}
	req := httptest.NewRequest(http.MethodPost, "/finish", strings.NewReader("return=/board"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Player", "p1")
	rec := httptest.NewRecorder()
	g.Play().ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("finish = %d %s", rec.Code, rec.Body.String())
	}
	if h.finishN != 1 {
		t.Fatalf("Finish calls = %d", h.finishN)
	}
}

func TestSettingsShowNewTimers(t *testing.T) {
	t.Parallel()
	g := loadedGame(t, newFakeHelper(t.TempDir(), true))
	page := settingsPage(t, g)
	for _, want := range []string{"Favorite vote", "Winner screen", `name="favorite_vote"`, `value="20"`} {
		if !strings.Contains(page, want) {
			t.Fatalf("settings missing %q: %s", want, page)
		}
	}
}

func TestPhoneUsesCompactVisualViewportLayout(t *testing.T) {
	t.Parallel()
	g := loadedGame(t, newFakeHelper(t.TempDir(), true))
	for _, tt := range []struct {
		path string
		want []string
	}{
		{path: "/static/game.js", want: []string{"visualViewport", "--apples-visual-height", "apples-compact-height", "height < 760"}},
		{path: "/static/game.css", want: []string{".apples-lock-bar .ui-btn", "min-height: 44px", "var(--apples-visual-height, 100dvh)"}},
	} {
		rec := httptest.NewRecorder()
		g.Play().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s = %d", tt.path, rec.Code)
		}
		for _, want := range tt.want {
			if !strings.Contains(rec.Body.String(), want) {
				t.Fatalf("%s missing %q", tt.path, want)
			}
		}
	}
}

func boardRosterNames(t *testing.T, g *Game) []string {
	t.Helper()
	rec := httptest.NewRecorder()
	g.Board(rec, httptest.NewRequest(http.MethodGet, "/board", nil))
	body := rec.Body.String()
	var names []string
	for {
		i := strings.Index(body, "<strong>")
		if i < 0 {
			break
		}
		body = body[i+len("<strong>"):]
		j := strings.Index(body, "</strong>")
		if j < 0 {
			break
		}
		names = append(names, body[:j])
		body = body[j:]
	}
	return names
}
