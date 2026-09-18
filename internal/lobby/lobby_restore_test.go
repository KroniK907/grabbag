package lobby_test

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/KroniK907/grabbag/internal/lobby"
)

func TestRestartRestore(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		run  func(*testing.T)
	}{
		{name: "empty skip", run: testEmptyRosterSkipsKeepOrClear},
		{name: "frozen writes and Keep lock", run: testRestartAsksKeepOrClearAndFreezesWrites},
		{name: "night Clear", run: testNightClearLeavesSettingsAndClosesRoom},
		{name: "admin-only board", run: testAdminOnlyBoardOmitsRoster},
		{name: "post-Keep debounce", run: testKickClockStartsAtKeepNotBoot},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.run(t)
		})
	}
}

func testEmptyRosterSkipsKeepOrClear(t *testing.T) {
	t.Helper()
	_, handler, room := testLobby(t)
	if room.RestorePending() {
		t.Fatal("empty sqlite asked keep-or-clear")
	}
	board := lobbyRequest(t, handler, http.MethodGet, "/board", nil, nil).Body.String()
	if strings.Contains(board, "Waiting for the host to decide to keep or clear this room") {
		t.Fatalf("empty board overlay = %q", board)
	}
	join := lobbyRequest(t, handler, http.MethodPost, "/lobby/join", url.Values{"display_name": {"Ada"}}, nil)
	if join.Code != http.StatusSeeOther {
		t.Fatalf("join on empty room = %d", join.Code)
	}
}

