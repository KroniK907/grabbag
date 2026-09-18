package quips

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestFactorySettingsMatchGMs(t *testing.T) {
	t.Parallel()
	s := factorySettings()
	if s.RoundCount != 3 || !s.LastQuipEnabled || s.RoundMultiplierIncreaseBy != 1 {
		t.Fatalf("match knobs: %#v", s)
	}
	if s.SeatedVotePoints != 100 || s.AudienceVotePoints != 10 {
		t.Fatalf("vote points: %#v", s)
	}
	if s.WriteSec != 75 || s.VoteSec != 30 || s.WinnerScreenSec != 5 || s.FinalScoresSec != 10 {
		t.Fatalf("timers: %#v", s)
	}
	if s.HostControlledReveals || s.AllowSelfVote || s.LiveVoteCounts || s.ShowMatchedWord {
		t.Fatalf("flags should default off: %#v", s)
	}
	if s.QuipCharCap != 120 {
		t.Fatalf("cap=%d", s.QuipCharCap)
	}
}

func TestSettingsDefaultsAndRangeRejects(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	h := newFakeHelper(dir)
	g := New()
	if err := g.Load(h); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = g.Shutdown() })

	page := settingsPage(t, g)
	for _, needle := range []string{
		`<h2>Match</h2>`, `<h2>Timers</h2>`, `<h2>Voting</h2>`, `<h2>Compose</h2>`, `<h2>Libraries</h2>`,
		`value="3"`, `value="100"`, `value="10"`, `value="75"`, `value="30"`, `value="120"`,
		"Prompt Library",
	} {
		if !strings.Contains(page, needle) {
			t.Fatalf("missing %q in %s", needle, page)
		}
	}

	bad := postSettings(g, "/round-count", url.Values{"round_count": {"0"}})
	if bad.Code != http.StatusOK || !strings.Contains(bad.Body.String(), "Round count is 1 to 20") {
		t.Fatalf("range = %d %s", bad.Code, bad.Body.String())
	}
	ok := postSettings(g, "/round-count", url.Values{"round_count": {"5"}})
	if ok.Code != http.StatusSeeOther {
		t.Fatalf("save = %d %s", ok.Code, ok.Body.String())
	}
	var stored matchSettings
	if err := json.Unmarshal(h.kv[kvMatchSettings], &stored); err != nil {
		t.Fatal(err)
	}
	if stored.RoundCount != 5 {
		t.Fatalf("kv roundCount=%d", stored.RoundCount)
	}
}

func TestGameSettingsUseHTMX(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	h := newFakeHelper(dir)
	g := New()
	if err := g.Load(h); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = g.Shutdown() })

	page := settingsPage(t, g)
	for _, want := range []string{
		`id="game-settings"`,
		`hx-post="/settings/game/round-count"`,
		`hx-target="#game-settings"`,
		`hx-swap="outerHTML"`,
	} {
		if !strings.Contains(page, want) {
			t.Fatalf("settings missing %s", want)
		}
	}
	if strings.Contains(page, `onchange="this.form.submit()"`) {
		t.Fatal("settings still submits with a full navigation")
	}

	rec := postSettingsHX(g, "/round-count", url.Values{"round_count": {"5"}})
	if rec.Code != http.StatusOK {
		t.Fatalf("htmx round-count = %d %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, "<!doctype html>") || strings.Contains(body, "<html") {
		t.Fatal("htmx round-count returned a full document")
	}
	if !strings.Contains(body, `id="game-settings"`) || !strings.Contains(body, `value="5"`) {
		t.Fatalf("htmx round-count missing settings body: %s", body)
	}
}

func TestSettingsFreezeAfterStart(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	h := newFakeHelper(dir)
	g := New()
	if err := g.Load(h); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = g.Shutdown() })

	g.mu.Lock()
	g.engine = &engine{Phase: phaseWrite}
	g.started = true
	g.mu.Unlock()

	frozen := postSettings(g, "/seated-vote-points", url.Values{"seated_vote_points": {"50"}})
	if frozen.Code != http.StatusOK || !strings.Contains(frozen.Body.String(), "This match is running. Setup is locked.") {
		t.Fatalf("freeze = %d %s", frozen.Code, frozen.Body.String())
	}
	if !strings.Contains(frozen.Body.String(), "disabled") {
		t.Fatal("controls should render disabled while frozen")
	}
}

func TestLoadSeedsMatchSettingsKV(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	h := newFakeHelper(dir)
	g := New()
	if err := g.Load(h); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = g.Shutdown() })

	raw, ok := h.kv[kvMatchSettings]
	if !ok {
		t.Fatal("Load did not seed match-settings")
	}
	s, parsed := parseMatchSettings(raw)
	if !parsed || s.RoundCount != 3 {
		t.Fatalf("parsed=%v settings=%#v", parsed, s)
	}
	if !s.packOn("quips", "comedy") {
		t.Fatalf("shipped pack should default on: %#v", s.Enabled)
	}
}

func settingsPage(t *testing.T, g *Game) string {
	t.Helper()
	rec := httptest.NewRecorder()
	g.Settings().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET settings = %d", rec.Code)
	}
	return rec.Body.String()
}

func postSettings(g *Game, path string, vals url.Values) *httptest.ResponseRecorder {
	return postSettingsHeader(g, path, vals, nil)
}

func postSettingsHX(g *Game, path string, vals url.Values) *httptest.ResponseRecorder {
	return postSettingsHeader(g, path, vals, http.Header{"HX-Request": {"true"}})
}

func postSettingsHeader(g *Game, path string, vals url.Values, extra http.Header) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(vals.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for k, vs := range extra {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	g.Settings().ServeHTTP(rec, req)
	return rec
}
