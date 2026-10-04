package host

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/KroniK907/grabbag/internal/games"
	"github.com/KroniK907/grabbag/internal/store"
)

func getFrame(t *testing.T, handler http.Handler, path string, cookies []*http.Cookie) tenantFrame {
	t.Helper()
	rec := requestWithCookie(t, handler, http.MethodGet, path, nil, cookies)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s = %d %q", path, rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("GET %s content type = %q", path, got)
	}
	var frame tenantFrame
	if err := json.Unmarshal(rec.Body.Bytes(), &frame); err != nil {
		t.Fatalf("GET %s body: %v", path, err)
	}
	return frame
}

func hxPost(t *testing.T, handler http.Handler, path string, form url.Values, cookies []*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "http://grabbag.test"+path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestShellDocumentsRenderTheCurrentTenant(t *testing.T) {
	t.Parallel()
	_, handler, rt, _ := testGameHandler(t, 0, 0)
	admin := finishAndJoinHost(t, handler)

	board := requestWithCookie(t, handler, http.MethodGet, "/board", nil, admin)
	page := board.Body.String()
	for _, want := range []string{
		`data-theme="neon-dark"`, `id="shell-stage"`, `data-surface="board"`, `data-tenant="lobby"`,
		`data-boot="` + rt.boot + `"`, `data-stream="/lobby/events"`, `src="/static/shell.js?v=`,
		`src="/static/htmx.min.js?v=`, `src="/lobby/static/lobby.js?v=`, `id="connection-overlay"`,
		`id="notice-root"`, `data-notice-targets="board seated"`, `id="board-roster"`,
	} {
		if !strings.Contains(page, want) {
			t.Fatalf("board shell missing %q: %s", want, page)
		}
	}
	for _, bad := range []string{"sse.min.js", "sse-connect", "location.reload"} {
		if strings.Contains(page, bad) {
			t.Fatalf("board shell has %q", bad)
		}
	}
	if got := board.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("board Cache-Control = %q", got)
	}

	phone := requestWithCookie(t, handler, http.MethodGet, "/", nil, admin).Body.String()
	for _, want := range []string{
		`data-surface="phone"`, `data-tenant="lobby"`, "data-player", `id="shell-frame"`, `class="shell-bar"`,
		`id="phone-inner"`, `data-notice-targets="host seated"`, `id="shell-help"`,
	} {
		if !strings.Contains(phone, want) {
			t.Fatalf("phone shell missing %q: %s", want, phone)
		}
	}
	frame := shellFrameParts(t, phone)
	if frame.leaveHidden || !frame.helpHidden || !strings.Contains(frame.host, `id="host-drawer"`) {
		t.Fatalf("host's Lobby frame = %+v", frame)
	}
	if strings.Contains(frame.stage, `id="host-drawer"`) || strings.Contains(frame.stage, `hx-post="/lobby/leave"`) {
		t.Fatalf("Lobby tenant renders the host drawer or Leave: %s", frame.stage)
	}
	stranger := request(t, handler, http.MethodGet, "/", nil, "").Body.String()
	if strings.Contains(stranger, "data-player") || !strings.Contains(stranger, `hx-post="/lobby/join"`) {
		t.Fatalf("stranger phone shell = %s", stranger)
	}
	if frame := shellFrameParts(t, stranger); !frame.leaveHidden || !frame.helpHidden || frame.host != "" {
		t.Fatalf("stranger frame = %+v", frame)
	}
}

