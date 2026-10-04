package shelltest_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"

	"github.com/KroniK907/grabbag/internal/games"
	"github.com/KroniK907/grabbag/internal/host"
	"github.com/KroniK907/grabbag/internal/store"
)

const password = "shelltest-pass"

// testModeJS runs before any page script. It turns on the shell's records
// and the leak counter.
const testModeJS = `window.grabbagShellTest = true;`

func chromeBinary() string {
	for _, name := range []string{"chromium-browser", "chromium", "google-chrome", "google-chrome-stable", "headless_shell"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return ""
}

// room is one running host. Each tab is its own headless browser.
type room struct {
	t       *testing.T
	server  *httptest.Server
	db      *store.DB
	handler atomic.Pointer[http.Handler]
	// intercept, when set, answers a request instead of the host. It
	// returns false to pass the request on.
	intercept atomic.Pointer[func(http.ResponseWriter, *http.Request) bool]
	operator  []*http.Cookie
	chrome    string
}

// restart swaps in a fresh host handler on the same store and port, as a
// process restart would, and drops every open connection.
func (r *room) restart() {
	r.t.Helper()
	handler, err := host.NewHandler(r.db, "")
	if err != nil {
		r.t.Fatal(err)
	}
	r.handler.Store(&handler)
	r.server.CloseClientConnections()
}

// tab is one page in its own browser process, so each phone has its own
// cookies.
type tab struct {
	r    *room
	name string
	ctx  context.Context
}

func newRoom(t *testing.T) *room {
	t.Helper()
	bin := chromeBinary()
	if bin == "" {
		t.Skip("no Chromium found")
	}
	if testing.Short() {
		t.Skip("browser suite skipped in -short")
	}
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	handler, err := host.NewHandler(db, "")
	if err != nil {
		t.Fatal(err)
	}
	r := &room{t: t, db: db, chrome: bin}
	r.handler.Store(&handler)
	r.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if f := r.intercept.Load(); f != nil && (*f)(w, req) {
			return
		}
		(*r.handler.Load()).ServeHTTP(w, req)
	}))
	t.Cleanup(r.server.Close)

	setup := r.post("/setup", url.Values{"password": {password}, "confirm": {password}}, false)
	r.operator = setup.Cookies()
	if len(r.operator) == 0 {
		t.Fatalf("setup set no admin cookie: %d", setup.StatusCode)
	}
	r.post("/settings/open", nil, false)
	return r
}

// post sends a form as the operator. hx marks it as an htmx request.
func (r *room) post(path string, form url.Values, hx bool) *http.Response {
	r.t.Helper()
	if form == nil {
		form = url.Values{}
	}
	req, err := http.NewRequest(http.MethodPost, r.server.URL+path, strings.NewReader(form.Encode()))
	if err != nil {
		r.t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if hx {
		req.Header.Set("HX-Request", "true")
	}
	for _, c := range r.operator {
		req.AddCookie(c)
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		r.t.Fatal(err)
	}
	_ = resp.Body.Close()
	return resp
}

// open loads path in a fresh browser with test mode and reduced motion, so
// transitions are instant swaps.
func (r *room) open(name, path string) *tab {
	r.t.Helper()
	return r.openURL(name, r.server.URL+path)
}

// openSized is open in a browser window of w by h CSS pixels.
func (r *room) openSized(name, path string, w, h int64) *tab {
	r.t.Helper()
	return r.openURL(name, r.server.URL+path, chromedp.EmulateViewport(w, h))
}

// openURL is open for a full URL. before runs ahead of the navigation.
func (r *room) openURL(name, target string, before ...chromedp.Action) *tab {
	r.t.Helper()
	opts := append(chromedp.DefaultExecAllocatorOptions[:], chromedp.ExecPath(r.chrome))
	if os.Getenv("CI") != "" {
		// GitHub's Ubuntu runners block the user namespaces Chromium's sandbox
		// needs. The test browser only loads this test's own local server.
		opts = append(opts, chromedp.NoSandbox)
	}
	alloc, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
	r.t.Cleanup(cancelAlloc)
	ctx, cancel := chromedp.NewContext(alloc)
	r.t.Cleanup(cancel)
	tb := &tab{r: r, name: name, ctx: ctx}
	actions := append([]chromedp.Action{
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, err := page.AddScriptToEvaluateOnNewDocument(testModeJS).Do(ctx)
			return err
		}),
		emulation.SetEmulatedMedia().WithFeatures([]*emulation.MediaFeature{{Name: "prefers-reduced-motion", Value: "reduce"}}),
	}, before...)
	err := chromedp.Run(ctx, append(actions, chromedp.Navigate(target))...)
	if err != nil {
		r.t.Fatalf("%s: open %s: %v", name, target, err)
	}
	tb.waitFor(`window.grabbagShell && grabbagShell.current() !== null`)
	tb.run(`window.__shellMark = "first-load"`)
	return tb
}

