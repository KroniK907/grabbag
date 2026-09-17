package host

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestNotifyPublishesNoticeJSON(t *testing.T) {
	t.Parallel()
	_, handler, _, fake := testGameHandler(t, 0, 0)
	admin := finishAndJoinHost(t, handler)
	requestWithCookie(t, handler, http.MethodPost, "/settings/load", url.Values{"game_id": {"fake"}}, admin)

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

	fake.helper.Notify("seated", "warning", "Deck is thin.", 5)
	frame := readHostSSEFrame(t, events, "event: notice")
	if !strings.Contains(frame, `"target":"seated"`) ||
		!strings.Contains(frame, `"type":"warning"`) ||
		!strings.Contains(frame, `"message":"Deck is thin."`) ||
		!strings.Contains(frame, `"duration":5`) {
		t.Fatalf("notice frame = %q", frame)
	}
}

func TestStartAndLoadFailureStayOnPageAndToastHost(t *testing.T) {
	t.Parallel()
	_, handler, _, _ := testGameHandler(t, 2, 8)
	admin := cookieAfterSetup(t, handler)

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

	start := requestWithCookie(t, handler, http.MethodPost, "/settings/start", nil, admin)
	if start.Code != http.StatusSeeOther {
		t.Fatalf("Start before Load = %d %q", start.Code, start.Body.String())
	}
	if got := start.Header().Get("Location"); got != "/settings" {
		t.Fatalf("Start Location = %q", got)
	}
	if !noticeCookieHas(t, start, "host", "Start needs a loaded game and at least one seated player.") {
		t.Fatalf("Start missing notice cookie: %v", start.Result().Cookies())
	}
	frame := readHostSSEFrame(t, events, "event: notice")
	if !strings.Contains(frame, `"target":"host"`) || !strings.Contains(frame, `"type":"error"`) {
		t.Fatalf("Start notice = %q", frame)
	}

	join := requestWithCookie(t, handler, http.MethodPost, "/lobby/join", url.Values{
		"display_name":   {"Ada"},
		"admin_password": {"correct horse"},
	}, admin)
	host := join.Result().Cookies()
	load := requestWithCookie(t, handler, http.MethodPost, "/settings/load", url.Values{"game_id": {"missing"}}, host)
	if load.Code != http.StatusSeeOther {
		t.Fatalf("bad Load = %d %q", load.Code, load.Body.String())
	}
	if !noticeCookieHas(t, load, "host", "Could not load that game.") {
		t.Fatalf("Load missing notice cookie: %v", load.Result().Cookies())
	}

	settings := requestWithCookie(t, handler, http.MethodGet, "/settings", nil, admin).Body.String()
	if !strings.Contains(settings, `id="notice-root"`) ||
		!strings.Contains(settings, `id="notice-targets"`) ||
		!strings.Contains(settings, `data-notice-targets="host"`) {
		t.Fatalf("settings chrome missing notice: %q", settings)
	}
}

func TestLoadRefusedAboveMaxToasts(t *testing.T) {
	t.Parallel()
	_, handler, _, _ := testGameHandler(t, 0, 1)
	admin := finishAndJoinHost(t, handler)
	requestWithCookie(t, handler, http.MethodPost, "/settings/open", nil, admin)
	requestWithCookie(t, handler, http.MethodPost, "/lobby/join", url.Values{"display_name": {"Bea"}}, nil)

	load := requestWithCookie(t, handler, http.MethodPost, "/settings/load", url.Values{"game_id": {"fake"}}, admin)
	if load.Code != http.StatusSeeOther {
		t.Fatalf("Load above max = %d %q", load.Code, load.Body.String())
	}
	if !noticeCookieHas(t, load, "host", "Could not load that game.") {
		t.Fatalf("Load above max missing toast cookie")
	}
}

func noticeCookieHas(t *testing.T, rec *httptest.ResponseRecorder, target, message string) bool {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name != noticeCookieName {
			continue
		}
		raw, err := url.QueryUnescape(c.Value)
		if err != nil {
			t.Fatal(err)
		}
		var payload noticePayload
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			t.Fatal(err)
		}
		return payload.Target == target && payload.Type == "error" && payload.Message == message
	}
	return false
}

func readHostSSEFrame(t *testing.T, reader *bufio.Reader, want string) string {
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
			return event.String()
		}
	}
	t.Fatalf("timed out waiting for %q", want)
	return ""
}