func TestTenantFrameFollowsStartAndStop(t *testing.T) {
	t.Parallel()
	_, handler, rt, _ := testGameHandler(t, 0, 0)
	admin := finishAndJoinHost(t, handler)

	lobbyBoard := getFrame(t, handler, "/board/tenant", admin)
	if lobbyBoard.Tenant != "lobby" || lobbyBoard.Boot != rt.boot || lobbyBoard.Stream != streamRoom ||
		!strings.Contains(lobbyBoard.HTML, `id="board-roster"`) || lobbyBoard.Theme != "neon-dark" {
		t.Fatalf("lobby board frame = %+v", lobbyBoard)
	}
	if len(lobbyBoard.Assets.JS) != 1 || !strings.HasPrefix(lobbyBoard.Assets.JS[0], "/lobby/static/lobby.js") {
		t.Fatalf("lobby assets = %+v", lobbyBoard.Assets)
	}
	lobbyPhone := getFrame(t, handler, "/tenant", admin)
	if lobbyPhone.Tenant != "lobby" || !lobbyPhone.Player || lobbyPhone.Kicked || lobbyPhone.Help || !strings.Contains(lobbyPhone.Host, `id="host-drawer"`) {
		t.Fatalf("lobby phone frame = %+v", lobbyPhone)
	}
	if help := requestWithCookie(t, handler, http.MethodGet, "/help", nil, admin); help.Code != http.StatusNotFound {
		t.Fatalf("GET /help on the Lobby = %d", help.Code)
	}

	load := hxPost(t, handler, "/settings/load", url.Values{"game_id": {"fake"}}, admin)
	if load.Code != http.StatusNoContent || load.Header().Get("HX-Trigger") != "grabbag:tenant" {
		t.Fatalf("htmx Load = %d trigger %q", load.Code, load.Header().Get("HX-Trigger"))
	}
	loaded := getFrame(t, handler, "/tenant", admin)
	if loaded.Generation <= lobbyPhone.Generation || loaded.Tenant != "lobby" {
		t.Fatalf("Load did not move the generation on: %d then %+v", lobbyPhone.Generation, loaded)
	}

	start := hxPost(t, handler, "/settings/start", nil, admin)
	if start.Code != http.StatusNoContent || start.Header().Get("HX-Trigger") != "grabbag:tenant" {
		t.Fatalf("htmx Start = %d trigger %q", start.Code, start.Header().Get("HX-Trigger"))
	}
	if loc := start.Header().Get("Location"); loc != "" {
		t.Fatalf("htmx Start redirected to %q", loc)
	}
	game := getFrame(t, handler, "/board/tenant", admin)
	if game.Tenant != "fake" || game.Generation <= loaded.Generation || !strings.Contains(game.HTML, "FAKE-BOARD") {
		t.Fatalf("started board frame = %+v", game)
	}
	if len(game.Assets.CSS) != 1 || game.Assets.CSS[0] != "/games/fake/static/fake.css" {
		t.Fatalf("game assets = %+v", game.Assets)
	}
	gamePhone := getFrame(t, handler, "/tenant", admin)
	if gamePhone.Tenant != "fake" || !strings.Contains(gamePhone.HTML, "FAKE-PHONE") || !gamePhone.Help ||
		!strings.Contains(gamePhone.Host, `id="host-drawer"`) || strings.Contains(gamePhone.HTML, `id="host-drawer"`) {
		t.Fatalf("started phone frame = %+v", gamePhone)
	}
	if game.Help || game.Host != "" {
		t.Fatalf("board frame has phone chrome: %+v", game)
	}
	if help := requestWithCookie(t, handler, http.MethodGet, "/help", nil, admin); help.Code != http.StatusOK || help.Body.String() != "FAKE-HELP" {
		t.Fatalf("GET /help during the round = %d %q", help.Code, help.Body.String())
	}
	stranger := getFrame(t, handler, "/tenant", nil)
	if stranger.Tenant != "lobby" || stranger.Player || stranger.Kicked || !strings.Contains(stranger.HTML, `hx-post="/lobby/join"`) {
		t.Fatalf("stranger frame during a round = %+v", stranger)
	}

	stop := hxPost(t, handler, "/settings/stop", nil, admin)
	if stop.Code != http.StatusNoContent {
		t.Fatalf("htmx Stop = %d", stop.Code)
	}
	back := getFrame(t, handler, "/board/tenant", admin)
	if back.Tenant != "lobby" || back.Generation <= game.Generation {
		t.Fatalf("stopped board frame = %+v", back)
	}
}

func TestPlainFormPostsStillRedirect(t *testing.T) {
	t.Parallel()
	_, handler, _, _ := testGameHandler(t, 0, 0)
	admin := finishAndJoinHost(t, handler)
	requestWithCookie(t, handler, http.MethodPost, "/settings/load", url.Values{"game_id": {"fake"}}, admin)
	start := requestWithCookie(t, handler, http.MethodPost, "/settings/start", nil, admin)
	if start.Code != http.StatusSeeOther || start.Header().Get("Location") != "/" {
		t.Fatalf("plain Start = %d %q", start.Code, start.Header().Get("Location"))
	}
}

func TestHTMXStartFailureNoticesWithoutRedirect(t *testing.T) {
	t.Parallel()
	_, handler, _, _ := testGameHandler(t, 0, 0)
	admin := finishAndJoinHost(t, handler)
	rec := hxPost(t, handler, "/settings/start", nil, admin)
	if rec.Code != http.StatusNoContent || rec.Header().Get("Location") != "" {
		t.Fatalf("htmx Start before Load = %d %q", rec.Code, rec.Header().Get("Location"))
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == noticeCookieName {
			t.Fatal("htmx Start failure set the reload notice cookie")
		}
	}
}

