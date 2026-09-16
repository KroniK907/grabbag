package testinggame

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/KroniK907/grabbag/internal/games"
	"github.com/KroniK907/grabbag/internal/ui"
)

func TestCatalogMinAndMax(t *testing.T) {
	t.Parallel()
	var got games.Factory
	for _, f := range games.Catalog() {
		if f.ID == "testing" {
			got = f
		}
	}
	if got.New == nil {
		t.Fatal("testing is not in the catalog")
	}
	g := got.New()
	if g.ID() != "testing" || g.Name() != "Testing" || g.MinPlayers() != 1 || g.MaxPlayers() != 0 {
		t.Fatalf("id=%s name=%s min=%d max=%d", g.ID(), g.Name(), g.MinPlayers(), g.MaxPlayers())
	}
}

func TestTapSeatedOnly(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		player games.Player
		want   int
	}{
		{
			name:   "seated increments",
			player: games.Player{ID: "p1", DisplayName: "Ada", Seated: true, Connected: true},
			want:   1,
		},
		{
			name:   "disconnected seated increments",
			player: games.Player{ID: "p1", DisplayName: "Ada", Seated: true, Connected: false},
			want:   1,
		},
		{
			name:   "wait is a no-op",
			player: games.Player{ID: "p1", DisplayName: "Ada", Waiting: true, Audience: true},
			want:   0,
		},
		{
			name:   "audience is a no-op",
			player: games.Player{ID: "p1", DisplayName: "Ada", Audience: true},
			want:   0,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := newFakeHelper(tt.player)
			g := startedGame(t, h)
			rec := play(g, http.MethodPost, "/tap", tt.player.ID)
			if rec.Code != http.StatusNoContent {
				t.Fatalf("status = %d", rec.Code)
			}
			if got := tapCount(g, h, "p1"); got != tt.want {
				t.Fatalf("taps = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestFlashOnlyOnFirstBoardPaint(t *testing.T) {
	t.Parallel()
	h := newFakeHelper(games.Player{ID: "p1", DisplayName: "Ada", Seated: true})
	g := startedGame(t, h)
	play(g, http.MethodPost, "/tap", "p1")

	first := httptest.NewRecorder()
	g.Board(first, httptest.NewRequest(http.MethodGet, "/board", nil))
	if !strings.Contains(first.Body.String(), "testing-row-flash") {
		t.Fatal("first paint after tap missing flash")
	}
	second := httptest.NewRecorder()
	g.Board(second, httptest.NewRequest(http.MethodGet, "/board", nil))
	if strings.Contains(second.Body.String(), "testing-row-flash") {
		t.Fatal("second paint still flashed")
	}
}

func TestEndGameAuth(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		player     games.Player
		admin      bool
		wantFinish int
		wantCode   int
	}{
		{
			name:       "claimed host finishes",
			player:     games.Player{ID: "p1", DisplayName: "Ada", Seated: true, ClaimedHost: true},
			wantFinish: 1,
			wantCode:   http.StatusSeeOther,
		},
		{
			name:       "board admin finishes",
			player:     games.Player{ID: "tv", DisplayName: "TV"},
			admin:      true,
			wantFinish: 1,
			wantCode:   http.StatusSeeOther,
		},
		{
			name:       "seated player is a silent no-op",
			player:     games.Player{ID: "p2", DisplayName: "Bea", Seated: true},
			wantFinish: 0,
			wantCode:   http.StatusNoContent,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := newFakeHelper(tt.player)
			h.admin = tt.admin
			g := startedGame(t, h)
			rec := play(g, http.MethodPost, "/end", tt.player.ID)
			if rec.Code != tt.wantCode {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantCode)
			}
			if tt.wantFinish == 1 && rec.Header().Get("Location") != "/" {
				t.Fatalf("end Location = %q", rec.Header().Get("Location"))
			}
			if h.finish != tt.wantFinish {
				t.Fatalf("finish = %d, want %d", h.finish, tt.wantFinish)
			}
		})
	}
}

