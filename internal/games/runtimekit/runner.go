// Package runtimekit is the optional match runtime for game packages. A Game
// keeps its rules in a pure engine and hands the plumbing around it to a
// Runner: the lock, the timer tick, the player and host POST pipeline, SSE
// publish after a change, and Pause, Resume, and Finish calls made with the
// lock released. Host never sees the kit. A game that wants its own runtime
// implements games.Game directly and skips this package.
package runtimekit

import (
	"html/template"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/KroniK907/grabbag/internal/games"
)

// DefaultTick is how often the Runner asks the engine to advance when
// Config.Tick is zero.
const DefaultTick = 250 * time.Millisecond

// Result is what an action or tick asks the Runner to do once run state has
// changed. The Runner publishes and calls host for every field that is set,
// even when Err is set.
type Result struct {
	// Err is the phone error for the player who acted. Empty is success.
	Err string
	// Changed publishes Config.Event.
	Changed bool
	// Events are extra SSE names to publish, in order, after Config.Event.
	Events []string
	// Pause, Resume, and Finish call the matching Helper method with the
	// lock released.
	Pause  bool
	Resume bool
	Finish bool
}

// Action is one player or host POST against the engine. The form is already
// parsed. Close over the request for form values.
type Action[E any] func(e E, p games.Player, now time.Time) Result

// Config wires a Runner to one game. Every hook runs with the lock held.
type Config[E any] struct {
	// Game is the player-facing name used in render errors.
	Game string
	// Event is the SSE name published when a Result has Changed.
	Event string
	// Pages and AssetVersion back Render and Page.
	Pages        *template.Template
	AssetVersion string
	// Tick is the timer poll interval. Zero is DefaultTick.
	Tick time.Duration

	// Advance runs on every tick while a match runs and is not paused.
	// Fire expired timers here. Nil means the game has no timers.
	Advance func(e E, now time.Time) Result
	// Hold freezes or thaws the engine clock when host pauses or resumes.
	// Games with their own holds, such as a reshuffle overlay, fold them in
	// here.
	Hold func(e E, paused bool, now time.Time)
	// Gate refuses an Act or HostAct before it runs. Return the phone error,
	// or "" to let the action run. Nil lets every action run, paused or not.
	Gate func(e E, p games.Player, paused bool) string
	// Reply picks the template and view that answer an Act or HostAct. msg
	// is the phone error or "". e is the zero E when the action ended the
	// match.
	Reply func(e E, p games.Player, r *http.Request, msg string) (name string, data any)
	// Cleanup runs on Stop and Shutdown after the tick stops and before the
	// engine is dropped. e may be the zero E. unload is true on Shutdown;
	// the helper is still set.
	Cleanup func(e E, unload bool)
}

// Runner owns one game's match runtime. Methods named Locked, and Engine,
// Running, Paused, Rand, Hold, Publish, Apply, and Defer, need the lock
// held. The rest take it.
type Runner[E any] struct {
	cfg Config[E]

	mu      sync.Mutex
	helper  games.Helper
	engine  E
	running bool
	paused  bool
	stop    chan struct{}
	pending Result
	rng     *rand.Rand
	now     func() time.Time
}

// New builds an unloaded Runner.
func New[E any](cfg Config[E]) *Runner[E] {
	if cfg.Tick <= 0 {
		cfg.Tick = DefaultTick
	}
	return &Runner[E]{cfg: cfg}
}

// Lock takes the game lock. Game code that reads or changes run state outside
// a hook holds it.
func (r *Runner[E]) Lock() { r.mu.Lock() }

// Unlock releases the game lock.
func (r *Runner[E]) Unlock() { r.mu.Unlock() }

// Engine is the running match, or the zero E. Lock held.
func (r *Runner[E]) Engine() E { return r.engine }

// Running reports a started match. Lock held.
func (r *Runner[E]) Running() bool { return r.running }

// Paused reports a host pause. Lock held.
func (r *Runner[E]) Paused() bool { return r.paused }

// Helper is the loaded helper, or nil.
func (r *Runner[E]) Helper() games.Helper {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.helper
}

// HelperLocked is Helper with the lock held.
func (r *Runner[E]) HelperLocked() games.Helper { return r.helper }

// Now is the game clock: SetClock's func, or wall time.
func (r *Runner[E]) Now() time.Time {
	if r.now != nil {
		return r.now()
	}
	return time.Now()
}

// SetClock replaces the clock for tests and previews. Call it before the
// match runs.
func (r *Runner[E]) SetClock(now func() time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.now = now
}

