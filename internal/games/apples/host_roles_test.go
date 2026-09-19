package apples

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestSettingsHostRoleToggles(t *testing.T) {
	t.Parallel()
	h := newFakeHelper(t.TempDir(), true)
	g := loadedGame(t, h)
	page := settingsPage(t, g)
	if !strings.Contains(page, "Host always judges") || !strings.Contains(page, "Host reveals answers") {
		t.Fatalf("host settings missing: %s", page)
	}
}

func TestHostJudgeKeepsClaimedHost(t *testing.T) {
	t.Parallel()
	h, g := tinyGame(t, 6, 40, 1)
	postSettings(g, "/hand-size", url.Values{"hand_size": {"3"}})
	postSettings(g, "/voting", url.Values{"voting": {"off"}})
	postSettings(g, "/timer-submit", url.Values{"submit": {"0"}})
	postSettings(g, "/timer-judge-pick", url.Values{"judge_pick": {"0"}})
	postSettings(g, "/host-judge", url.Values{"enabled": {"1"}})
	h.sitHost("p1", "Host")
	h.sit("p2", "Sam")
	seedRNG(g)
	if err := g.Start(h); err != nil {
		t.Fatal(err)
	}
	if currentJudge(g) != "p1" {
		t.Fatalf("judge = %s, want p1", currentJudge(g))
	}
	runMiniRound(t, g, "p1", "p2")
	if currentJudge(g) != "p1" {
		t.Fatalf("after round judge = %s, want p1", currentJudge(g))
	}
}

func TestHostRevealsOnlyHostMayReveal(t *testing.T) {
	t.Parallel()
	h, g := tinyGame(t, 6, 40, 1)
	postSettings(g, "/hand-size", url.Values{"hand_size": {"3"}})
	postSettings(g, "/voting", url.Values{"voting": {"off"}})
	postSettings(g, "/timer-submit", url.Values{"submit": {"0"}})
	postSettings(g, "/timer-between", url.Values{"between": {"0"}})
	postSettings(g, "/host-reveals", url.Values{"enabled": {"1"}})
	h.sitHost("p1", "Host")
	h.sit("p2", "Sam")
	seedRNG(g)
	if err := g.Start(h); err != nil {
		t.Fatal(err)
	}
	judge := currentJudge(g)
	other := "p2"
	if judge == "p2" {
		other = "p1"
	}
	enterReveal(t, g, judge, other)
	if judge != "p1" {
		bad := playPOST(g, "/reveal", judge, nil)
		if !strings.Contains(bad.Body.String(), "Only the host can reveal") {
			t.Fatalf("judge reveal gate = %s", bad.Body.String())
		}
	}
	hostPhone := httptest.NewRecorder()
	hostReq := httptest.NewRequest(http.MethodGet, "/", nil)
	hostReq.Header.Set("X-Player", "p1")
	g.Phone(hostPhone, hostReq)
	if judge != "p1" && !strings.Contains(hostPhone.Body.String(), ">Reveal next<") {
		t.Fatalf("host phone missing reveal: %s", hostPhone.Body.String())
	}
	ok := playPOST(g, "/reveal", "p1", nil)
	if ok.Code != http.StatusOK {
		t.Fatalf("host reveal = %s", ok.Body.String())
	}
	if allRevealed(g) {
		return
	}
	g.mu.Lock()
	revealed := 0
	for _, p := range g.match.Packets {
		if p.Revealed {
			revealed++
		}
	}
	g.mu.Unlock()
	if revealed != 1 {
		t.Fatalf("revealed count = %d", revealed)
	}
}

func TestHostRevealsSkipsAutoBetweenTimer(t *testing.T) {
	t.Parallel()
	h, g := tinyGame(t, 6, 40, 1)
	postSettings(g, "/hand-size", url.Values{"hand_size": {"3"}})
	postSettings(g, "/voting", url.Values{"voting": {"off"}})
	postSettings(g, "/timer-submit", url.Values{"submit": {"0"}})
	postSettings(g, "/timer-between", url.Values{"between": {"0"}})
	postSettings(g, "/host-reveals", url.Values{"enabled": {"1"}})
	h.sitHost("p1", "Host")
	h.sit("p2", "Sam")
	seedRNG(g)
	if err := g.Start(h); err != nil {
		t.Fatal(err)
	}
	judge := currentJudge(g)
	other := "p2"
	if judge == "p2" {
		other = "p1"
	}
	enterReveal(t, g, judge, other)
	g.mu.Lock()
	revealed := 0
	for _, p := range g.match.Packets {
		if p.Revealed {
			revealed++
		}
	}
	g.mu.Unlock()
	if revealed != 0 {
		t.Fatalf("auto timer revealed %d packets, want 0", revealed)
	}
}

func enterReveal(t *testing.T, g *Game, judge, submitter string) {
	t.Helper()
	draw := playPOST(g, "/draw", judge, nil)
	if draw.Code != http.StatusOK {
		t.Fatal(draw.Body.String())
	}
	if keep := playPOST(g, "/keep-prompt", judge, nil); keep.Code != http.StatusOK {
		t.Fatal(keep.Body.String())
	}
	card := firstHandCard(g, submitter)
	if slot := playPOST(g, "/slot", submitter, url.Values{"card": {card}}); slot.Code != http.StatusOK {
		t.Fatal(slot.Body.String())
	}
	if lock := playPOST(g, "/lock", submitter, nil); lock.Code != http.StatusOK {
		t.Fatal(lock.Body.String())
	}
	if phaseOf(g) != phaseReveal {
		t.Fatalf("phase = %s", phaseOf(g))
	}
}

func runMiniRound(t *testing.T, g *Game, judge, submitter string) {
	t.Helper()
	enterReveal(t, g, judge, submitter)
	for phaseOf(g) == phaseReveal && !allRevealed(g) {
		playPOST(g, "/reveal", judge, nil)
	}
	winner := firstPacket(g)
	if conf := playPOST(g, "/confirm", judge, url.Values{"winner": {winner}}); conf.Code != http.StatusOK {
		t.Fatal(conf.Body.String())
	}
}