func testRestartAsksKeepOrClearAndFreezesWrites(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	db, handler, _ := openTestLobby(t, dir, nil)
	host := joinNamed(t, handler, "Ada", "correct horse")
	guestCookie := joinNamed(t, handler, "Bea", "")
	_ = db.Close()

	_, handler, room := openTestLobby(t, dir, nil)
	if !room.RestorePending() {
		t.Fatal("roster reopen skipped keep-or-clear")
	}

	board := lobbyRequest(t, handler, http.MethodGet, "/board", nil, nil).Body.String()
	if !strings.Contains(board, "Waiting for the host to decide to keep or clear this room") ||
		!strings.Contains(board, "Ada") ||
		!strings.Contains(board, "Bea") ||
		strings.Contains(board, `action="/settings/keep"`) {
		t.Fatalf("board overlay = %q", board)
	}
	if !strings.Contains(board, `class="keep-overlay"`) {
		t.Fatal("board missing keep overlay")
	}

	phone := lobbyRequestAll(t, handler, http.MethodGet, "/", nil, host, operatorCookie()).Body.String()
	if !strings.Contains(phone, "keep this room") || strings.Contains(phone, `action="/lobby/ready"`) {
		t.Fatalf("restored phone = %q", phone)
	}
	if !strings.Contains(phone, `id="restore-modal"`) ||
		!strings.Contains(phone, `action="/settings/keep"`) ||
		!strings.Contains(phone, `action="/settings/clear-room"`) {
		t.Fatalf("host phone missing Keep/Clear modal: %q", phone)
	}
	drawerAt := strings.Index(phone, `id="host-drawer"`)
	if drawerAt < 0 {
		t.Fatal("host phone missing drawer")
	}
	drawerEnd := strings.Index(phone[drawerAt:], "</aside>")
	if drawerEnd < 0 {
		t.Fatal("host drawer missing close tag")
	}
	drawer := phone[drawerAt : drawerAt+drawerEnd]
	if strings.Contains(drawer, `action="/settings/keep"`) || strings.Contains(drawer, `action="/settings/clear-room"`) {
		t.Fatalf("host drawer still has Keep/Clear: %q", drawer)
	}
	if strings.Contains(phone, `action="/settings/start"`) || strings.Contains(phone, `action="/settings/sit"`) {
		t.Fatalf("Start/sit shown while pending: %q", phone)
	}

	guestPhone := lobbyRequest(t, handler, http.MethodGet, "/", nil, guestCookie).Body.String()
	if strings.Contains(guestPhone, `id="restore-modal"`) ||
		strings.Contains(guestPhone, `action="/settings/keep"`) ||
		strings.Contains(guestPhone, `action="/settings/clear-room"`) {
		t.Fatalf("guest phone has Keep/Clear: %q", guestPhone)
	}

	guest := lobbyRequest(t, handler, http.MethodGet, "/", nil, nil).Body.String()
	if !strings.Contains(guest, `action="/lobby/join"`) {
		t.Fatalf("anonymous phone = %q", guest)
	}

	join := lobbyRequest(t, handler, http.MethodPost, "/lobby/join", url.Values{"display_name": {"Cara"}}, nil)
	if join.Code != http.StatusConflict {
		t.Fatalf("join while pending = %d", join.Code)
	}
	leave := lobbyRequest(t, handler, http.MethodPost, "/lobby/leave", nil, host)
	if leave.Code != http.StatusConflict {
		t.Fatalf("leave while pending = %d", leave.Code)
	}
	ready := lobbyRequest(t, handler, http.MethodPost, "/lobby/ready", nil, host)
	if ready.Code != http.StatusConflict {
		t.Fatalf("ready while pending = %d", ready.Code)
	}
	kick := lobbyRequest(t, handler, http.MethodPost, "/settings/kick", url.Values{"player_id": {"nope"}}, operatorCookie())
	if kick.Code != http.StatusConflict {
		t.Fatalf("kick while pending = %d", kick.Code)
	}
	beat := lobbyRequest(t, handler, http.MethodPost, "/lobby/heartbeat", nil, host)
	if beat.Code != http.StatusNoContent {
		t.Fatalf("heartbeat while pending = %d", beat.Code)
	}

	settings := lobbyRequest(t, handler, http.MethodGet, "/settings", nil, operatorCookie()).Body.String()
	if !strings.Contains(settings, `action="/settings/keep"`) || !strings.Contains(settings, `action="/settings/clear-room"`) {
		t.Fatalf("settings missing Keep/Clear: %q", settings)
	}

	keep := lobbyRequest(t, handler, http.MethodPost, "/settings/keep", nil, operatorCookie())
	if keep.Code != http.StatusSeeOther {
		t.Fatalf("Keep = %d %q", keep.Code, keep.Body.String())
	}
	if keep.Header().Get("Location") != "/" {
		t.Fatalf("drawer Keep Location = %q, want /", keep.Header().Get("Location"))
	}
	if room.RestorePending() {
		t.Fatal("Keep left pending")
	}
	if !strings.Contains(settings, `name="return" value="/settings"`) {
		t.Fatalf("settings Keep missing return: %q", settings)
	}
	again := lobbyRequest(t, handler, http.MethodPost, "/settings/keep", nil, operatorCookie())
	if again.Code != http.StatusSeeOther {
		t.Fatalf("second Keep = %d", again.Code)
	}
	clear := lobbyRequest(t, handler, http.MethodPost, "/settings/clear-room", nil, operatorCookie())
	if clear.Code != http.StatusSeeOther {
		t.Fatalf("Clear after Keep = %d", clear.Code)
	}
	if _, ok := findPlayer(t, room, host); !ok {
		t.Fatal("Clear after Keep wiped the roster")
	}
	late := lobbyRequest(t, handler, http.MethodPost, "/lobby/join", url.Values{"display_name": {"Cara"}}, nil)
	if late.Code != http.StatusSeeOther {
		t.Fatalf("join after Keep = %d", late.Code)
	}
}