func (tb *tab) run(js string) {
	tb.r.t.Helper()
	if err := chromedp.Run(tb.ctx, chromedp.Evaluate(js, nil)); err != nil {
		tb.r.t.Fatalf("%s: %s: %v", tb.name, js, err)
	}
}

func (tb *tab) eval(js string, out any) {
	tb.r.t.Helper()
	if err := chromedp.Run(tb.ctx, chromedp.Evaluate(js, out)); err != nil {
		tb.r.t.Fatalf("%s: %s: %v", tb.name, js, err)
	}
}

func (tb *tab) waitFor(js string) {
	tb.r.t.Helper()
	ctx, cancel := context.WithTimeout(tb.ctx, 15*time.Second)
	defer cancel()
	var ok bool
	if err := chromedp.Run(ctx, chromedp.Poll(js, &ok, chromedp.WithPollingInterval(50*time.Millisecond))); err != nil {
		var state string
		_ = chromedp.Run(tb.ctx, chromedp.Evaluate(`(async () => JSON.stringify({
			cur: window.grabbagShell && grabbagShell.current(),
			errors: window.grabbagShell && grabbagShell.record && grabbagShell.record.errors,
			mounts: window.grabbagShell && grabbagShell.record && grabbagShell.record.mounts,
			frame: await fetch(location.pathname === "/board" ? "/board/tenant" : "/tenant").then((r) => r.json()).then((f) => ({tenant: f.tenant, generation: f.generation, player: f.player})),
		}))()`, &state, func(p *runtime.EvaluateParams) *runtime.EvaluateParams { return p.WithAwaitPromise(true) }))
		tb.r.t.Fatalf("%s: waiting for %s: %v (state %s)", tb.name, js, err, state)
	}
}

// submit submits the first form matching selector the way a tap would, so
// htmx handles it.
func (tb *tab) submit(selector string) {
	tb.r.t.Helper()
	tb.waitFor(fmt.Sprintf(`!!document.querySelector(%q)`, selector))
	tb.run(fmt.Sprintf(`document.querySelector(%q).requestSubmit()`, selector))
}

func (tb *tab) tenant() string {
	var id string
	tb.eval(`grabbagShell.current().tenant`, &id)
	return id
}

func (tb *tab) waitTenant(id string) {
	tb.r.t.Helper()
	tb.waitFor(fmt.Sprintf(`grabbagShell.current().tenant === %q && document.getElementById("shell-stage").getAttribute("data-tenant") === %q`, id, id))
}

// health is what a tab reports about itself.
type health struct {
	Mark    string            `json:"mark"`
	Loads   int               `json:"loads"`
	Reloads []string          `json:"reloads"`
	Leaks   []json.RawMessage `json:"leaks"`
	Errors  []json.RawMessage `json:"errors"`
	Mounts  []mount           `json:"mounts"`
	Events  []string          `json:"events"`
}

type mount struct {
	Tenant     string `json:"tenant"`
	Generation int    `json:"generation"`
	Surface    string `json:"surface"`
}

func (tb *tab) health() health {
	tb.r.t.Helper()
	var raw string
	tb.eval(`JSON.stringify({
		mark: window.__shellMark || "",
		loads: Number(sessionStorage.getItem("grabbagShellLoads") || "0"),
		reloads: JSON.parse(sessionStorage.getItem("grabbagShellReloads") || "[]"),
		leaks: grabbagShell.record.leaks,
		errors: grabbagShell.record.errors,
		mounts: grabbagShell.record.mounts,
		events: grabbagShell.record.events,
	})`, &raw)
	var h health
	if err := json.Unmarshal([]byte(raw), &h); err != nil {
		tb.r.t.Fatal(err)
	}
	return h
}

// assertSteady fails when the tab navigated or reloaded, leaked, or logged
// a shell error.
func (tb *tab) assertSteady() {
	tb.r.t.Helper()
	h := tb.health()
	if h.Mark != "first-load" || h.Loads != 1 || len(h.Reloads) != 0 {
		tb.r.t.Fatalf("%s reloaded: mark=%q loads=%d reloads=%v", tb.name, h.Mark, h.Loads, h.Reloads)
	}
	if len(h.Leaks) != 0 {
		tb.r.t.Fatalf("%s leaked after unmount: %s", tb.name, joinRaw(h.Leaks))
	}
	if len(h.Errors) != 0 {
		tb.r.t.Fatalf("%s shell errors: %s", tb.name, joinRaw(h.Errors))
	}
}

