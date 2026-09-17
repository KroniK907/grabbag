package lobby_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNoticeChromeOnSettings(t *testing.T) {
	t.Parallel()
	_, handler, _ := testLobby(t)

	settings := lobbyRequest(t, handler, http.MethodGet, "/settings", nil, operatorCookie()).Body.String()
	if !strings.Contains(settings, `id="notice-root"`) ||
		!strings.Contains(settings, `id="notice-close"`) ||
		!strings.Contains(settings, `data-notice-targets="host"`) {
		t.Fatalf("settings missing notice chrome: %q", settings)
	}
}

func TestClaimedHostPhoneMatchesHostTarget(t *testing.T) {
	t.Parallel()
	_, handler, _ := testLobby(t)
	cookie := joinNamed(t, handler, "Host", "correct horse")
	phone := lobbyRequest(t, handler, http.MethodGet, "/", nil, cookie).Body.String()
	if !strings.Contains(phone, `id="notice-root"`) ||
		!strings.Contains(phone, `data-notice-targets="host seated"`) {
		t.Fatalf("claimed-host phone targets = %q", phone)
	}
}

func TestNoticePartialUsesCurrentURL(t *testing.T) {
	t.Parallel()
	_, handler, _ := testLobby(t)

	board := getNoticeTargets(t, handler, "http://grabbag.test/board", nil)
	if !strings.Contains(board, `data-notice-targets="board"`) {
		t.Fatalf("board targets = %q", board)
	}

	settings := getNoticeTargets(t, handler, "http://grabbag.test/settings", operatorCookie())
	if !strings.Contains(settings, `data-notice-targets="host"`) {
		t.Fatalf("settings targets = %q", settings)
	}
}

func getNoticeTargets(t *testing.T, handler http.Handler, current string, cookie *http.Cookie) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "http://grabbag.test/lobby/partials/notice-targets", nil)
	req.Header.Set("HX-Current-URL", current)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("notice-targets status = %d %q", rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}