func TestKickedPhoneFrameAsksForReload(t *testing.T) {
	t.Parallel()
	_, handler, rt, _ := testGameHandler(t, 0, 0)
	admin := finishAndJoinHost(t, handler)
	guest := request(t, handler, http.MethodPost, "/lobby/join", url.Values{"display_name": {"Bea"}}, "").Result().Cookies()
	if frame := getFrame(t, handler, "/tenant", guest); !frame.Player || frame.Kicked {
		t.Fatalf("guest frame before kick = %+v", frame)
	}
	player, ok, err := rt.room.PlayerFromRequest(requestFromCookies(guest))
	if err != nil || !ok {
		t.Fatalf("guest player = %v %v", ok, err)
	}
	if kick := hxPost(t, handler, "/settings/kick", url.Values{"player_id": {player.ID}}, admin); kick.Code != http.StatusNoContent {
		t.Fatalf("kick = %d", kick.Code)
	}
	frame := getFrame(t, handler, "/tenant", guest)
	if frame.Player || !frame.Kicked || !strings.Contains(frame.HTML, "The host kicked you") {
		t.Fatalf("kicked frame = %+v", frame)
	}
	if presence := requestWithCookie(t, handler, http.MethodGet, "/lobby/presence", nil, guest); presence.Code != http.StatusGone {
		t.Fatalf("kicked presence = %d", presence.Code)
	}
}

func TestLockedBoardTenant(t *testing.T) {
	t.Parallel()
	_, handler, rt, _ := testGameHandler(t, 0, 0)
	admin := finishAndJoinHost(t, handler)
	request(t, handler, http.MethodPost, "/lobby/join", url.Values{"display_name": {"Bea"}}, "")

	toggle := hxPost(t, handler, "/settings/admin-only-board", url.Values{"enabled": {"1"}}, admin)
	if toggle.Header().Get("HX-Trigger") != "grabbag:tenant" {
		t.Fatalf("admin-only toggle trigger = %q", toggle.Header().Get("HX-Trigger"))
	}

	locked := getFrame(t, handler, "/board/tenant", nil)
	if locked.Tenant != lockedTenantID || locked.Stream != streamLocked || !strings.Contains(locked.HTML, "Host screen only") {
		t.Fatalf("locked frame = %+v", locked)
	}
	if len(locked.Assets.CSS)+len(locked.Assets.JS)+len(locked.Assets.External) != 0 {
		t.Fatalf("locked assets = %+v", locked.Assets)
	}
	for _, secret := range []string{"Bea", "Ada", "ui-qr", "192.168.10.24", "board-roster"} {
		if strings.Contains(locked.HTML, secret) {
			t.Fatalf("locked frame leaks %q: %s", secret, locked.HTML)
		}
	}
	page := request(t, handler, http.MethodGet, "/board", nil, "").Body.String()
	if !strings.Contains(page, `data-tenant="locked"`) || !strings.Contains(page, `data-stream="`+streamLocked+`"`) ||
		strings.Contains(page, "Bea") || strings.Contains(page, "lobby.js") {
		t.Fatalf("locked board shell = %s", page)
	}

	unlocked := getFrame(t, handler, "/board/tenant", admin)
	if unlocked.Tenant != "lobby" || unlocked.Stream != streamRoom || !strings.Contains(unlocked.HTML, "Bea") {
		t.Fatalf("admin board frame = %+v", unlocked)
	}
	if unlocked.Generation <= locked.Generation-1 || rt.generation.Load() == 0 {
		t.Fatalf("generation did not move with the lock: %d", rt.generation.Load())
	}
}

func TestLockedStreamCarriesOnlyTenantAndTheme(t *testing.T) {
	t.Parallel()
	_, handler, rt, _ := testGameHandler(t, 0, 0)
	finishAndJoinHost(t, handler)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+streamLocked, nil)
	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	reader := bufio.NewReader(resp.Body)
	readHostSSE(t, reader, ": connected")

	rt.log.Write("Secret log line")
	rt.notify("board", "info", "Secret notice", 3)
	rt.events.Publish("roster")
	rt.events.PublishData("theme", "neon-light")
	rt.events.Publish("tenant")

	var got []string
	for len(got) < 2 {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(line, "Secret") {
			t.Fatalf("locked stream leaked %q", line)
		}
		if name, ok := strings.CutPrefix(strings.TrimSpace(line), "event: "); ok {
			got = append(got, name)
		}
	}
	if got[0] != "theme" || got[1] != "tenant" {
		t.Fatalf("locked stream events = %v", got)
	}
}

