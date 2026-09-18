package host

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/KroniK907/grabbag/internal/games"
	"github.com/KroniK907/grabbag/internal/store"
)

func TestGameContractLoadStartStopAndDrawerPick(t *testing.T) {
	t.Parallel()
	db, handler, rt, fake := testGameHandler(t, 0, 0)
	admin := finishAndJoinHost(t, handler)

	phone := requestWithCookie(t, handler, http.MethodGet, "/", nil, admin)
	body := phone.Body.String()
	if !strings.Contains(body, "Game library") || !strings.Contains(body, "game-library-phone") {
		t.Fatalf("host drawer missing game library: %q", body)
	}
	if strings.Contains(body, ">Start<") {
		t.Fatal("Start shown before Load")
	}
	settings := requestWithCookie(t, handler, http.MethodGet, "/settings", nil, admin).Body.String()
	if strings.Contains(settings, "Game Settings") || strings.Contains(settings, "fake-settings") {
		t.Fatalf("Game Settings before Load: %q", settings)
	}

	load := requestWithCookie(t, handler, http.MethodPost, "/settings/load", url.Values{"game_id": {"fake"}}, admin)
	if load.Code != http.StatusSeeOther {
		t.Fatalf("Load status = %d %q", load.Code, load.Body.String())
	}
	if !fake.loaded {
		t.Fatal("fake was not loaded")
	}
	phone = requestWithCookie(t, handler, http.MethodGet, "/", nil, admin)
	body = phone.Body.String()
	if !strings.Contains(body, ">Start<") || !strings.Contains(body, `value="fake"`) || !strings.Contains(body, "is-loaded") {
		t.Fatalf("drawer after Load = %q", body)
	}
	settings = requestWithCookie(t, handler, http.MethodGet, "/settings", nil, admin).Body.String()
	if !strings.Contains(settings, "Game Settings") || !strings.Contains(settings, "fake-settings") {
		t.Fatalf("settings after Load = %q", settings)
	}
	board := requestWithCookie(t, handler, http.MethodGet, "/board", nil, admin).Body.String()
	if strings.Contains(board, "FAKE-BOARD") {
		t.Fatal("board handed to the game before Start")
	}
	if !strings.Contains(board, `sse:round`) || !strings.Contains(board, `hx-get="/board"`) {
		t.Fatalf("lobby board missing round swap: %q", board)
	}

	start := requestWithCookie(t, handler, http.MethodPost, "/settings/start", nil, admin)
	if start.Code != http.StatusSeeOther {
		t.Fatalf("Start status = %d %q", start.Code, start.Body.String())
	}
	again := requestWithCookie(t, handler, http.MethodPost, "/settings/start", nil, admin)
	if again.Code != http.StatusSeeOther {
		t.Fatalf("second Start status = %d", again.Code)
	}
	board = requestWithCookie(t, handler, http.MethodGet, "/board", nil, admin).Body.String()
	if !strings.Contains(board, "FAKE-BOARD") || !strings.Contains(board, "rtt-zero=yes") {
		t.Fatalf("started board = %q", board)
	}
	phone = requestWithCookie(t, handler, http.MethodGet, "/", nil, admin)
	body = phone.Body.String()
	if !strings.Contains(body, "FAKE-PHONE") || !strings.Contains(body, `action="/lobby/leave"`) || !strings.Contains(body, ">Leave<") || strings.Contains(body, "ui-gear") || !strings.Contains(body, ">Stop<") {
		t.Fatalf("in-game phone = %q", body)
	}
	if !strings.Contains(body, `hx-get="/lobby/presence"`) {
		t.Fatalf("in-game phone missing presence watch: %q", body)
	}
	if !strings.Contains(body, ">Pause<") || strings.Contains(body, ">Resume<") {
		t.Fatalf("in-game drawer pause = %q", body)
	}
	if !strings.Contains(body, `hx-post="/settings/pause"`) || !strings.Contains(body, `sse:pause`) {
		t.Fatalf("in-game drawer missing live pause: %q", body)
	}
	play := requestWithCookie(t, handler, http.MethodGet, "/play/ping", nil, admin)
	if play.Code != http.StatusOK || play.Body.String() != "play-ok" {
		t.Fatalf("play = %d %q", play.Code, play.Body.String())
	}

	if err := fake.helper.KVSet("score", []byte("7")); err != nil {
		t.Fatal(err)
	}
	stop := requestWithCookie(t, handler, http.MethodPost, "/settings/stop", nil, admin)
	if stop.Code != http.StatusSeeOther {
		t.Fatalf("Stop status = %d", stop.Code)
	}
	board = requestWithCookie(t, handler, http.MethodGet, "/board", nil, admin).Body.String()
	if strings.Contains(board, "FAKE-BOARD") {
		t.Fatal("Stop did not return Lobby board")
	}
	got, ok, err := db.KVGet(context.Background(), "fake", "score")
	if err != nil || !ok || string(got) != "7" {
		t.Fatalf("KV after Stop = %q ok=%v err=%v", got, ok, err)
	}
	play = requestWithCookie(t, handler, http.MethodGet, "/play/ping", nil, admin)
	if play.Code != http.StatusOK || play.Body.String() != "play-ok" {
		t.Fatalf("play GET after Stop = %d %q", play.Code, play.Body.String())
	}
	playPost := requestWithCookie(t, handler, http.MethodPost, "/play/ping", nil, admin)
	if playPost.Code != http.StatusNotFound {
		t.Fatalf("play POST after Stop = %d", playPost.Code)
	}
	phone = requestWithCookie(t, handler, http.MethodGet, "/", nil, admin)
	if !strings.Contains(phone.Body.String(), ">Start<") {
		t.Fatalf("Start missing after Stop: %q", phone.Body.String())
	}

	clear := requestWithCookie(t, handler, http.MethodPost, "/settings/clear-game-data", nil, admin)
	if clear.Code != http.StatusSeeOther {
		t.Fatalf("clear status = %d", clear.Code)
	}
	_, ok, err = db.KVGet(context.Background(), "fake", "score")
	if err != nil || ok {
		t.Fatalf("KV after clear ok=%v err=%v", ok, err)
	}

	off := requestWithCookie(t, handler, http.MethodPost, "/settings/shutdown", nil, admin)
	if off.Code != http.StatusSeeOther {
		t.Fatalf("Shutdown status = %d", off.Code)
	}
	settings = requestWithCookie(t, handler, http.MethodGet, "/settings", nil, admin).Body.String()
	if strings.Contains(settings, "Game Settings") {
		t.Fatalf("Game Settings after Shutdown: %q", settings)
	}
	if rt.loadedID != "" || fake.loaded {
		t.Fatalf("runtime still loaded id=%q loaded=%v", rt.loadedID, fake.loaded)
	}

	logPage := requestWithCookie(t, handler, http.MethodGet, "/settings/log", nil, admin).Body.String()
	if !strings.Contains(logPage, "Load fake") || !strings.Contains(logPage, "Start fake") || !strings.Contains(logPage, "Finish") {
		t.Fatalf("log page = %q", logPage)
	}
}

