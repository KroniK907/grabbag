package lobby_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/KroniK907/grabbag/internal/lobby"
)

func TestAudioLevelsDefaultAndSave(t *testing.T) {
	t.Parallel()
	_, handler, room := testLobby(t)
	ctx := context.Background()

	levels, err := room.AudioLevels(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if levels != lobby.DefaultAudioLevels {
		t.Fatalf("fresh levels = %+v, want %+v", levels, lobby.DefaultAudioLevels)
	}

	if rec := lobbyRequest(t, handler, http.MethodPost, "/settings/audio", url.Values{"layer": {"music"}, "volume": {"10"}}, nil); rec.Code == http.StatusSeeOther {
		t.Fatal("a signed-out POST changed the mixer")
	}

	for _, form := range []url.Values{
		{"layer": {"music"}, "volume": {"30"}},
		{"layer": {"effects"}, "muted": {"1"}},
		{"layer": {"master"}, "volume": {"100"}, "muted": {"0"}},
	} {
		if rec := lobbyRequest(t, handler, http.MethodPost, "/settings/audio", form, operatorCookie()); rec.Code != http.StatusSeeOther {
			t.Fatalf("POST %v = %d %q", form, rec.Code, rec.Body.String())
		}
	}
	levels, err = room.AudioLevels(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := lobby.AudioLevels{
		Master:  lobby.AudioLevel{Volume: 100},
		Music:   lobby.AudioLevel{Volume: 30},
		Effects: lobby.AudioLevel{Volume: 75, Muted: true},
	}
	if levels != want {
		t.Fatalf("levels = %+v, want %+v", levels, want)
	}
	if got := room.AudioLevelsJSON(ctx); !strings.Contains(got, `"music":{"volume":30,"muted":false}`) {
		t.Fatalf("levels JSON = %s", got)
	}

	for _, form := range []url.Values{
		{"layer": {"voice"}, "volume": {"30"}},
		{"layer": {"music"}, "volume": {"101"}},
		{"layer": {"music"}, "volume": {"-1"}},
		{"layer": {"music"}},
	} {
		if rec := lobbyRequest(t, handler, http.MethodPost, "/settings/audio", form, operatorCookie()); rec.Code != http.StatusBadRequest {
			t.Fatalf("POST %v = %d, want 400", form, rec.Code)
		}
	}
}

// A slider posts without a swap target and gets 204. Mute targets the
// Audio group and gets it back.
func TestAudioWriteResponses(t *testing.T) {
	t.Parallel()
	_, handler, _ := testLobby(t)
	post := func(form url.Values, target string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/settings/audio", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("HX-Request", "true")
		if target != "" {
			req.Header.Set("HX-Target", target)
		}
		req.AddCookie(operatorCookie())
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}
	if rec := post(url.Values{"layer": {"music"}, "volume": {"42"}}, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("slider = %d, want 204", rec.Code)
	}
	rec := post(url.Values{"layer": {"music"}, "muted": {"1"}}, "settings-audio")
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `id="settings-audio"`) || !strings.Contains(body, "Unmute Music") || !strings.Contains(body, `value="42"`) {
		t.Fatalf("mute = %d %q", rec.Code, body)
	}
}

func TestAudioTestNeedsAdminAndALayer(t *testing.T) {
	t.Parallel()
	_, handler, _ := testLobby(t)
	if rec := lobbyRequest(t, handler, http.MethodPost, "/settings/audio/test", url.Values{"layer": {"music"}}, nil); rec.Code == http.StatusSeeOther {
		t.Fatal("a signed-out test played")
	}
	if rec := lobbyRequest(t, handler, http.MethodPost, "/settings/audio/test", url.Values{"layer": {"master"}}, operatorCookie()); rec.Code != http.StatusBadRequest {
		t.Fatalf("master test = %d, want 400", rec.Code)
	}
	if rec := lobbyRequest(t, handler, http.MethodPost, "/settings/audio/test", url.Values{"layer": {"effects"}}, operatorCookie()); rec.Code != http.StatusSeeOther {
		t.Fatalf("effects test = %d, want 303", rec.Code)
	}
}

// The TV is usually signed out, so its report needs no session. Only the
// three states are kept, and the last report wins.
func TestBoardAudioStatus(t *testing.T) {
	t.Parallel()
	_, handler, room := testLobby(t)
	if room.AudioStatus() != "" {
		t.Fatalf("status before any board = %q", room.AudioStatus())
	}
	status := func() string {
		return lobbyRequest(t, handler, http.MethodGet, "/settings/audio/status", nil, operatorCookie()).Body.String()
	}
	if !strings.Contains(status(), "No board open") {
		t.Fatalf("status partial = %q", status())
	}
	for _, state := range []string{lobby.AudioWaiting, lobby.AudioUnlocked} {
		if rec := lobbyRequest(t, handler, http.MethodPost, "/board/audio", url.Values{"state": {state}}, nil); rec.Code != http.StatusNoContent {
			t.Fatalf("report %s = %d", state, rec.Code)
		}
	}
	if room.AudioStatus() != lobby.AudioUnlocked || !strings.Contains(status(), "<strong>On</strong>") {
		t.Fatalf("status = %q, partial %q", room.AudioStatus(), status())
	}
	if rec := lobbyRequest(t, handler, http.MethodPost, "/board/audio", url.Values{"state": {"loud"}}, nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown state = %d, want 400", rec.Code)
	}
	if room.AudioStatus() != lobby.AudioUnlocked {
		t.Fatalf("an unknown state changed the status to %q", room.AudioStatus())
	}
	if rec := lobbyRequest(t, handler, http.MethodGet, "/settings/audio/status", nil, nil); strings.Contains(rec.Body.String(), "Board sound") {
		t.Fatal("a signed-out request read the status")
	}
}