func joinRaw(list []json.RawMessage) string {
	parts := make([]string, len(list))
	for i, m := range list {
		parts[i] = string(m)
	}
	return strings.Join(parts, "\n")
}

// joinPhone opens a phone and joins through the Join form.
func (r *room) joinPhone(name, pass string) *tab {
	r.t.Helper()
	return r.join(r.open(name, "/"), name, pass)
}

// join fills and sends the Join form on tb.
func (r *room) join(tb *tab, name, pass string) *tab {
	r.t.Helper()
	tb.waitFor(`!!document.querySelector('form[hx-post="/lobby/join"]')`)
	fill := fmt.Sprintf(`(() => {
		const f = document.querySelector('form[hx-post="/lobby/join"]');
		f.querySelector('[name=display_name]').value = %q;
		const p = f.querySelector('[name=admin_password]');
		if (p) p.value = %q;
		f.requestSubmit();
	})()`, name, pass)
	tb.run(fill)
	tb.waitFor(`document.getElementById("shell-stage").hasAttribute("data-player") && !!document.querySelector(".ui-you")`)
	return tb
}

// table is a board and four seated phones, the host first.
type table struct {
	r      *room
	board  *tab
	host   *tab
	phones []*tab
}

func (tt *table) all() []*tab { return append([]*tab{tt.board}, tt.phones...) }

func newTable(t *testing.T) *table {
	r := newRoom(t)
	tt := &table{r: r}
	tt.board = r.open("board", "/board")
	tt.host = r.joinPhone("Host", password)
	tt.phones = []*tab{tt.host, r.joinPhone("Bea", ""), r.joinPhone("Cy", ""), r.joinPhone("Dee", "")}
	tt.board.waitFor(`document.querySelectorAll(".ui-seat").length === 4 && !document.querySelector(".ui-token-dim")`)
	return tt
}

func (tt *table) waitAll(id string) {
	tt.r.t.Helper()
	for _, tb := range tt.all() {
		tb.waitTenant(id)
	}
}

func (tt *table) load(id string) {
	tt.r.t.Helper()
	tt.host.submit(fmt.Sprintf(`form[hx-post="/settings/load"]:has(input[name=game_id][value=%q])`, id))
	tt.host.waitFor(`!!document.querySelector('form[hx-post="/settings/start"]')`)
}

func (tt *table) start(id string) {
	tt.r.t.Helper()
	tt.host.submit(`form[hx-post="/settings/start"]`)
	tt.waitAll(id)
}

func (tt *table) stop() {
	tt.r.t.Helper()
	tt.host.submit(`form[hx-post="/settings/stop"]`)
	tt.waitAll("lobby")
}

func catalogIDs() []string {
	var ids []string
	for _, f := range games.Catalog() {
		ids = append(ids, f.ID)
	}
	sort.Strings(ids)
	return ids
}

// TestLobbyStartEndStartEveryGame is the GM-017 cycle: for every registered
// game, Lobby to game to Lobby to game on the board and every phone, with no
// navigation, no reload, no leak, and no shell error.
func TestLobbyStartEndStartEveryGame(t *testing.T) {
	tt := newTable(t)
	for _, id := range catalogIDs() {
		t.Run(id, func(t *testing.T) {
			tt.r.t = t
			tt.load(id)
			tt.start(id)
			prefix := id
			if id == "borrowedtruths" {
				prefix = "bt"
			}
			tt.board.waitFor(fmt.Sprintf(`!!document.querySelector("#shell-stage #%s-board")`, prefix))
			for _, tb := range tt.phones {
				tb.waitFor(fmt.Sprintf(`!!document.querySelector("#shell-stage #%s-phone") && !document.getElementById("shell-leave").hidden && !document.getElementById("shell-help-button").hidden`, prefix))
			}
			if id == "testing" {
				tt.phones[1].run(`document.querySelector(".ui-plunger").click()`)
				tt.board.waitFor(`[...document.querySelectorAll("[data-taps]")].some((el) => el.textContent.trim() === "1")`)
			}
			tt.stop()
			tt.start(id)
			tt.stop()
			for _, tb := range tt.all() {
				tb.assertSteady()
				h := tb.health()
				mounted := map[string]int{}
				for _, m := range h.Mounts {
					mounted[m.Tenant]++
				}
				if mounted[id] < 2 {
					t.Fatalf("%s mounted %s %d times, want 2: %+v", tb.name, id, mounted[id], h.Mounts)
				}
			}
		})
	}
}