func TestLobbyBoardShowsGameRailButtons(t *testing.T) {
	t.Parallel()
	_, handler, _, fake := testGameHandler(t, 0, 0)
	fake.buttons = []games.BoardButton{
		{Label: "Public scores", Path: "/play/scores"},
		{Label: "Host only deck", Path: "/play/deck", HostOnly: true},
		{Label: "Long wrapped rail action", Path: "/play/long"},
		{Label: "Dropped fourth", Path: "/play/four"},
	}
	admin := finishAndJoinHost(t, handler)

	before := requestWithCookie(t, handler, http.MethodGet, "/board", nil, nil).Body.String()
	if strings.Contains(before, "Public scores") || strings.Contains(before, `class="ui-rail-actions"`) ||
		strings.Contains(before, "Now playing:") {
		t.Fatalf("board before Load = %q", before)
	}

	load := requestWithCookie(t, handler, http.MethodPost, "/settings/load", url.Values{"game_id": {"fake"}}, admin)
	if load.Code != http.StatusSeeOther {
		t.Fatalf("Load status = %d %q", load.Code, load.Body.String())
	}

	tv := requestWithCookie(t, handler, http.MethodGet, "/board", nil, nil).Body.String()
	if !strings.Contains(tv, `href="/play/scores"`) || !strings.Contains(tv, "Public scores") {
		t.Fatalf("TV board missing public rail button: %q", tv)
	}
	if !strings.Contains(tv, "Now playing:") || !strings.Contains(tv, `class="ui-now-playing-name"`) ||
		!strings.Contains(tv, ">fake<") {
		t.Fatalf("TV board missing loaded game name: %q", tv)
	}
	if strings.Contains(tv, "Host only deck") || strings.Contains(tv, "/play/deck") {
		t.Fatalf("TV board showed a host-only rail button: %q", tv)
	}
	if !strings.Contains(tv, "Long wrapped rail action") || strings.Contains(tv, "Dropped fourth") {
		t.Fatalf("TV board rail cap = %q", tv)
	}

	hostBoard := requestWithCookie(t, handler, http.MethodGet, "/board", nil, admin).Body.String()
	if !strings.Contains(hostBoard, `href="/play/deck"`) || !strings.Contains(hostBoard, "Host only deck") {
		t.Fatalf("host board missing host-only rail button: %q", hostBoard)
	}
	if strings.Contains(hostBoard, "Dropped fourth") {
		t.Fatalf("host board showed a fourth rail button: %q", hostBoard)
	}
}

