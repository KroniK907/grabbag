package host

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/KroniK907/hackbox/internal/games"
	"github.com/KroniK907/hackbox/internal/store"
)

func TestKeepReloadsSelectedGameAndClearLeavesKV(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	db, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	fake := &fakeGame{}
	handler, _, err := newHandler(db, "http://192.168.10.24:8654/", []games.Factory{{
		ID:  "fake",
		New: func() games.Game { return fake },
	}})
	if err != nil {
		t.Fatal(err)
	}
	setup := request(t, handler, http.MethodPost, "/setup", url.Values{
		"password": {"correct horse"},
		"confirm":  {"correct horse"},
	}, "")
	admin := setup.Result().Cookies()
	join := request(t, handler, http.MethodPost, "/lobby/join", url.Values{
		"display_name":   {"Ada"},
		"admin_password": {"correct horse"},
	}, "")
	host := join.Result().Cookies()
	load := requestWithCookie(t, handler, http.MethodPost, "/settings/load", url.Values{"game_id": {"fake"}}, host)
	if load.Code != http.StatusSeeOther {
		t.Fatalf("Load = %d %q", load.Code, load.Body.String())
	}
	if err := db.KVSet(context.Background(), "fake", "score", []byte("12")); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()

	db, err = store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	reloaded := &fakeGame{}
	handler, rt, err := newHandler(db, "http://192.168.10.24:8654/", []games.Factory{{
		ID:  "fake",
		New: func() games.Game { return reloaded },
	}})
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.loaded {
		t.Fatal("selected game loaded before Keep")
	}
	if !rt.room.RestorePending() {
		t.Fatal("restart with roster was not pending")
	}

	ok, err := db.HasAdminSession(context.Background(), admin[0].Value)
	if err != nil || !ok {
		t.Fatalf("operator session lost after restart ok=%v err=%v", ok, err)
	}
	keep := requestWithCookie(t, handler, http.MethodPost, "/settings/keep", nil, admin)
	if keep.Code != http.StatusSeeOther {
		t.Fatalf("Keep = %d %q", keep.Code, keep.Body.String())
	}
	if !reloaded.loaded {
		t.Fatal("Keep did not Load the selected game")
	}
	if reloaded.started {
		t.Fatal("Keep started the game")
	}

	start := requestWithCookie(t, handler, http.MethodPost, "/settings/start", nil, admin)
	if start.Code != http.StatusSeeOther {
		t.Fatalf("Start after Keep = %d", start.Code)
	}

	_ = db.Close()
	db, err = store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	clearedGame := &fakeGame{}
	handler, _, err = newHandler(db, "http://192.168.10.24:8654/", []games.Factory{{
		ID:  "fake",
		New: func() games.Game { return clearedGame },
	}})
	if err != nil {
		t.Fatal(err)
	}
	clear := requestWithCookie(t, handler, http.MethodPost, "/settings/clear-room", nil, admin)
	if clear.Code != http.StatusSeeOther {
		t.Fatalf("Clear = %d %q", clear.Code, clear.Body.String())
	}
	got, present, err := db.KVGet(context.Background(), "fake", "score")
	if err != nil || !present || string(got) != "12" {
		t.Fatalf("KV after night Clear = %q present=%v err=%v", got, present, err)
	}
	if !clearedGame.loaded {
		t.Fatal("Clear did not keep the selected game Loaded")
	}
	if clearedGame.started {
		t.Fatal("Clear restored a running Start")
	}
	board := requestWithCookie(t, handler, http.MethodGet, "/board", nil, nil).Body.String()
	if strings.Contains(board, "Ada") || strings.Contains(board, "Waiting for the host to decide to keep or clear this room") {
		t.Fatalf("cleared board = %q", board)
	}
}