// TestLobbyActionsStayInPlace taps Ready, Reroll, Stand, Sit, auto-start,
// Leave, and Join again. None of them may reload the phone.
func TestLobbyActionsStayInPlace(t *testing.T) {
	tt := newTable(t)
	tt.load("testing")
	bea := tt.phones[1]

	bea.submit(`form[hx-post="/lobby/ready"]`)
	bea.waitFor(`!!document.querySelector(".ui-plunger.is-ready")`)
	tt.board.waitFor(`document.querySelectorAll(".ui-check").length === 1`)

	var before, after string
	bea.eval(`document.querySelector(".ui-token-self svg title").textContent`, &before)
	bea.submit(`form[hx-post="/lobby/reroll"]`)
	bea.waitFor(fmt.Sprintf(`document.querySelector(".ui-token-self svg title").textContent !== %q`, before))
	bea.eval(`document.querySelector(".ui-token-self svg title").textContent`, &after)
	if after == before {
		t.Fatal("reroll kept the same face")
	}

	tt.host.submit(`form[hx-post="/settings/stand"]`)
	tt.host.waitFor(`!!document.querySelector('form[hx-post="/settings/sit"]')`)
	tt.host.submit(`form[hx-post="/settings/sit"]`)
	tt.host.waitFor(`!!document.querySelector('form[hx-post="/settings/stand"]')`)

	tt.host.waitFor(`/On|Off/.test(document.querySelector('form[hx-post="/settings/auto-start"] button').textContent)`)
	var auto string
	tt.host.eval(`document.querySelector('form[hx-post="/settings/auto-start"] button').textContent`, &auto)
	tt.host.submit(`form[hx-post="/settings/auto-start"]`)
	tt.host.waitFor(fmt.Sprintf(`document.querySelector('form[hx-post="/settings/auto-start"] button').textContent !== %q`, auto))

	dee := tt.phones[3]
	dee.run(`window.confirm = () => true`)
	dee.submit(`form[hx-post="/lobby/leave"]`)
	dee.waitFor(`!document.getElementById("shell-stage").hasAttribute("data-player") && !!document.querySelector('form[hx-post="/lobby/join"]')`)
	tt.board.waitFor(`document.querySelectorAll(".ui-seat").length === 3`)
	dee.run(`(() => {
		const f = document.querySelector('form[hx-post="/lobby/join"]');
		f.querySelector('[name=display_name]').value = "Dee";
		f.requestSubmit();
	})()`)
	dee.waitFor(`document.getElementById("shell-stage").hasAttribute("data-player") && document.querySelector(".ui-you").textContent === "Dee"`)

	for _, tb := range tt.all() {
		tb.assertSteady()
	}
}

// TestBurstSettlesOnCurrentTenant fires Start and Stop back to back, then a
// pile of no-op refetch triggers. Each tab mounts each generation at most
// once and ends on what the host is showing.
func TestBurstSettlesOnCurrentTenant(t *testing.T) {
	tt := newTable(t)
	tt.load("testing")
	for i := 0; i < 3; i++ {
		tt.r.post("/settings/start", nil, true)
		tt.r.post("/settings/stop", nil, true)
	}
	tt.r.post("/settings/start", nil, true)
	tt.waitAll("testing")

	for _, tb := range tt.all() {
		h := tb.health()
		seen := map[int]int{}
		for _, m := range h.Mounts {
			seen[m.Generation]++
			if seen[m.Generation] > 1 {
				t.Fatalf("%s mounted generation %d twice: %+v", tb.name, m.Generation, h.Mounts)
			}
		}
		count := len(h.Mounts)
		tb.run(`for (let i = 0; i < 25; i++) document.dispatchEvent(new CustomEvent("grabbag:tenant"))`)
		time.Sleep(600 * time.Millisecond)
		if got := len(tb.health().Mounts); got != count {
			t.Fatalf("%s remounted on no-op triggers: %d mounts, want %d", tb.name, got, count)
		}
		if tb.tenant() != "testing" {
			t.Fatalf("%s settled on %s", tb.name, tb.tenant())
		}
		tb.assertSteady()
	}
}