func TestStopFillsInRoundWaiterWhenRoomIsOpen(t *testing.T) {
	t.Parallel()
	_, handler, _, _ := testGameHandler(t, 0, 0)
	admin := finishAndJoinHost(t, handler)
	requestWithCookie(t, handler, http.MethodPost, "/settings/open", nil, admin)
	requestWithCookie(t, handler, http.MethodPost, "/settings/load", url.Values{"game_id": {"fake"}}, admin)
	requestWithCookie(t, handler, http.MethodPost, "/settings/start", nil, admin)

	waitJoin := request(t, handler, http.MethodPost, "/lobby/join", url.Values{"display_name": {"Bea"}}, "")
	waitCookies := waitJoin.Result().Cookies()
	if beat := requestWithCookie(t, handler, http.MethodPost, "/lobby/heartbeat", nil, waitCookies); beat.Code != http.StatusNoContent {
		t.Fatalf("waiter heartbeat = %d", beat.Code)
	}
	inRound := requestWithCookie(t, handler, http.MethodGet, "/", nil, waitCookies).Body.String()
	if strings.Contains(inRound, "You are in line.") {
		t.Fatalf("in-round wait phone stayed on Lobby: %q", inRound)
	}

	stop := requestWithCookie(t, handler, http.MethodPost, "/settings/stop", nil, admin)
	if stop.Code != http.StatusSeeOther {
		t.Fatalf("Stop status = %d", stop.Code)
	}
	after := requestWithCookie(t, handler, http.MethodGet, "/", nil, waitCookies).Body.String()
	if strings.Contains(after, "Leave wait list") || strings.Contains(after, "You are in line.") {
		t.Fatalf("waiter still in line after Stop: %q", after)
	}
	if !strings.Contains(after, "Bea") || strings.Contains(after, "Join wait list") {
		t.Fatalf("seated phone after Stop = %q", after)
	}
}

