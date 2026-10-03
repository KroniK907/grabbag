package runtimekit_test

import (
	"errors"
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/KroniK907/grabbag/internal/games"
	"github.com/KroniK907/grabbag/internal/games/runtimekit"
)

type counter struct {
	N      int
	Timer  runtimekit.Timer
	Paused bool
}

type fakeHelper struct {
	mu        sync.Mutex
	run       *runtimekit.Runner[*counter]
	published []string
	finished  bool
	paused    bool
}

func (h *fakeHelper) Seated() []games.Player   { return nil }
func (h *fakeHelper) Waiting() []games.Player  { return nil }
func (h *fakeHelper) Audience() []games.Player { return nil }
func (h *fakeHelper) Player(id string) (games.Player, bool) {
	return games.Player{ID: id, ClaimedHost: id == "host"}, id != ""
}
func (h *fakeHelper) PlayerFromRequest(r *http.Request) (games.Player, bool, error) {
	p, ok := h.Player(r.Header.Get("X-Player"))
	return p, ok, nil
}
func (h *fakeHelper) DataDir() string                    { return "" }
func (h *fakeHelper) KVGet(string) ([]byte, bool, error) { return nil, false, nil }
func (h *fakeHelper) KVSet(string, []byte) error         { return nil }
func (h *fakeHelper) Notify(string, string, string, int) {}
func (h *fakeHelper) Log(string)                         {}
func (h *fakeHelper) Theme() string                      { return "neon-light" }
func (h *fakeHelper) HasAdmin(r *http.Request) bool      { return r.Header.Get("X-Admin") == "1" }
func (h *fakeHelper) Finish()                            { h.set(func() { h.finished = true }); _ = h.run.Stop() }
func (h *fakeHelper) Pause()                             { h.set(func() { h.paused = true }); _ = h.run.Pause() }
func (h *fakeHelper) Resume()                            { h.set(func() { h.paused = false }); _ = h.run.Resume() }
func (h *fakeHelper) Publish(name string)                { h.set(func() { h.published = append(h.published, name) }) }
func (h *fakeHelper) set(fn func())                      { h.mu.Lock(); fn(); h.mu.Unlock() }
func (h *fakeHelper) events() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.published...)
}
func (h *fakeHelper) state() (finished, paused bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.finished, h.paused
}

var pages = template.Must(template.New("phone").Parse(`{{if .Gone}}gone{{else}}n={{.N}}{{end}} msg={{.Msg}}`))

type phone struct {
	Gone bool
	N    int
	Msg  string
}

func newRunner(t *testing.T, cfg runtimekit.Config[*counter]) (*runtimekit.Runner[*counter], *fakeHelper) {
	t.Helper()
	cfg.Game = "Counter"
	cfg.Event = "counter"
	cfg.Pages = pages
	cfg.Reply = func(e *counter, _ games.Player, _ *http.Request, msg string) (string, any) {
		if e == nil {
			return "phone", phone{Gone: true, Msg: msg}
		}
		return "phone", phone{N: e.N, Msg: msg}
	}
	run := runtimekit.New(cfg)
	h := &fakeHelper{run: run}
	run.Load(h)
	if err := run.Start(h, func(games.Helper) (*counter, error) { return &counter{}, nil }); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = run.Shutdown() })
	return run, h
}

func do(run *runtimekit.Runner[*counter], player string, admin bool, fn runtimekit.Action[*counter]) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Player", player)
	if admin {
		req.Header.Set("X-Admin", "1")
	}
	rec := httptest.NewRecorder()
	if player == "host" || admin {
		run.HostAct(rec, req, fn)
	} else {
		run.Act(rec, req, fn)
	}
	return rec
}

func bump(e *counter, _ games.Player, _ time.Time) runtimekit.Result {
	e.N++
	return runtimekit.Result{Changed: true, Events: []string{"counter-extra"}}
}