// TestLockedBoardHearsNoRoomData locks the board, makes the host write a log
// line, a notice, and a roster change, and checks none of it reached the
// locked board. Then it unlocks live.
func TestLockedBoardHearsNoRoomData(t *testing.T) {
	r := newRoom(t)
	board := r.open("board", "/board")
	board.waitTenant("lobby")

	r.post("/settings/admin-only-board", url.Values{"enabled": {"1"}}, true)
	board.waitTenant("locked")
	board.waitFor(`grabbagShell.current().live && grabbagShell.current().stream === "/shell/events/locked"`)
	board.waitFor(`!document.body.textContent.includes("Audience Members") && document.body.textContent.includes("Host screen only")`)
	board.run(`grabbagShell.record.events.length = 0`)

	// A log line, a notice, a roster change, then a tenant change.
	if resp, err := http.Post(r.server.URL+"/shell/error", "application/json", strings.NewReader(`{"surface":"phone","tenant":"x","phase":"mount","message":"secret"}`)); err != nil || resp.StatusCode != http.StatusNoContent {
		t.Fatalf("shell error post = %v %v", resp, err)
	}
	r.post("/settings/load", url.Values{"game_id": {"no-such-game"}}, true)
	r.post("/settings/close", nil, true)
	r.post("/settings/auto-start", url.Values{"enabled": {"0"}}, true)
	board.waitFor(`grabbagShell.record.events.includes("tenant")`)
	time.Sleep(500 * time.Millisecond)
	for _, name := range board.health().Events {
		if name != "tenant" && name != "theme" {
			t.Fatalf("locked board heard %q (all: %v)", name, board.health().Events)
		}
	}

	r.post("/settings/admin-only-board", url.Values{"enabled": {"0"}}, true)
	board.waitTenant("lobby")
	board.waitFor(`grabbagShell.current().live && grabbagShell.current().stream === "/lobby/events"`)
	board.run(`grabbagShell.record.events.length = 0`)
	r.post("/settings/open", nil, true)
	board.waitFor(`grabbagShell.record.events.includes("roster")`)
	board.assertSteady()
}