func TestStopCyclesWhenAfterGameIsCycle(t *testing.T) {
	t.Parallel()
	_, handler, _, _ := testGameHandler(t, 0, 0)
	admin := finishAndJoinHost(t, handler)
	requestWithCookie(t, handler, http.MethodPost, "/settings/open", nil, admin)
	requestWithCookie(t, handler, http.MethodPost, "/settings/seat-cap", url.Values{"seat_cap": {"2"}}, admin)
	twoJoin := request(t, handler, http.MethodPost, "/lobby/join", url.Values{"display_name": {"Two"}}, "")
	twoCookies := twoJoin.Result().Cookies()
	if beat := requestWithCookie(t, handler, http.MethodPost, "/lobby/heartbeat", nil, twoCookies); beat.Code != http.StatusNoContent {
		t.Fatalf("two heartbeat = %d", beat.Code)
	}
	requestWithCookie(t, handler, http.MethodPost, "/settings/cycle-mode", url.Values{"cycle_mode": {"cycle"}}, admin)
	requestWithCookie(t, handler, http.MethodPost, "/settings/load", url.Values{"game_id": {"fake"}}, admin)
	requestWithCookie(t, handler, http.MethodPost, "/settings/start", nil, admin)

	waitJoin := request(t, handler, http.MethodPost, "/lobby/join", url.Values{"display_name": {"Wait"}}, "")
	waitCookies := waitJoin.Result().Cookies()
	if beat := requestWithCookie(t, handler, http.MethodPost, "/lobby/heartbeat", nil, waitCookies); beat.Code != http.StatusNoContent {
		t.Fatalf("waiter heartbeat = %d", beat.Code)
	}

	stop := requestWithCookie(t, handler, http.MethodPost, "/settings/stop", nil, admin)
	if stop.Code != http.StatusSeeOther {
		t.Fatalf("Stop status = %d", stop.Code)
	}
	after := requestWithCookie(t, handler, http.MethodGet, "/", nil, waitCookies).Body.String()
	if strings.Contains(after, "Leave wait list") || strings.Contains(after, "You are in line.") {
		t.Fatalf("waiter still in line after cycle Stop: %q", after)
	}
}

func TestStartedWaitPhoneGetsGameBody(t *testing.T) {
	t.Parallel()
	_, handler, _, _ := testGameHandler(t, 0, 0)
	admin := finishAndJoinHost(t, handler)
	requestWithCookie(t, handler, http.MethodPost, "/settings/load", url.Values{"game_id": {"fake"}}, admin)
	requestWithCookie(t, handler, http.MethodPost, "/settings/start", nil, admin)

	waitJoin := request(t, handler, http.MethodPost, "/lobby/join", url.Values{"display_name": {"Bea"}}, "")
	waitCookies := waitJoin.Result().Cookies()
	phone := requestWithCookie(t, handler, http.MethodGet, "/", nil, waitCookies)
	body := phone.Body.String()
	if !strings.Contains(body, "FAKE-PHONE") || !strings.Contains(body, `action="/lobby/leave"`) || !strings.Contains(body, ">Leave<") || strings.Contains(body, "ui-gear") {
		t.Fatalf("wait phone missing game wrap: %q", body)
	}
	if strings.Contains(body, "You are in line.") {
		t.Fatalf("wait phone stayed on Lobby: %q", body)
	}
}

func TestStartedAudiencePhoneGetsGameBody(t *testing.T) {
	t.Parallel()
	_, handler, _, _ := testGameHandler(t, 0, 0)
	admin := finishAndJoinHost(t, handler)
	requestWithCookie(t, handler, http.MethodPost, "/settings/load", url.Values{"game_id": {"fake"}}, admin)
	requestWithCookie(t, handler, http.MethodPost, "/settings/start", nil, admin)

	join := request(t, handler, http.MethodPost, "/lobby/join", url.Values{"display_name": {"Bea"}}, "")
	cookies := join.Result().Cookies()
	optOut := requestWithCookie(t, handler, http.MethodPost, "/lobby/wait", nil, cookies)
	if optOut.Code != http.StatusSeeOther {
		t.Fatalf("wait-toggle status = %d", optOut.Code)
	}
	phone := requestWithCookie(t, handler, http.MethodGet, "/", nil, cookies)
	body := phone.Body.String()
	if !strings.Contains(body, "FAKE-PHONE") || !strings.Contains(body, `action="/lobby/leave"`) || !strings.Contains(body, ">Leave<") || strings.Contains(body, "ui-gear") {
		t.Fatalf("audience phone missing game wrap: %q", body)
	}
	if strings.Contains(body, "You are watching") {
		t.Fatalf("audience phone stayed on Lobby: %q", body)
	}
}