func TestEndGameFromBoardReturnsToBoard(t *testing.T) {
	t.Parallel()
	h := newFakeHelper(games.Player{ID: "p1", DisplayName: "Ada", Seated: true, ClaimedHost: true})
	h.admin = true
	g := startedGame(t, h)
	req := httptest.NewRequest(http.MethodPost, "/end", strings.NewReader(url.Values{"return": {"/board"}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	g.Play().ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/board" {
		t.Fatalf("board end = %d %q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestHideLatencyKVSurvivesClearAndStop(t *testing.T) {
	t.Parallel()
	h := newFakeHelper(games.Player{ID: "p1", DisplayName: "Ada", Seated: true, LastHeartbeatRTT: 12 * time.Millisecond})
	g := startedGame(t, h)

	post := httptest.NewRequest(http.MethodPost, "/hide-latency", strings.NewReader(url.Values{"hide": {"1"}}.Encode()))
	post.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	g.Settings().ServeHTTP(rec, post)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("hide-latency status = %d", rec.Code)
	}
	if string(h.kv[kvHideLatency]) != "1" {
		t.Fatalf("kv = %q", h.kv[kvHideLatency])
	}

	play(g, http.MethodPost, "/tap", "p1")
	if tapCount(g, h, "p1") != 1 {
		t.Fatal("expected a tap before Stop")
	}
	if err := g.Stop(); err != nil {
		t.Fatal(err)
	}
	if tapCount(g, h, "p1") != 0 {
		t.Fatal("Stop left tap counts")
	}
	if string(h.kv[kvHideLatency]) != "1" {
		t.Fatal("Stop cleared Hide latency")
	}

	board := httptest.NewRecorder()
	g.Board(board, httptest.NewRequest(http.MethodGet, "/board", nil))
	if strings.Contains(board.Body.String(), `data-latency`) {
		t.Fatalf("board still shows latency: %s", board.Body.String())
	}
	if tapCount(g, h, "p1") != 0 {
		t.Fatalf("board after Stop taps = %s", board.Body.String())
	}

	delete(h.kv, kvHideLatency)
	settings := httptest.NewRecorder()
	g.Settings().ServeHTTP(settings, httptest.NewRequest(http.MethodGet, "/", nil))
	body := settings.Body.String()
	if strings.Contains(body, "checked") {
		t.Fatalf("checkbox still checked after clear: %s", body)
	}
}

func TestInfoPageAndBoardButton(t *testing.T) {
	t.Parallel()
	h := newFakeHelper(games.Player{ID: "p1", DisplayName: "Ada", Seated: true})
	g := New()
	if err := g.Load(h); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = g.Shutdown() })

	buttons := g.BoardButtons()
	if len(buttons) != 1 || buttons[0].Label != "About Testing" || buttons[0].Path != "/play/info" || buttons[0].HostOnly {
		t.Fatalf("BoardButtons = %#v", buttons)
	}

	rec := httptest.NewRecorder()
	g.Play().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/info", nil))
	page := rec.Body.String()
	if rec.Code != http.StatusOK {
		t.Fatalf("info status = %d", rec.Code)
	}
	if !strings.Contains(page, "prove the host game contract") ||
		!strings.Contains(page, `href="/board"`) ||
		!strings.Contains(page, "Back to board") {
		t.Fatalf("info page = %s", page)
	}
}

func TestBoardAndPhoneChrome(t *testing.T) {
	t.Parallel()
	host := games.Player{
		ID:               "p1",
		DisplayName:      "Ada",
		AvatarSeed:       "seed-ada",
		Seated:           true,
		ClaimedHost:      true,
		Connected:        true,
		LastHeartbeatRTT: 25 * time.Millisecond,
	}
	h := newFakeHelper(host)
	h.admin = true
	g := startedGame(t, h)

	board := httptest.NewRecorder()
	g.Board(board, httptest.NewRequest(http.MethodGet, "/board", nil))
	html := board.Body.String()
	for _, want := range []string{"Testing", "Ada", "connected", "25 ms", "End game", "sse:testing", "sse:tap", "sse:pause", "/play/partials/board", "/play/static/game.css?v=info-2", `sse:round`, `hx-get="/board"`, `data-theme="neon-light"`} {
		if !strings.Contains(html, want) {
			t.Fatalf("board missing %q in %s", want, html)
		}
	}
	if strings.Contains(html, "is-paused") || strings.Contains(html, "testing-paused") {
		t.Fatal("running board looks paused")
	}

	h.admin = false
	board = httptest.NewRecorder()
	g.Board(board, httptest.NewRequest(http.MethodGet, "/board", nil))
	if strings.Contains(board.Body.String(), "End game") {
		t.Fatal("board End game without admin")
	}

	phone := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Player", "p1")
	g.Phone(phone, req)
	html = phone.Body.String()
	for _, want := range []string{"Ada", "25 ms", "End game", "/play/tap", "/play/partials/phone", "sse:pause", "/play/static/game.css?v=info-2"} {
		if !strings.Contains(html, want) {
			t.Fatalf("phone missing %q in %s", want, html)
		}
	}
	if strings.Contains(html, "is-paused") || strings.Contains(html, "testing-paused") {
		t.Fatal("running phone looks paused")
	}

	guest := games.Player{ID: "p2", DisplayName: "Bea", Seated: true}
	h.byID["p2"] = guest
	phone = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Player", "p2")
	g.Phone(phone, req)
	if strings.Contains(phone.Body.String(), "End game") {
		t.Fatal("non-host phone shows End game")
	}
}

func TestBoardStampsHelperTheme(t *testing.T) {
	t.Parallel()
	h := newFakeHelper(games.Player{ID: "p1", DisplayName: "Ada", Seated: true})
	h.theme = ui.ThemeNeonDark
	g := startedGame(t, h)
	board := httptest.NewRecorder()
	g.Board(board, httptest.NewRequest(http.MethodGet, "/board", nil))
	html := board.Body.String()
	if !strings.Contains(html, `data-theme="neon-dark"`) {
		t.Fatalf("board missing dark theme: %s", html)
	}
	if strings.Contains(html, `data-theme="neon-light"`) {
		t.Fatal("board still stamped light")
	}
}

func TestTickPublishesWhileStarted(t *testing.T) {
	old := tickEvery
	tickEvery = 15 * time.Millisecond
	defer func() { tickEvery = old }()

	h := newFakeHelper(games.Player{ID: "p1", DisplayName: "Ada", Seated: true})
	g := startedGame(t, h)
	deadline := time.Now().Add(400 * time.Millisecond)
	for time.Now().Before(deadline) {
		if h.publishedCount(eventTick) >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if h.publishedCount(eventTick) < 1 {
		t.Fatal("no testing tick while Started")
	}
	if err := g.Stop(); err != nil {
		t.Fatal(err)
	}
	n := h.publishedCount(eventTick)
	time.Sleep(40 * time.Millisecond)
	if h.publishedCount(eventTick) != n {
		t.Fatal("tick continued after Stop")
	}
}

func TestPauseStopsTickAndIgnoresTaps(t *testing.T) {
	old := tickEvery
	tickEvery = 15 * time.Millisecond
	defer func() { tickEvery = old }()

	h := newFakeHelper(games.Player{ID: "p1", DisplayName: "Ada", Seated: true})
	g := startedGame(t, h)
	deadline := time.Now().Add(400 * time.Millisecond)
	for time.Now().Before(deadline) {
		if h.publishedCount(eventTick) >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if h.publishedCount(eventTick) < 1 {
		t.Fatal("no testing tick before Pause")
	}
	if err := g.Pause(); err != nil {
		t.Fatal(err)
	}
	n := h.publishedCount(eventTick)
	time.Sleep(40 * time.Millisecond)
	if h.publishedCount(eventTick) != n {
		t.Fatal("tick continued after Pause")
	}
	play(g, http.MethodPost, "/tap", "p1")
	if tapCount(g, h, "p1") != 0 {
		t.Fatal("tap counted while paused")
	}
	board := httptest.NewRecorder()
	g.Board(board, httptest.NewRequest(http.MethodGet, "/board", nil))
	boardHTML := board.Body.String()
	if !strings.Contains(boardHTML, `class="testing-board is-paused"`) ||
		!strings.Contains(boardHTML, `class="testing-paused"`) {
		t.Fatalf("paused board = %q", boardHTML)
	}
	phone := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Player", "p1")
	g.Phone(phone, req)
	html := phone.Body.String()
	if !strings.Contains(html, `class="testing-phone ui-stack is-paused"`) ||
		!strings.Contains(html, `class="testing-paused"`) {
		t.Fatalf("paused phone = %q", html)
	}
	if err := g.Resume(); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(400 * time.Millisecond)
	for time.Now().Before(deadline) {
		if h.publishedCount(eventTick) > n {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if h.publishedCount(eventTick) <= n {
		t.Fatal("no testing tick after Resume")
	}
	play(g, http.MethodPost, "/tap", "p1")
	if tapCount(g, h, "p1") != 1 {
		t.Fatal("tap ignored after Resume")
	}
}

func startedGame(t *testing.T, h *fakeHelper) *Game {
	t.Helper()
	g := New()
	if err := g.Load(h); err != nil {
		t.Fatal(err)
	}
	if err := g.Start(h); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = g.Stop(); _ = g.Shutdown() })
	return g
}

func play(g *Game, method, path, playerID string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("X-Player", playerID)
	rec := httptest.NewRecorder()
	g.Play().ServeHTTP(rec, req)
	return rec
}

func tapCount(g *Game, h *fakeHelper, id string) int {
	req := httptest.NewRequest(http.MethodGet, "/board", nil)
	rec := httptest.NewRecorder()
	g.Board(rec, req)
	body := rec.Body.String()
	marker := `data-player="` + id + `"`
	i := strings.Index(body, marker)
	if i < 0 {
		return 0
	}
	chunk := body[i:]
	tapAt := strings.Index(chunk, `data-taps>`)
	if tapAt < 0 {
		return -1
	}
	chunk = chunk[tapAt+len(`data-taps>`):]
	end := strings.Index(chunk, "<")
	if end < 0 {
		return -1
	}
	n := 0
	for _, c := range chunk[:end] {
		if c < '0' || c > '9' {
			return -1
		}
		n = n*10 + int(c-'0')
	}
	return n
}

type fakeHelper struct {
	mu        sync.Mutex
	byID      map[string]games.Player
	reqID     string
	kv        map[string][]byte
	admin     bool
	theme     string
	finish    int
	published []string
}

func newFakeHelper(players ...games.Player) *fakeHelper {
	h := &fakeHelper{
		byID: make(map[string]games.Player),
		kv:   make(map[string][]byte),
	}
	for _, p := range players {
		h.byID[p.ID] = p
		if h.reqID == "" {
			h.reqID = p.ID
		}
	}
	return h
}

func (h *fakeHelper) Seated() []games.Player {
	var out []games.Player
	for _, p := range h.byID {
		if p.Seated {
			out = append(out, p)
		}
	}
	return out
}

func (h *fakeHelper) Waiting() []games.Player {
	var out []games.Player
	for _, p := range h.byID {
		if p.Waiting {
			out = append(out, p)
		}
	}
	return out
}

func (h *fakeHelper) Audience() []games.Player {
	var out []games.Player
	for _, p := range h.byID {
		if p.Audience || !p.Seated {
			out = append(out, p)
		}
	}
	return out
}

func (h *fakeHelper) Player(id string) (games.Player, bool) {
	p, ok := h.byID[id]
	return p, ok
}

func (h *fakeHelper) PlayerFromRequest(r *http.Request) (games.Player, bool, error) {
	id := r.Header.Get("X-Player")
	if id == "" {
		id = h.reqID
	}
	p, ok := h.byID[id]
	return p, ok, nil
}

func (h *fakeHelper) DataDir() string { return "" }

func (h *fakeHelper) KVGet(key string) ([]byte, bool, error) {
	v, ok := h.kv[key]
	return v, ok, nil
}

func (h *fakeHelper) KVSet(key string, value []byte) error {
	h.kv[key] = append([]byte(nil), value...)
	return nil
}

func (h *fakeHelper) Finish() { h.finish++ }
func (h *fakeHelper) Pause()  {}
func (h *fakeHelper) Resume() {}

func (h *fakeHelper) Publish(name string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.published = append(h.published, name)
}

func (h *fakeHelper) Log(string) {}

func (h *fakeHelper) Theme() string {
	if h.theme == "" {
		return ui.DefaultTheme
	}
	return h.theme
}

func (h *fakeHelper) HasAdmin(*http.Request) bool { return h.admin }

func (h *fakeHelper) publishedCount(name string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for _, got := range h.published {
		if got == name {
			n++
		}
	}
	return n
}

var _ games.Helper = (*fakeHelper)(nil)