func TestActPublishesAndReplies(t *testing.T) {
	run, h := newRunner(t, runtimekit.Config[*counter]{})
	rec := do(run, "p1", false, bump)
	if rec.Code != http.StatusOK || rec.Body.String() != "n=1 msg=" {
		t.Fatalf("%d %q", rec.Code, rec.Body.String())
	}
	if got := strings.Join(h.events(), ","); got != "counter,counter-extra" {
		t.Fatalf("published %q", got)
	}
}

func TestActErrorRepliesWithoutPublish(t *testing.T) {
	run, h := newRunner(t, runtimekit.Config[*counter]{})
	rec := do(run, "p1", false, func(*counter, games.Player, time.Time) runtimekit.Result {
		return runtimekit.Result{Err: "Not now."}
	})
	if rec.Body.String() != "n=0 msg=Not now." || len(h.events()) != 0 {
		t.Fatalf("%q published %v", rec.Body.String(), h.events())
	}
}

func TestActNeedsSignInAndRunningMatch(t *testing.T) {
	run, _ := newRunner(t, runtimekit.Config[*counter]{})
	if rec := do(run, "", false, bump); rec.Code != http.StatusUnauthorized {
		t.Fatalf("signed out: %d", rec.Code)
	}
	_ = run.Stop()
	if rec := do(run, "p1", false, bump); rec.Code != http.StatusNotFound {
		t.Fatalf("stopped: %d", rec.Code)
	}
}

func TestHostActIsHostOrAdminOnly(t *testing.T) {
	run, _ := newRunner(t, runtimekit.Config[*counter]{})
	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	req.Header.Set("X-Player", "p1")
	rec := httptest.NewRecorder()
	run.HostAct(rec, req, bump)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("player: %d", rec.Code)
	}
	if rec := do(run, "host", false, bump); rec.Body.String() != "n=1 msg=" {
		t.Fatalf("host: %q", rec.Body.String())
	}
	if rec := do(run, "", true, bump); rec.Body.String() != "n=2 msg=" {
		t.Fatalf("admin: %q", rec.Body.String())
	}
}