func TestStartedUnknownCookieStaysOnLobbyJoin(t *testing.T) {
	t.Parallel()
	_, handler, _, _ := testGameHandler(t, 0, 0)
	admin := finishAndJoinHost(t, handler)
	requestWithCookie(t, handler, http.MethodPost, "/settings/load", url.Values{"game_id": {"fake"}}, admin)
	requestWithCookie(t, handler, http.MethodPost, "/settings/start", nil, admin)

	phone := request(t, handler, http.MethodGet, "/", nil, "")
	body := phone.Body.String()
	if strings.Contains(body, "FAKE-PHONE") {
		t.Fatalf("unknown cookie used the game body: %q", body)
	}
	if !strings.Contains(body, `name="display_name"`) {
		t.Fatalf("unknown cookie left Lobby join: %q", body)
	}
}

func TestStartRefusedBeforeLoadOrZeroSeatedOrBelowMin(t *testing.T) {
	t.Parallel()
	_, handler, _, _ := testGameHandler(t, 2, 8)
	admin := cookieAfterSetup(t, handler)

	refused := requestWithCookie(t, handler, http.MethodPost, "/settings/start", nil, admin)
	if refused.Code != http.StatusSeeOther {
		t.Fatalf("Start before Load = %d", refused.Code)
	}

	join := requestWithCookie(t, handler, http.MethodPost, "/lobby/join", url.Values{
		"display_name":   {"Ada"},
		"admin_password": {"correct horse"},
	}, admin)
	host := join.Result().Cookies()
	load := requestWithCookie(t, handler, http.MethodPost, "/settings/load", url.Values{"game_id": {"fake"}}, host)
	if load.Code != http.StatusSeeOther {
		t.Fatalf("Load = %d %q", load.Code, load.Body.String())
	}
	start := requestWithCookie(t, handler, http.MethodPost, "/settings/start", nil, host)
	if start.Code != http.StatusSeeOther {
		t.Fatalf("Start below min = %d %q", start.Code, start.Body.String())
	}
}

func TestAutoStartUsesExistingStartWrite(t *testing.T) {
	t.Parallel()
	_, handler, rt, fake := testGameHandler(t, 0, 0)
	admin := finishAndJoinHost(t, handler)
	if beat := requestWithCookie(t, handler, http.MethodPost, "/lobby/heartbeat", nil, admin); beat.Code != http.StatusNoContent {
		t.Fatalf("heartbeat = %d", beat.Code)
	}
	requestWithCookie(t, handler, http.MethodPost, "/settings/load", url.Values{"game_id": {"fake"}}, admin)
	requestWithCookie(t, handler, http.MethodPost, "/settings/auto-start", url.Values{"enabled": {"1"}}, admin)
	ready := requestWithCookie(t, handler, http.MethodPost, "/lobby/ready", nil, admin)
	if ready.Code != http.StatusSeeOther {
		t.Fatalf("ready = %d %q", ready.Code, ready.Body.String())
	}
	if !fake.started || !rt.started {
		t.Fatalf("auto-start did not Start fake=%v runtime=%v", fake.started, rt.started)
	}
}

func TestLoadRefusedAboveGameMax(t *testing.T) {
	t.Parallel()
	_, handler, _, _ := testGameHandler(t, 0, 1)
	admin := finishAndJoinHost(t, handler)
	requestWithCookie(t, handler, http.MethodPost, "/settings/open", nil, admin)
	requestWithCookie(t, handler, http.MethodPost, "/lobby/join", url.Values{"display_name": {"Bea"}}, nil)

	load := requestWithCookie(t, handler, http.MethodPost, "/settings/load", url.Values{"game_id": {"fake"}}, admin)
	if load.Code != http.StatusSeeOther {
		t.Fatalf("Load above max = %d %q", load.Code, load.Body.String())
	}
}