func TestShellErrorGoesToTheLog(t *testing.T) {
	t.Parallel()
	_, handler, rt, _ := testGameHandler(t, 0, 0)
	finishAndJoinHost(t, handler)
	body := `{"surface":"board","tenant":"quips","phase":"mount","message":"TypeError: x is undefined\n  at mount"}`
	req := httptest.NewRequest(http.MethodPost, "http://grabbag.test/shell/error", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("shell error = %d %q", rec.Code, rec.Body.String())
	}
	lines := strings.Join(rt.log.Lines(), "\n")
	if !strings.Contains(lines, "shell error: board quips mount: TypeError: x is undefined at mount") {
		t.Fatalf("log = %q", lines)
	}

	bad := httptest.NewRequest(http.MethodPost, "http://grabbag.test/shell/error", strings.NewReader("not json"))
	badRec := httptest.NewRecorder()
	handler.ServeHTTP(badRec, bad)
	if badRec.Code != http.StatusBadRequest {
		t.Fatalf("bad shell error = %d", badRec.Code)
	}
}

func TestBootIDDiffersPerProcess(t *testing.T) {
	t.Parallel()
	_, _, a, _ := testGameHandler(t, 0, 0)
	_, _, b, _ := testGameHandler(t, 0, 0)
	if a.boot == "" || a.boot == b.boot {
		t.Fatalf("boot ids %q and %q", a.boot, b.boot)
	}
}

// TestGameStaticServesTheRequestedGame checks /games/<id>/static/ serves
// that game's files whichever game is loaded, so a slow phone never gets
// another game's script under the first game's URL.
func TestGameStaticServesTheRequestedGame(t *testing.T) {
	t.Parallel()
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	handler, _, err := newHandler(db, "", games.Catalog())
	if err != nil {
		t.Fatal(err)
	}
	admin := finishAndJoinHost(t, handler)
	for _, loaded := range []string{"quips", "testing"} {
		requestWithCookie(t, handler, http.MethodPost, "/settings/load", url.Values{"game_id": {loaded}}, admin)
		for _, id := range []string{"apples", "quips", "testing", "borrowedtruths"} {
			rec := requestWithCookie(t, handler, http.MethodGet, games.StaticPath(id)+"game.js", nil, admin)
			if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `grabbagShell.register("`+id+`"`) {
				t.Fatalf("with %s loaded, %s game.js = %d, registers %s? %v", loaded, id, rec.Code, id, strings.Contains(rec.Body.String(), id))
			}
		}
	}
	if rec := requestWithCookie(t, handler, http.MethodGet, "/games/nope/static/game.js", nil, admin); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown game static = %d", rec.Code)
	}
}

// frameParts is what the phone shell document shows around the stage.
type frameParts struct {
	helpHidden  bool
	leaveHidden bool
	host        string
	stage       string
}

var (
	helpButtonRe = regexp.MustCompile(`(?s)<button\s[^>]*id="shell-help-button"[^>]*>`)
	leaveFormRe  = regexp.MustCompile(`(?s)<form\s[^>]*id="shell-leave"[^>]*>`)
	hostPanelRe  = regexp.MustCompile(`(?s)<div id="shell-host" class="shell-host">(.*?)</div>\s*<div class="shell-main">`)
	stageRe      = regexp.MustCompile(`(?s)<div\s+id="shell-stage".*?>(.*)</div>\s*</div>\s*</div>\s*<div id="shell-help"`)
)

func shellFrameParts(t *testing.T, page string) frameParts {
	t.Helper()
	help, leave := helpButtonRe.FindString(page), leaveFormRe.FindString(page)
	host, stage := hostPanelRe.FindStringSubmatch(page), stageRe.FindStringSubmatch(page)
	if help == "" || leave == "" || host == nil || stage == nil {
		t.Fatalf("phone shell has no frame: %s", page)
	}
	hidden := regexp.MustCompile(`\shidden[\s>]`)
	return frameParts{
		helpHidden:  hidden.MatchString(help),
		leaveHidden: hidden.MatchString(leave),
		host:        strings.TrimSpace(host[1]),
		stage:       stage[1],
	}
}