func testNightClearLeavesSettingsAndClosesRoom(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	db, handler, _ := openTestLobby(t, dir, nil)
	joinNamed(t, handler, "Ada", "correct horse")
	lobbyRequest(t, handler, http.MethodPost, "/settings/open", nil, operatorCookie())
	lobbyRequest(t, handler, http.MethodPost, "/settings/seat-cap", url.Values{"seat_cap": {"6"}}, operatorCookie())
	_ = db.Close()

	_, handler, room := openTestLobby(t, dir, nil)
	cleared := lobbyRequest(t, handler, http.MethodPost, "/settings/clear-room", nil, operatorCookie())
	if cleared.Code != http.StatusSeeOther {
		t.Fatalf("Clear = %d %q", cleared.Code, cleared.Body.String())
	}
	if room.RestorePending() {
		t.Fatal("Clear left pending")
	}
	board := lobbyRequest(t, handler, http.MethodGet, "/board", nil, nil).Body.String()
	if strings.Contains(board, "Waiting for the host to decide to keep or clear this room") || strings.Contains(board, "Ada") {
		t.Fatalf("cleared board = %q", board)
	}
	settings := lobbyRequest(t, handler, http.MethodGet, "/settings", nil, operatorCookie()).Body.String()
	if !strings.Contains(settings, `value="6"`) {
		t.Fatalf("seat cap lost: %q", settings)
	}
	if !strings.Contains(settings, ">Closed<") {
		t.Fatalf("room not closed: %q", settings)
	}
	again := lobbyRequest(t, handler, http.MethodPost, "/settings/clear-room", nil, operatorCookie())
	if again.Code != http.StatusSeeOther {
		t.Fatalf("second Clear = %d", again.Code)
	}
}

func testAdminOnlyBoardOmitsRoster(t *testing.T) {
	t.Helper()
	_, handler, _ := testLobby(t)
	joinNamed(t, handler, "Ada", "correct horse")
	lobbyRequest(t, handler, http.MethodPost, "/settings/admin-only-board", url.Values{"enabled": {"1"}}, operatorCookie())

	locked := lobbyRequest(t, handler, http.MethodGet, "/board", nil, nil).Body.String()
	if strings.Contains(locked, "Ada") || strings.Contains(locked, `sse-connect="/lobby/events"`) {
		t.Fatalf("locked board leaked roster: %q", locked)
	}
	if !strings.Contains(locked, "Host screen only") {
		t.Fatalf("locked board = %q", locked)
	}
	adminBoard := lobbyRequest(t, handler, http.MethodGet, "/board", nil, operatorCookie()).Body.String()
	if !strings.Contains(adminBoard, "Ada") {
		t.Fatalf("admin board missing roster: %q", adminBoard)
	}
}

func testKickClockStartsAtKeepNotBoot(t *testing.T) {
	t.Helper()
	clk := &testClock{now: time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)}
	dir := t.TempDir()
	db, handler, _ := openTestLobby(t, dir, func(c *lobby.Config) {
		c.Clock = clk.Now
	})
	joinNamed(t, handler, "Host", "correct horse")
	lobbyRequest(t, handler, http.MethodPost, "/settings/open", nil, operatorCookie())
	guest := joinWithoutBeat(t, handler, "Bea", "")
	lobbyRequest(t, handler, http.MethodPost, "/settings/kick-timeout", url.Values{"kick_timeout": {"1"}}, operatorCookie())
	lobbyRequest(t, handler, http.MethodPost, "/settings/disconnect-after", url.Values{"disconnect_after": {"0"}}, operatorCookie())
	lobbyRequest(t, handler, http.MethodPost, "/settings/protect-host", url.Values{"enabled": {"0"}}, operatorCookie())
	_ = db.Close()

	_, handler, room := openTestLobby(t, dir, func(c *lobby.Config) {
		c.Clock = clk.Now
	})
	clk.advance(lobby.StartupDebounce + 3*time.Second)
	if err := room.TickLiveness(context.Background(), clk.Now()); err != nil {
		t.Fatal(err)
	}
	if _, ok := findPlayer(t, room, guest); !ok {
		t.Fatal("guest was kicked while keep-or-clear was pending")
	}

	lobbyRequest(t, handler, http.MethodPost, "/settings/keep", nil, operatorCookie())
	clk.advance(2 * time.Second)
	if err := room.TickLiveness(context.Background(), clk.Now()); err != nil {
		t.Fatal(err)
	}
	if _, ok := findPlayer(t, room, guest); !ok {
		t.Fatal("guest was kicked before post-Keep debounce")
	}
	clk.advance(lobby.StartupDebounce + 2*time.Second)
	if err := room.TickLiveness(context.Background(), clk.Now()); err != nil {
		t.Fatal(err)
	}
	if _, ok := findPlayer(t, room, guest); ok {
		t.Fatal("guest was not kicked after Keep debounce")
	}
}