func TestPauseResumeAndAutoPause(t *testing.T) {
	t.Parallel()
	_, handler, rt, fake := testGameHandler(t, 0, 0)
	admin := finishAndJoinHost(t, handler)
	requestWithCookie(t, handler, http.MethodPost, "/settings/load", url.Values{"game_id": {"fake"}}, admin)
	requestWithCookie(t, handler, http.MethodPost, "/settings/start", nil, admin)
	paused := requestWithCookie(t, handler, http.MethodPost, "/settings/pause", nil, admin)
	if paused.Code != http.StatusNoContent {
		t.Fatalf("Pause status = %d %q", paused.Code, paused.Body.String())
	}
	if !fake.paused {
		t.Fatal("Pause did not reach the game")
	}
	pausedPhone := requestWithCookie(t, handler, http.MethodGet, "/", nil, admin).Body.String()
	if !strings.Contains(pausedPhone, ">Resume<") || strings.Contains(pausedPhone, ">Pause<") {
		t.Fatalf("paused drawer = %q", pausedPhone)
	}
	resumed := requestWithCookie(t, handler, http.MethodPost, "/settings/resume", nil, admin)
	if resumed.Code != http.StatusNoContent {
		t.Fatalf("Resume status = %d %q", resumed.Code, resumed.Body.String())
	}
	if fake.paused {
		t.Fatal("Resume left the game paused")
	}
	running := requestWithCookie(t, handler, http.MethodGet, "/", nil, admin).Body.String()
	if !strings.Contains(running, ">Pause<") || strings.Contains(running, ">Resume<") {
		t.Fatalf("resumed drawer = %q", running)
	}

	auto := requestWithCookie(t, handler, http.MethodPost, "/settings/auto-pause", url.Values{"enabled": {"1"}}, admin)
	if auto.Code != http.StatusSeeOther {
		t.Fatalf("auto-pause = %d", auto.Code)
	}
	player, ok, err := rt.room.PlayerFromRequest(requestFromCookies(admin))
	if err != nil || !ok {
		t.Fatal(err)
	}
	if err := rt.markDisconnected(context.Background(), player.ID); err != nil {
		t.Fatal(err)
	}
	if !fake.paused {
		t.Fatal("auto-pause did not pause on seated disconnect")
	}
}

func TestMidGameDisconnectReservesSeatUntilStop(t *testing.T) {
	t.Parallel()
	db, handler, rt, _ := testGameHandler(t, 0, 0)
	admin := finishAndJoinHost(t, handler)
	requestWithCookie(t, handler, http.MethodPost, "/settings/load", url.Values{"game_id": {"fake"}}, admin)
	requestWithCookie(t, handler, http.MethodPost, "/settings/start", nil, admin)
	requestWithCookie(t, handler, http.MethodPost, "/settings/open", nil, admin)

	host, ok, err := rt.room.PlayerFromRequest(requestFromCookies(admin))
	if err != nil || !ok {
		t.Fatal(err)
	}
	if err := rt.room.SetConnected(context.Background(), host.ID, false); err != nil {
		t.Fatal(err)
	}
	requestWithCookie(t, handler, http.MethodPost, "/lobby/join", url.Values{"display_name": {"Bea"}}, nil)

	var seated int
	if err := db.SQL().QueryRow(`SELECT COUNT(*) FROM roster WHERE seated = 1`).Scan(&seated); err != nil {
		t.Fatal(err)
	}
	if seated != 1 {
		t.Fatalf("seated after mid-game join = %d, want reserved host only", seated)
	}

	requestWithCookie(t, handler, http.MethodPost, "/settings/stop", nil, admin)
	var n int
	if err := db.SQL().QueryRow(`SELECT COUNT(*) FROM roster WHERE player_id = ?`, host.ID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatal("disconnected host still on the roster after Stop")
	}
}

func testGameHandler(t *testing.T, min, max int) (*store.DB, http.Handler, *runtime, *fakeGame) {
	t.Helper()
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	fake := &fakeGame{min: min, max: max}
	handler, rt, err := newHandler(db, "http://192.168.10.24:8654/", []games.Factory{{
		ID:  "fake",
		New: func() games.Game { return fake },
	}})
	if err != nil {
		t.Fatal(err)
	}
	return db, handler, rt, fake
}

