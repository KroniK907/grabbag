package lobby_test

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/KroniK907/hackbox/internal/lobby"
	"github.com/KroniK907/hackbox/internal/ui"
)

func TestNeonCabinetBoardAndPhones(t *testing.T) {
	t.Parallel()
	_, handler, room := testLobby(t)

	join := lobbyRequest(t, handler, http.MethodGet, "/", nil, nil)
	body := join.Body.String()
	if !strings.Contains(body, `data-theme="neon-light"`) {
		t.Fatalf("join default theme = %q", body)
	}
	if !strings.Contains(body, "Reroll face") || !strings.Contains(body, `name="avatar_seed"`) {
		t.Fatalf("join missing avatar reroll: %q", body)
	}
	seed := hiddenValue(t, body, "avatar_seed")
	if seed == "" {
		t.Fatal("join did not mint an avatar seed")
	}

	reroll := lobbyRequest(
		t,
		handler,
		http.MethodPost,
		"/lobby/reroll",
		url.Values{"display_name": {"Maya"}, "avatar_seed": {seed}},
		nil,
	)
	if reroll.Code != http.StatusSeeOther {
		t.Fatalf("reroll status = %d, want %d; body = %q", reroll.Code, http.StatusSeeOther, reroll.Body.String())
	}
	rerollCookie := cookieNamed(t, reroll, lobby.PlayerCookieName)
	after := lobbyRequest(t, handler, http.MethodGet, "/", nil, rerollCookie)
	newSeed := hiddenValue(t, after.Body.String(), "avatar_seed")
	if newSeed == "" || newSeed == seed {
		t.Fatalf("reroll seed = %q, old = %q", newSeed, seed)
	}

	joined := lobbyRequest(
		t,
		handler,
		http.MethodPost,
		"/lobby/join",
		url.Values{
			"display_name":   {"Maya"},
			"admin_password": {"correct horse"},
			"avatar_seed":    {newSeed},
		},
		rerollCookie,
	)
	if joined.Code != http.StatusSeeOther {
		t.Fatalf("Join status = %d; body = %q", joined.Code, joined.Body.String())
	}
	player := playerFromCookie(t, room, cookieNamed(t, joined, lobby.PlayerCookieName))
	if player.AvatarSeed != newSeed {
		t.Fatalf("stored seed = %q, want %q", player.AvatarSeed, newSeed)
	}

	board := lobbyRequest(t, handler, http.MethodGet, "/board", nil, nil).Body.String()
	if !strings.Contains(board, `data-theme="neon-light"`) ||
		!strings.Contains(board, `class="ui-rail"`) ||
		!strings.Contains(board, `class="ui-cluster"`) ||
		!strings.Contains(board, `class="ui-marquee"`) ||
		!strings.Contains(board, "CLOSED") ||
		strings.Contains(board, "empty chair") ||
		strings.Contains(board, "<h2>Audience</h2>") {
		t.Fatalf("board chrome = %q", board)
	}
	if !strings.Contains(board, string(ui.AvatarSVG(newSeed))) {
		t.Fatal("board token did not render the stored avatar seed")
	}
	if !strings.Contains(board, `class="ui-qr"`) || !strings.Contains(board, "<svg") {
		t.Fatalf("board missing join QR: %q", board)
	}

	seated := lobbyRequest(t, handler, http.MethodGet, "/", nil, cookieNamed(t, joined, lobby.PlayerCookieName)).Body.String()
	if !strings.Contains(seated, `class="ui-plunger"`) || strings.Contains(seated, `action="/lobby/ready"`) {
		t.Fatalf("seated phone = %q", seated)
	}

	guest := joinNamed(t, handler, "Theo", "")
	waiting := lobbyRequest(t, handler, http.MethodGet, "/", nil, guest).Body.String()
	if strings.Contains(waiting, "ui-plunger") || strings.Contains(waiting, "READY") {
		t.Fatalf("waiting phone included Ready: %q", waiting)
	}
}

func hiddenValue(t *testing.T, body, name string) string {
	t.Helper()
	re := regexp.MustCompile(`name="` + regexp.QuoteMeta(name) + `" value="([^"]*)"`)
	match := re.FindStringSubmatch(body)
	if len(match) != 2 {
		t.Fatalf("missing hidden %s in %q", name, body)
	}
	return match[1]
}
