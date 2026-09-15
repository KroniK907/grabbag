package host

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/KroniK907/hackbox/internal/store"
)

func TestSetupLifecycle(t *testing.T) {
	t.Parallel()
	db, handler := testHandler(t)

	assertRedirect(t, handler, http.MethodGet, "/", nil, "/setup")
	assertStatus(t, handler, http.MethodGet, "/setup", nil, http.StatusOK)

	bad := url.Values{"password": {"abcdefgh"}, "confirm": {"different"}}
	assertStatus(t, handler, http.MethodPost, "/setup", bad, http.StatusBadRequest)
	hasHash, err := db.HasAdminHash(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if hasHash {
		t.Fatal("mismatched passwords wrote a hash")
	}

	short := url.Values{"password": {"short"}, "confirm": {"short"}}
	assertStatus(t, handler, http.MethodPost, "/setup", short, http.StatusBadRequest)
	hasHash, err = db.HasAdminHash(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if hasHash {
		t.Fatal("short password wrote a hash")
	}

	form := url.Values{"password": {"correct horse"}, "confirm": {"correct horse"}}
	rec := request(t, handler, http.MethodPost, "/setup", form, "")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("Finish status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != "/board" {
		t.Fatalf("Finish Location = %q, want /board", got)
	}

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("Finish cookies = %d, want 1", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != adminCookieName || cookie.Value == "" {
		t.Fatalf("admin cookie = %#v", cookie)
	}
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/" || cookie.Domain != "" {
		t.Fatalf("admin cookie flags = %#v", cookie)
	}
	if cookie.Secure {
		t.Fatal("plain LAN HTTP cookie must not be Secure")
	}
	hasSession, err := db.HasAdminSession(context.Background(), cookie.Value)
	if err != nil {
		t.Fatal(err)
	}
	if !hasSession {
		t.Fatal("cookie does not identify a server-side session")
	}

	hash, err := db.AdminHash(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") || strings.Contains(hash, "correct horse") {
		t.Fatalf("stored value is not an argon2id hash: %q", hash)
	}

	assertStatus(t, handler, http.MethodPost, "/setup", form, http.StatusConflict)
	assertRedirect(t, handler, http.MethodGet, "/setup", nil, "/board")
	for _, path := range []string{"/", "/board", "/settings"} {
		assertStatus(t, handler, http.MethodGet, path, nil, http.StatusOK)
	}

	board := request(t, handler, http.MethodGet, "/board", nil, "")
	if !strings.Contains(board.Body.String(), "http://192.168.10.24:8654/") {
		t.Fatalf("board page missing LAN join URL: %q", board.Body.String())
	}
	phone := request(t, handler, http.MethodGet, "/", nil, "")
	if strings.Contains(phone.Body.String(), "http://192.168.10.24:8654/") {
		t.Fatal("phone stub should not show the LAN join URL")
	}
}

func TestAdminCookieIsSecureOnLocalhost(t *testing.T) {
	t.Parallel()
	form := url.Values{"password": {"correct horse"}, "confirm": {"correct horse"}}
	for _, host := range []string{"localhost:8654", "127.0.0.1:8654", "[::1]:8654"} {
		_, handler := testHandler(t)
		rec := request(t, handler, http.MethodPost, "/setup", form, host)
		cookies := rec.Result().Cookies()
		if len(cookies) != 1 || !cookies[0].Secure {
			t.Fatalf("%s admin cookie = %#v, want Secure", host, cookies)
		}
	}
}

func TestCrossOriginFinishIsForbidden(t *testing.T) {
	t.Parallel()
	db, handler := testHandler(t)
	form := url.Values{"password": {"correct horse"}, "confirm": {"correct horse"}}
	req := httptest.NewRequest(http.MethodPost, "http://hackbox.test/setup", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("cross-origin status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	hasHash, err := db.HasAdminHash(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if hasHash {
		t.Fatal("cross-origin Finish wrote a hash")
	}
}

func testHandler(t *testing.T) (*store.DB, http.Handler) {
	t.Helper()
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db, NewHandler(db, "http://192.168.10.24:8654/")
}

func assertRedirect(t *testing.T, handler http.Handler, method, path string, form url.Values, location string) {
	t.Helper()
	rec := request(t, handler, method, path, form, "")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("%s %s status = %d, want %d", method, path, rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != location {
		t.Fatalf("%s %s Location = %q, want %q", method, path, got, location)
	}
}

func assertStatus(t *testing.T, handler http.Handler, method, path string, form url.Values, status int) {
	t.Helper()
	rec := request(t, handler, method, path, form, "")
	if rec.Code != status {
		t.Fatalf("%s %s status = %d, want %d; body = %q", method, path, rec.Code, status, rec.Body.String())
	}
}

func request(t *testing.T, handler http.Handler, method, path string, form url.Values, host string) *httptest.ResponseRecorder {
	t.Helper()
	var body *strings.Reader
	if form == nil {
		body = strings.NewReader("")
	} else {
		body = strings.NewReader(form.Encode())
	}
	req := httptest.NewRequest(method, "http://hackbox.test"+path, body)
	if host != "" {
		req.Host = host
	}
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}