func finishAndJoinHost(t *testing.T, handler http.Handler) []*http.Cookie {
	t.Helper()
	setup := url.Values{"password": {"correct horse"}, "confirm": {"correct horse"}}
	request(t, handler, http.MethodPost, "/setup", setup, "")
	join := request(t, handler, http.MethodPost, "/lobby/join", url.Values{
		"display_name":   {"Ada"},
		"admin_password": {"correct horse"},
	}, "")
	cookies := join.Result().Cookies()
	if beat := requestWithCookie(t, handler, http.MethodPost, "/lobby/heartbeat", nil, cookies); beat.Code != http.StatusNoContent {
		t.Fatalf("host heartbeat = %d", beat.Code)
	}
	return cookies
}

func cookieAfterSetup(t *testing.T, handler http.Handler) []*http.Cookie {
	t.Helper()
	setup := url.Values{"password": {"correct horse"}, "confirm": {"correct horse"}}
	rec := request(t, handler, http.MethodPost, "/setup", setup, "")
	return rec.Result().Cookies()
}

func requestWithCookie(t *testing.T, handler http.Handler, method, path string, form url.Values, cookies []*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	body := strings.NewReader("")
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req := httptest.NewRequest(method, "http://grabbag.test"+path, body)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestTestingInfoFromLobbyBoard(t *testing.T) {
	t.Parallel()
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	handler, _, err := newHandler(db, "http://192.168.10.24:8654/", games.Catalog())
	if err != nil {
		t.Fatal(err)
	}
	admin := finishAndJoinHost(t, handler)
	load := requestWithCookie(t, handler, http.MethodPost, "/settings/load", url.Values{"game_id": {"testing"}}, admin)
	if load.Code != http.StatusSeeOther {
		t.Fatalf("Load testing = %d %q", load.Code, load.Body.String())
	}
	board := requestWithCookie(t, handler, http.MethodGet, "/board", nil, nil).Body.String()
	if !strings.Contains(board, "About Testing") || !strings.Contains(board, `href="/play/info"`) ||
		!strings.Contains(board, "Now playing:") || !strings.Contains(board, `class="ui-now-playing-name"`) ||
		!strings.Contains(board, ">Testing<") {
		t.Fatalf("lobby board missing About Testing: %q", board)
	}
	info := requestWithCookie(t, handler, http.MethodGet, "/play/info", nil, nil)
	body := info.Body.String()
	if info.Code != http.StatusOK ||
		!strings.Contains(body, "prove the host game contract") ||
		!strings.Contains(body, "Back to board") {
		t.Fatalf("info = %d %s", info.Code, body)
	}
}

func TestLoadPublishesRosterForBoardButtons(t *testing.T) {
	t.Parallel()
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	handler, _, err := newHandler(db, "http://192.168.10.24:8654/", games.Catalog())
	if err != nil {
		t.Fatal(err)
	}
	admin := finishAndJoinHost(t, handler)

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/lobby/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	stream, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stream.Body.Close() })
	events := bufio.NewReader(stream.Body)
	readHostSSE(t, events, ": connected")

	load := requestWithCookie(t, handler, http.MethodPost, "/settings/load", url.Values{"game_id": {"testing"}}, admin)
	if load.Code != http.StatusSeeOther {
		t.Fatalf("Load testing = %d %q", load.Code, load.Body.String())
	}
	readHostSSE(t, events, "event: roster")
}

func readHostSSE(t *testing.T, reader *bufio.Reader, want string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		var event strings.Builder
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				t.Fatalf("waiting for %q: %v; got %q", want, err, event.String())
			}
			event.WriteString(line)
			if line == "\n" {
				break
			}
		}
		if strings.Contains(event.String(), want) {
			return
		}
	}
	t.Fatalf("timed out waiting for %q", want)
}

func requestFromCookies(cookies []*http.Cookie) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "http://grabbag.test/", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	return req
}