// Finish calls host, and host calls Stop on the game. That only works if the
// Runner released the lock first.
func TestFinishReleasesLockBeforeHost(t *testing.T) {
	run, h := newRunner(t, runtimekit.Config[*counter]{})
	done := make(chan *httptest.ResponseRecorder)
	go func() {
		done <- do(run, "p1", false, func(*counter, games.Player, time.Time) runtimekit.Result {
			return runtimekit.Result{Finish: true}
		})
	}()
	select {
	case rec := <-done:
		if rec.Body.String() != "gone msg=" {
			t.Fatalf("reply after finish %q", rec.Body.String())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Finish deadlocked")
	}
	if finished, _ := h.state(); !finished {
		t.Fatal("host Finish not called")
	}
}

func TestGateAndHostPauseResume(t *testing.T) {
	var holds []bool
	run, h := newRunner(t, runtimekit.Config[*counter]{
		Gate: func(_ *counter, _ games.Player, paused bool) string {
			if paused {
				return "Paused."
			}
			return ""
		},
		Hold: func(_ *counter, paused bool, _ time.Time) { holds = append(holds, paused) },
	})
	req := func(fn func(http.ResponseWriter, *http.Request)) string {
		r := httptest.NewRequest(http.MethodPost, "/x", nil)
		r.Header.Set("X-Player", "host")
		rec := httptest.NewRecorder()
		fn(rec, r)
		return rec.Body.String()
	}
	req(run.HostPause)
	if _, paused := h.state(); !paused {
		t.Fatal("HostPause did not pause host")
	}
	if got := do(run, "p1", false, bump).Body.String(); got != "n=0 msg=Paused." {
		t.Fatalf("gated act %q", got)
	}
	req(run.HostResume)
	if _, paused := h.state(); paused {
		t.Fatal("HostResume did not resume host")
	}
	if got := do(run, "p1", false, bump).Body.String(); got != "n=1 msg=" {
		t.Fatalf("act after resume %q", got)
	}
	if len(holds) != 2 || !holds[0] || holds[1] {
		t.Fatalf("holds %v", holds)
	}
}

func TestTickAdvancesUntilPausedOrStopped(t *testing.T) {
	var mu sync.Mutex
	clock := t0
	now := func() time.Time { mu.Lock(); defer mu.Unlock(); return clock }
	setNow := func(at time.Time) { mu.Lock(); clock = at; mu.Unlock() }
	run, h := newRunner(t, runtimekit.Config[*counter]{
		Tick: time.Millisecond,
		Advance: func(e *counter, at time.Time) runtimekit.Result {
			if !e.Timer.Expired(at) {
				return runtimekit.Result{}
			}
			e.Timer.Clear()
			e.N++
			return runtimekit.Result{Changed: true}
		},
		Hold: func(e *counter, paused bool, at time.Time) {
			if paused {
				e.Timer.Freeze(at)
			} else {
				e.Timer.Thaw(at)
			}
		},
	})
	run.SetClock(now)
	run.Lock()
	run.Engine().Timer.Arm("", time.Second, t0, false)
	run.Unlock()

	_ = run.Pause()
	setNow(t0.Add(time.Hour))
	time.Sleep(20 * time.Millisecond)
	if n := len(h.events()); n != 0 {
		t.Fatalf("paused tick published %d", n)
	}
	_ = run.Resume()
	setNow(t0.Add(time.Hour + time.Second))
	waitFor(t, func() bool { return len(h.events()) == 1 })

	run.Lock()
	run.Engine().Timer.Arm("", time.Second, now(), false)
	run.Unlock()
	_ = run.Stop()
	setNow(t0.Add(2 * time.Hour))
	time.Sleep(20 * time.Millisecond)
	if n := len(h.events()); n != 1 {
		t.Fatalf("stopped tick published, %d events", n)
	}
}

func TestStartErrorLeavesNoMatch(t *testing.T) {
	run := runtimekit.New(runtimekit.Config[*counter]{})
	h := &fakeHelper{run: run}
	run.Load(h)
	err := run.Start(h, func(games.Helper) (*counter, error) { return nil, errors.New("no") })
	run.Lock()
	defer run.Unlock()
	if err == nil || run.Running() {
		t.Fatalf("err=%v running=%v", err, run.Running())
	}
}

func TestPageStampsThemeAndAssets(t *testing.T) {
	run, _ := newRunner(t, runtimekit.Config[*counter]{AssetVersion: "v9"})
	page := run.Page("Counter")
	if page.Theme != "neon-light" || page.GameCSS != "/play/static/game.css?v=v9" || page.GameJS != "/play/static/game.js?v=v9" {
		t.Fatalf("%+v", page)
	}
}

func waitFor(t *testing.T, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatal("timed out")
		}
		time.Sleep(time.Millisecond)
	}
}

// Defer runs where the lock cannot be released, such as a view. Its publish
// goes out at once and its host call waits for the next Apply.
func TestDeferQueuesHostCallsUntilApply(t *testing.T) {
	run, h := newRunner(t, runtimekit.Config[*counter]{})
	run.Lock()
	run.Defer(runtimekit.Result{Changed: true, Pause: true})
	run.Unlock()
	if _, paused := h.state(); paused {
		t.Fatal("Defer called host under the lock")
	}
	if got := strings.Join(h.events(), ","); got != "counter" {
		t.Fatalf("deferred publish %q", got)
	}
	run.Tick()
	if _, paused := h.state(); !paused {
		t.Fatal("next tick did not make the deferred Pause call")
	}
}

func TestTickFiresAdvanceOnce(t *testing.T) {
	calls := 0
	run, h := newRunner(t, runtimekit.Config[*counter]{
		Tick: time.Hour,
		Advance: func(e *counter, _ time.Time) runtimekit.Result {
			calls++
			return runtimekit.Result{Finish: true}
		},
	})
	run.Tick()
	if finished, _ := h.state(); calls != 1 || !finished {
		t.Fatalf("calls=%d finished=%v", calls, finished)
	}
	run.Tick()
	if calls != 1 {
		t.Fatalf("tick after Finish advanced a stopped match, calls=%d", calls)
	}
}