// Rand is the match random source, seeded from the clock on first use.
// Lock held.
func (r *Runner[E]) Rand() *rand.Rand {
	if r.rng == nil {
		r.rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	return r.rng
}

// SetRand replaces the random source for tests. Call it before Start.
func (r *Runner[E]) SetRand(rng *rand.Rand) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rng = rng
}

// Install puts e in place as a running match with no helper and no tick.
// Previews and tests use it to drive views from a fixed engine.
func (r *Runner[E]) Install(e E) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.engine = e
	r.running = true
}

// Load stores h and drops any match. Game.Load calls it first.
func (r *Runner[E]) Load(h games.Helper) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopTickLocked()
	r.helper = h
	r.dropLocked()
}

// Start stores h, builds the match with begin under the lock, and starts the
// tick. An error from begin leaves no match running.
func (r *Runner[E]) Start(h games.Helper, begin func(h games.Helper) (E, error)) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.helper = h
	e, err := begin(h)
	if err != nil {
		return err
	}
	r.engine = e
	r.running = true
	r.paused = false
	r.startTickLocked()
	return nil
}

// Pause holds the engine clock and closes the Gate's paused flag.
func (r *Runner[E]) Pause() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.paused = true
	r.holdLocked()
	return nil
}

// Resume thaws the engine clock.
func (r *Runner[E]) Resume() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.paused = false
	r.holdLocked()
	return nil
}

// Stop ends the match. The helper stays.
func (r *Runner[E]) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopTickLocked()
	if r.cfg.Cleanup != nil {
		r.cfg.Cleanup(r.engine, false)
	}
	r.dropLocked()
	return nil
}

// Shutdown ends the match and drops the helper.
func (r *Runner[E]) Shutdown() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopTickLocked()
	if r.cfg.Cleanup != nil {
		r.cfg.Cleanup(r.engine, true)
	}
	r.dropLocked()
	r.helper = nil
	return nil
}

func (r *Runner[E]) dropLocked() {
	var zero E
	r.engine = zero
	r.pending = Result{}
	r.running = false
	r.paused = false
}

// Hold re-applies the Hold hook, for a game whose own hold changed. Lock
// held.
func (r *Runner[E]) Hold() { r.holdLocked() }

func (r *Runner[E]) holdLocked() {
	if r.running && r.cfg.Hold != nil {
		r.cfg.Hold(r.engine, r.paused, r.Now())
	}
}

// Publish sends each name on the room hub. Lock held.
func (r *Runner[E]) Publish(names ...string) {
	if r.helper == nil {
		return
	}
	for _, name := range names {
		r.helper.Publish(name)
	}
}

// Apply publishes for res, then makes its host calls and any that Defer
// queued. Host calls release the lock and take it back, since host may call
// Stop or Pause on this game, so re-read the engine after Apply. Lock held.
func (r *Runner[E]) Apply(res Result) {
	r.publishFor(res)
	res.Pause = res.Pause || r.pending.Pause
	res.Resume = res.Resume || r.pending.Resume
	res.Finish = res.Finish || r.pending.Finish
	r.pending = Result{}
	h := r.helper
	if h == nil || !(res.Pause || res.Resume || res.Finish) {
		return
	}
	r.mu.Unlock()
	defer r.mu.Lock()
	if res.Pause {
		h.Pause()
	}
	if res.Resume {
		h.Resume()
	}
	if res.Finish {
		h.Finish()
	}
}

// Defer publishes for res now and queues its host calls for the next Apply,
// action reply, or tick. Use it where the lock cannot be released, such as
// while building a view. Lock held.
func (r *Runner[E]) Defer(res Result) {
	r.publishFor(res)
	r.pending.Pause = r.pending.Pause || res.Pause
	r.pending.Resume = r.pending.Resume || res.Resume
	r.pending.Finish = r.pending.Finish || res.Finish
}

func (r *Runner[E]) publishFor(res Result) {
	if res.Changed && r.cfg.Event != "" {
		r.Publish(r.cfg.Event)
	}
	r.Publish(res.Events...)
}

func (r *Runner[E]) startTickLocked() {
	r.stopTickLocked()
	if r.cfg.Advance == nil {
		return
	}
	stop := make(chan struct{})
	r.stop = stop
	go func() {
		tick := time.NewTicker(r.cfg.Tick)
		defer tick.Stop()
		for {
			select {
			case <-stop:
				return
			case <-tick.C:
				r.mu.Lock()
				select {
				case <-stop:
					// Stop ran while this tick waited on the lock.
				default:
					r.tickLocked()
				}
				r.mu.Unlock()
			}
		}
	}()
}