// TestKickReloadsOnce kicks a phone. That phone reloads exactly once, for
// "kicked", and lands on Join. The host phone does not reload.
func TestKickReloadsOnce(t *testing.T) {
	r := newRoom(t)
	hostPhone := r.joinPhone("Host", password)
	bea := r.joinPhone("Bea", "")

	hostPhone.waitFor(`[...document.querySelectorAll('form[hx-post="/settings/kick"] span')].some((s) => s.textContent.trim() === "Bea")`)
	hostPhone.run(`[...document.querySelectorAll('form[hx-post="/settings/kick"]')]
		.find((f) => f.querySelector("span").textContent.trim() === "Bea").requestSubmit()`)
	var ok bool
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		err := chromedp.Run(bea.ctx, chromedp.Evaluate(`sessionStorage.getItem("grabbagShellLoads") === "2" && !!window.grabbagShell && document.body.textContent.includes("The host kicked you")`, &ok))
		if err == nil && ok {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !ok {
		t.Fatalf("kicked phone did not reload to Join: %+v", bea.health())
	}
	time.Sleep(1500 * time.Millisecond)
	h := bea.health()
	if h.Loads != 2 || len(h.Reloads) != 1 || h.Reloads[0] != "kicked" {
		t.Fatalf("kicked phone loads=%d reloads=%v, want one kicked reload", h.Loads, h.Reloads)
	}
	hostPhone.assertSteady()
}

// TestReconnectKeepsThePageAndRestartReloads drops every stream. The shells
// reconnect and refetch without a reload. Then the host "restarts" with a
// new boot ID, and each shell reloads exactly once, for "host-restarted".
func TestReconnectKeepsThePageAndRestartReloads(t *testing.T) {
	r := newRoom(t)
	board := r.open("board", "/board")
	bea := r.joinPhone("Bea", "")

	r.server.CloseClientConnections()
	for _, tb := range []*tab{board, bea} {
		tb.waitFor(`grabbagShell.current().live`)
		tb.waitFor(`grabbagShell.record.mounts.length >= 2`)
		tb.assertSteady()
	}

	r.restart()
	for _, tb := range []*tab{board, bea} {
		deadline := time.Now().Add(20 * time.Second)
		var ok bool
		for time.Now().Before(deadline) {
			err := chromedp.Run(tb.ctx, chromedp.Evaluate(`sessionStorage.getItem("grabbagShellLoads") === "2" && !!window.grabbagShell && grabbagShell.current() !== null`, &ok))
			if err == nil && ok {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		h := tb.health()
		if !ok || h.Loads != 2 || len(h.Reloads) != 1 || h.Reloads[0] != "host-restarted" {
			t.Fatalf("%s after restart: loads=%d reloads=%v", tb.name, h.Loads, h.Reloads)
		}
	}
}

// TestJoinWhileTheFormIsReplaced submits Join and removes the form before
// the response, as a roster refetch of #phone-inner can. The phone must
// still swap to the room.
func TestJoinWhileTheFormIsReplaced(t *testing.T) {
	r := newRoom(t)
	bea := r.open("Bea", "/")
	bea.waitFor(`!!document.querySelector('form[hx-post="/lobby/join"]')`)
	bea.run(`(() => {
		const f = document.querySelector('form[hx-post="/lobby/join"]');
		f.querySelector('[name=display_name]').value = "Bea";
		f.requestSubmit();
		f.closest("#phone-inner").remove();
	})()`)
	bea.waitFor(`document.getElementById("shell-stage").hasAttribute("data-player") && document.querySelector(".ui-you").textContent === "Bea"`)
	bea.assertSteady()
}

// TestFailedScriptLoadIsRetried fails every game.js request during the first
// Start. After Stop, the next Start must fetch the script again and the game
// must register, with no new "did not register" error.
func TestFailedScriptLoadIsRetried(t *testing.T) {
	tt := newTable(t)
	var failing atomic.Bool
	failing.Store(true)
	fail := func(w http.ResponseWriter, req *http.Request) bool {
		if failing.Load() && strings.HasSuffix(req.URL.Path, "/games/quips/static/game.js") {
			http.Error(w, "flaky", http.StatusServiceUnavailable)
			return true
		}
		return false
	}
	tt.r.intercept.Store(&fail)
	tt.load("quips")
	tt.start("quips")
	tt.board.waitFor(`grabbagShell.record.errors.some((e) => e.phase === "register" && e.tenant === "quips")`)
	failing.Store(false)
	tt.stop()
	var before int
	tt.board.eval(`grabbagShell.record.errors.filter((e) => e.phase === "register").length`, &before)
	tt.start("quips")
	tt.board.waitFor(`grabbagShell.record.mounts.filter((m) => m.tenant === "quips").length >= 2`)
	var after int
	tt.board.eval(`grabbagShell.record.errors.filter((e) => e.phase === "register").length`, &after)
	if after != before {
		t.Fatalf("quips did not register after the retry: register errors %d -> %d", before, after)
	}
}

// TestFirstOpenAfterAnOutageReconciles keeps the board's stream down from
// the start, restarts the host, then lets the stream through. The first
// open must compare boot IDs and reload for host-restarted.
func TestFirstOpenAfterAnOutageReconciles(t *testing.T) {
	r := newRoom(t)
	var blocked atomic.Bool
	blocked.Store(true)
	block := func(w http.ResponseWriter, req *http.Request) bool {
		if blocked.Load() && req.URL.Path == "/lobby/events" {
			http.Error(w, "down", http.StatusServiceUnavailable)
			return true
		}
		return false
	}
	r.intercept.Store(&block)
	board := r.open("board", "/board")
	time.Sleep(300 * time.Millisecond)
	r.restart()
	blocked.Store(false)
	deadline := time.Now().Add(20 * time.Second)
	var ok bool
	for time.Now().Before(deadline) {
		err := chromedp.Run(board.ctx, chromedp.Evaluate(`sessionStorage.getItem("grabbagShellLoads") === "2" && !!window.grabbagShell && grabbagShell.current() !== null`, &ok))
		if err == nil && ok {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if h := board.health(); !ok || len(h.Reloads) != 1 || h.Reloads[0] != "host-restarted" {
		t.Fatalf("board after outage and restart: loads=%d reloads=%v", h.Loads, h.Reloads)
	}
}

// TestStaticPreviewRefitsOnResize loads an Apples reveal preview, then
// resizes the page in place the way the gallery resizes its frame. The
// shell's viewport values and the card text fit must follow.
func TestStaticPreviewRefitsOnResize(t *testing.T) {
	bin := chromeBinary()
	if bin == "" || testing.Short() {
		t.Skip("no Chromium, or -short")
	}
	server := httptest.NewServer(host.PreviewHandler())
	t.Cleanup(server.Close)
	tb := previewTab(t, bin)
	ctx := tb.ctx
	if err := chromedp.Run(ctx, chromedp.EmulateViewport(1280, 800),
		chromedp.Navigate(server.URL+"/dev/ui/s/apples/board/reveal?players=7")); err != nil {
		t.Fatal(err)
	}
	tb.waitFor(`getComputedStyle(document.documentElement).getPropertyValue("--shell-visual-height") === "800px"`)
	if err := chromedp.Run(ctx, chromedp.EmulateViewport(854, 480)); err != nil {
		t.Fatal(err)
	}
	tb.waitFor(`getComputedStyle(document.documentElement).getPropertyValue("--shell-visual-height") === "480px" && document.documentElement.classList.contains("shell-short")`)
	tb.waitFor(`[...document.querySelectorAll(".apples-tv .apples-slot:not(.down) .apples-slot-copy")].every((c) => c.scrollHeight <= c.clientHeight + 1 && c.scrollWidth <= c.clientWidth + 1)`)
}

// previewTab is a browser with no room, for static preview pages.
func previewTab(t *testing.T, bin string) *tab {
	t.Helper()
	opts := append(chromedp.DefaultExecAllocatorOptions[:], chromedp.ExecPath(bin))
	if os.Getenv("CI") != "" {
		opts = append(opts, chromedp.NoSandbox)
	}
	alloc, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
	t.Cleanup(cancelAlloc)
	ctx, cancel := chromedp.NewContext(alloc)
	t.Cleanup(cancel)
	return &tab{r: &room{t: t}, name: "preview", ctx: ctx}
}

// frameBox is where the phone frame put the column and its controls.
type frameBox struct {
	Mode      string  `json:"mode"`
	Width     float64 `json:"width"`
	Column    rect    `json:"column"`
	Bar       rect    `json:"bar"`
	Help      rect    `json:"help"`
	Leave     rect    `json:"leave"`
	Container float64 `json:"container"`
}

type rect struct {
	Left   float64 `json:"left"`
	Right  float64 `json:"right"`
	Top    float64 `json:"top"`
	Bottom float64 `json:"bottom"`
}

const frameBoxJS = `JSON.stringify((() => {
	const box = (el) => { const r = el.getBoundingClientRect(); return {left: r.left, right: r.right, top: r.top, bottom: r.bottom}; };
	const column = document.querySelector(".shell-column");
	return {
		mode: document.getElementById("shell-frame").getAttribute("data-frame"),
		width: document.documentElement.clientWidth,
		column: box(column),
		bar: box(document.querySelector(".shell-bar")),
		help: box(document.getElementById("shell-help-button")),
		leave: box(document.getElementById("shell-leave")),
		container: document.getElementById("shell-stage").getBoundingClientRect().width,
	};
})())`

func (tb *tab) frameBox() frameBox {
	tb.r.t.Helper()
	var raw string
	tb.eval(frameBoxJS, &raw)
	var b frameBox
	if err := json.Unmarshal([]byte(raw), &b); err != nil {
		tb.r.t.Fatal(err)
	}
	return b
}

// TestPhoneFrameColumnAtEverySize checks GM-005 and GM-006 on a game phone
// preview: the column is 720px with 64px gutters when there is room and
// fills the width when there is not, and Help and Leave sit in the toolbar
// or the gutters without overlapping the column. A screen with
// data-column="wide" gets a 1200px column.
func TestPhoneFrameColumnAtEverySize(t *testing.T) {
	bin := chromeBinary()
	if bin == "" || testing.Short() {
		t.Skip("no Chromium, or -short")
	}
	server := httptest.NewServer(host.PreviewHandler())
	t.Cleanup(server.Close)
	tb := previewTab(t, bin)
	if err := chromedp.Run(tb.ctx, chromedp.EmulateViewport(393, 852),
		chromedp.Navigate(server.URL+"/dev/ui/s/testing/phone/live-seated")); err != nil {
		t.Fatal(err)
	}
	sizes := []struct {
		name   string
		w, h   int64
		wide   bool
		mode   string
		column float64
	}{
		{"phone", 393, 852, false, "phone", 393},
		{"ipad-portrait", 820, 1180, false, "phone", 820},
		{"ipad-landscape", 1180, 820, false, "gutter", 720},
		{"desktop", 1440, 900, false, "gutter", 720},
		{"ipad-landscape-wide", 1180, 820, true, "phone", 1180},
		{"desktop-wide", 1440, 900, true, "gutter", 1200},
	}
	for _, sz := range sizes {
		if err := chromedp.Run(tb.ctx, chromedp.EmulateViewport(sz.w, sz.h)); err != nil {
			t.Fatal(err)
		}
		tb.run(fmt.Sprintf(`document.querySelector("#shell-stage > *").toggleAttribute("data-column", %t); if (%t) document.querySelector("#shell-stage > *").setAttribute("data-column", "wide")`, sz.wide, sz.wide))
		tb.waitFor(fmt.Sprintf(`document.documentElement.clientWidth === %d && Math.round(document.querySelector(".shell-column").getBoundingClientRect().width) === %d && document.getElementById("shell-frame").getAttribute("data-frame") === %q`, sz.w, int(sz.column), sz.mode))
		b := tb.frameBox()
		if b.Container != b.Column.Right-b.Column.Left {
			t.Errorf("%s: stage %v is not the column %v", sz.name, b.Container, b.Column)
		}
		if sz.mode == "gutter" {
			if b.Column.Left < 64 || b.Width-b.Column.Right < 64 {
				t.Errorf("%s: gutters under 64px: %+v", sz.name, b)
			}
			if b.Help.Right > b.Column.Left || b.Leave.Left < b.Column.Right {
				t.Errorf("%s: Help or Leave overlaps the column: %+v", sz.name, b)
			}
			if b.Help.Top < b.Bar.Bottom || b.Leave.Top < b.Bar.Bottom {
				t.Errorf("%s: Help or Leave still in the toolbar: %+v", sz.name, b)
			}
		} else {
			if b.Column.Left != 0 || b.Column.Right != b.Width {
				t.Errorf("%s: phone column does not fill the width: %+v", sz.name, b)
			}
			if b.Help.Bottom > b.Bar.Bottom || b.Leave.Bottom > b.Bar.Bottom || b.Help.Left < 0 || b.Leave.Right > b.Width {
				t.Errorf("%s: Help or Leave outside the toolbar: %+v", sz.name, b)
			}
		}
	}
}

// TestHostPanelStaysOpenOnWideScreens checks GM-011: from 1170px the
// claimed host's panel stays open on the left without a drawer handle,
// other players get no panel, and collapsing it survives a reload.
func TestHostPanelStaysOpenOnWideScreens(t *testing.T) {
	r := newRoom(t)
	hostPhone := r.join(r.openSized("Host", "/", 1440, 900), "Host", password)
	bea := r.join(r.openSized("Bea", "/", 1440, 900), "Bea", "")
	const panelOpen = `(() => {
		const host = document.getElementById("shell-host").getBoundingClientRect();
		const drawer = document.getElementById("host-drawer");
		const handle = document.querySelector(".ui-drawer-handle");
		return Math.round(host.width) === 320 && host.left === 0 && !!drawer && drawer.getBoundingClientRect().width > 300 &&
			getComputedStyle(handle).display === "none" && document.querySelector(".shell-column").getBoundingClientRect().left >= 320 + 64;
	})()`
	hostPhone.waitFor(panelOpen)
	bea.waitFor(`document.getElementById("shell-host").innerHTML.trim() === "" && document.querySelector(".shell-column").getBoundingClientRect().left === (document.documentElement.clientWidth - 720) / 2`)

	hostPhone.run(`document.querySelector(".ui-drawer-collapse").click()`)
	const collapsed = `document.getElementById("shell-frame").getAttribute("data-host-panel") === "collapsed" &&
		Math.round(document.getElementById("shell-host").getBoundingClientRect().width) === 44 &&
		getComputedStyle(document.getElementById("host-drawer")).display === "none"`
	hostPhone.waitFor(collapsed)
	if err := chromedp.Run(hostPhone.ctx, chromedp.Reload()); err != nil {
		t.Fatal(err)
	}
	hostPhone.waitFor(`window.grabbagShell && grabbagShell.current() !== null`)
	hostPhone.waitFor(collapsed)
	hostPhone.run(`document.querySelector(".ui-drawer-expand").click()`)
	hostPhone.waitFor(panelOpen)

	// Under 1170px it is the slide-out drawer again.
	if err := chromedp.Run(hostPhone.ctx, chromedp.EmulateViewport(1000, 800)); err != nil {
		t.Fatal(err)
	}
	hostPhone.waitFor(`getComputedStyle(document.querySelector(".ui-drawer-handle")).display !== "none" && document.getElementById("host-drawer").getBoundingClientRect().right <= 0`)
	bea.assertSteady()
}

// TestHelpSheetKeepsTheTenantMounted checks GM-009 and GM-010: Help shows
// for a game and not the Lobby, the sheet opens over the game, and ✕, Esc,
// and the backdrop close it, with no unmount.
func TestHelpSheetKeepsTheTenantMounted(t *testing.T) {
	tt := newTable(t)
	bea := tt.phones[1]
	bea.waitFor(`document.getElementById("shell-help-button").hidden`)
	tt.load("testing")
	tt.start("testing")
	bea.waitFor(`!document.getElementById("shell-help-button").hidden`)
	var mounts int
	bea.eval(`grabbagShell.record.mounts.length`, &mounts)
	for _, closer := range []string{
		`document.querySelector(".shell-sheet-close").click()`,
		`document.dispatchEvent(new KeyboardEvent("keydown", {key: "Escape"}))`,
		`document.querySelector(".shell-sheet-scrim").click()`,
	} {
		bea.run(`document.getElementById("shell-help-button").click()`)
		bea.waitFor(`!document.getElementById("shell-help").hidden && document.getElementById("shell-help-body").textContent.includes("Latency")`)
		bea.run(closer)
		bea.waitFor(`document.getElementById("shell-help").hidden && !!document.querySelector("#shell-stage #testing-phone")`)
	}
	var after int
	bea.eval(`grabbagShell.record.mounts.length`, &after)
	if after != mounts {
		t.Fatalf("help remounted the tenant: %d mounts, then %d", mounts, after)
	}
	tt.stop()
	bea.waitFor(`document.getElementById("shell-help-button").hidden`)
	bea.assertSteady()
}