// Tick runs one tick now, as the ticker would. Tests use it to fire timers
// without waiting.
func (r *Runner[E]) Tick() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tickLocked()
}

func (r *Runner[E]) tickLocked() {
	if !r.running {
		return
	}
	if r.paused || r.cfg.Advance == nil {
		r.Apply(Result{})
		return
	}
	r.Apply(r.cfg.Advance(r.engine, r.Now()))
}

func (r *Runner[E]) stopTickLocked() {
	if r.stop != nil {
		close(r.stop)
		r.stop = nil
	}
}

// Player is the signed-in player on req. It answers 404 before Load and 401
// without a player cookie.
func (r *Runner[E]) Player(w http.ResponseWriter, req *http.Request) (games.Player, bool) {
	h := r.Helper()
	if h == nil {
		http.NotFound(w, req)
		return games.Player{}, false
	}
	p, ok, err := h.PlayerFromRequest(req)
	if err != nil || !ok {
		http.Error(w, "Sign in to play.", http.StatusUnauthorized)
		return games.Player{}, false
	}
	return p, true
}

// HostPlayer lets the claimed host or an admin session through. An admin
// with no player cookie gets the zero Player. It answers 404 before Load and
// 403 for anyone else.
func (r *Runner[E]) HostPlayer(w http.ResponseWriter, req *http.Request) (games.Player, bool) {
	h := r.Helper()
	if h == nil {
		http.NotFound(w, req)
		return games.Player{}, false
	}
	p, ok, err := h.PlayerFromRequest(req)
	if err != nil || !ok {
		p = games.Player{}
	}
	if h.HasAdmin(req) || p.ClaimedHost {
		return p, true
	}
	http.Error(w, "Host only.", http.StatusForbidden)
	return games.Player{}, false
}

// Act runs fn for the signed-in player and answers with Reply.
func (r *Runner[E]) Act(w http.ResponseWriter, req *http.Request, fn Action[E]) {
	p, ok := r.Player(w, req)
	if !ok {
		return
	}
	r.run(w, req, p, true, fn)
}

// HostAct is Act for the claimed host or an admin session.
func (r *Runner[E]) HostAct(w http.ResponseWriter, req *http.Request, fn Action[E]) {
	p, ok := r.HostPlayer(w, req)
	if !ok {
		return
	}
	r.run(w, req, p, true, fn)
}

// HostPause is a ready-made host Pause button. It passes the Gate like any
// HostAct.
func (r *Runner[E]) HostPause(w http.ResponseWriter, req *http.Request) {
	r.HostAct(w, req, func(E, games.Player, time.Time) Result { return Result{Pause: true} })
}

// HostResume is a ready-made host Resume button. It skips the Gate, since a
// paused Gate would refuse it.
func (r *Runner[E]) HostResume(w http.ResponseWriter, req *http.Request) {
	p, ok := r.HostPlayer(w, req)
	if !ok {
		return
	}
	r.run(w, req, p, false, func(E, games.Player, time.Time) Result { return Result{Resume: true} })
}

func (r *Runner[E]) run(w http.ResponseWriter, req *http.Request, p games.Player, gated bool, fn Action[E]) {
	if err := req.ParseForm(); err != nil {
		http.Error(w, "Could not read the form.", http.StatusBadRequest)
		return
	}
	r.mu.Lock()
	if !r.running {
		r.mu.Unlock()
		http.NotFound(w, req)
		return
	}
	msg := ""
	if gated && r.cfg.Gate != nil {
		msg = r.cfg.Gate(r.engine, p, r.paused)
	}
	if msg == "" {
		res := fn(r.engine, p, r.Now())
		msg = res.Err
		r.Apply(res)
	}
	name, data := r.cfg.Reply(r.engine, p, req, msg)
	r.mu.Unlock()
	r.Render(w, name, data, http.StatusOK)
}

// Render executes a game template. See the package func Render.
func (r *Runner[E]) Render(w http.ResponseWriter, name string, data any, status int) {
	Render(w, r.cfg.Pages, r.cfg.Game, name, data, status)
}

// Page is NewPage with the loaded helper and Config.AssetVersion.
func (r *Runner[E]) Page(title string) Page {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.PageLocked(title)
}

// PageLocked is Page with the lock held.
func (r *Runner[E]) PageLocked(title string) Page {
	return NewPage(title, r.helper, r.cfg.AssetVersion)
}
