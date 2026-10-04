package shelltest_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// awaitJSON evaluates an async expression that returns a JSON string and
// decodes it into out.
func (tb *tab) awaitJSON(js string, out any) {
	tb.r.t.Helper()
	var raw string
	err := chromedp.Run(tb.ctx, chromedp.Evaluate(js, &raw, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
		return p.WithAwaitPromise(true)
	}))
	if err != nil {
		tb.r.t.Fatalf("%s: %v", tb.name, err)
	}
	if err := json.Unmarshal([]byte(raw), out); err != nil {
		tb.r.t.Fatalf("%s: decode %s: %v", tb.name, raw, err)
	}
}

// click is a real mouse click on the first match, which counts as a user
// gesture.
func (tb *tab) click(selector string) {
	tb.r.t.Helper()
	ctx, cancel := context.WithTimeout(tb.ctx, 15*time.Second)
	defer cancel()
	if err := chromedp.Run(ctx, chromedp.Click(selector, chromedp.ByQuery, chromedp.NodeVisible)); err != nil {
		tb.r.t.Fatalf("%s: click %s: %v", tb.name, selector, err)
	}
}

// get fetches path as the operator and returns the body.
func (r *room) get(path string) string {
	r.t.Helper()
	req, err := http.NewRequest(http.MethodGet, r.server.URL+path, nil)
	if err != nil {
		r.t.Fatal(err)
	}
	for _, c := range r.operator {
		req.AddCookie(c)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		r.t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	return string(body)
}

func (r *room) waitStatus(want string) {
	r.t.Helper()
	for range 150 {
		if strings.Contains(r.get("/settings/audio/status"), want) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	r.t.Fatalf("board sound status never showed %q: %s", want, r.get("/settings/audio/status"))
}

const lobbyPlaying = `grabbagAudio.board.inspect().playing.some((p) => p.tenant === "lobby" && p.cue === "lobby" && p.state === "playing")`

// TestBoardMusicFollowsTheTenant is GM-013: lobby music plays on the board,
// goes quiet and is released during a game, and starts again at End. The
// board reports its sound state for /settings.
func TestBoardMusicFollowsTheTenant(t *testing.T) {
	r := newRoom(t)
	board := r.open("board", "/board")
	host := r.joinPhone("Host", password)
	board.waitFor(lobbyPlaying)
	r.waitStatus("<strong>On</strong>")

	tt := &table{r: r, board: board, host: host, phones: []*tab{host}}
	tt.load("testing")
	tt.start("testing")
	board.waitFor(`(() => { const s = grabbagAudio.board.inspect(); return s.playing.length === 0 && !s.scopes.some((x) => x.tenant === "lobby"); })()`)
	tt.stop()
	board.waitFor(lobbyPlaying)
	board.assertSteady()
}

// TestPickingAGameKeepsTheLobbyMusic: loading a game repaints the Lobby on
// the board. The same lobby cue keeps playing; it does not restart or
// crossfade into itself.
func TestPickingAGameKeepsTheLobbyMusic(t *testing.T) {
	r := newRoom(t)
	board := r.open("board", "/board")
	host := r.joinPhone("Host", password)
	board.waitFor(lobbyPlaying)
	board.run(`window.__lobbyCue = grabbagAudio.board.instances.find((i) => i.name === "lobby"); window.__gen = document.getElementById("shell-stage").dataset.generation`)

	tt := &table{r: r, board: board, host: host, phones: []*tab{host}}
	tt.load("testing")
	board.waitFor(`document.getElementById("shell-stage").dataset.generation !== __gen`)
	board.waitFor(`(() => { const live = grabbagAudio.board.instances.filter((i) => i.name === "lobby" && i.live()); return live.length === 1 && live[0] === __lobbyCue; })()`)
	board.assertSteady()
}

// TestSettingsMixerReachesTheBoard is GM-009 and GM-011: a slider or mute
// change on /settings reaches the board at once, and the test buttons play
// a sample there.
func TestSettingsMixerReachesTheBoard(t *testing.T) {
	r := newRoom(t)
	board := r.open("board", "/board")
	board.run(`window.__audioTests = []; grabbagAudio.board.tap((type, d) => { if (type === "audio-test") __audioTests.push(d.layer); })`)

	r.post("/settings/audio", url.Values{"layer": {"music"}, "volume": {"30"}}, true)
	board.waitFor(`grabbagAudio.board.inspect().levels.music.volume === 30`)
	r.post("/settings/audio", url.Values{"layer": {"effects"}, "muted": {"1"}}, true)
	board.waitFor(`grabbagAudio.board.inspect().levels.effects.muted === true`)
	r.post("/settings/audio/test", url.Values{"layer": {"effects"}}, true)
	board.waitFor(`__audioTests.join() === "effects"`)

	// A board that opens later starts from the stored levels.
	late := r.open("late board", "/board")
	late.waitFor(`(() => { const l = grabbagAudio.board.inspect().levels; return l.music.volume === 30 && l.effects.muted; })()`)
}

// blockAudioJS makes the board browse like a TV browser that blocks sound.
// Headless Chromium always allows audio, whatever autoplay-policy says, so
// AudioContext starts suspended here and resumes only from a user gesture.
const blockAudioJS = `
(() => {
  const Real = window.AudioContext;
  window.AudioContext = class extends Real {
    constructor(opts) {
      super(opts);
      super.suspend();
    }
    resume() {
      return navigator.userActivation.isActive ? super.resume() : new Promise(() => {});
    }
  };
})();`

// TestSoundCardUnlocksAndDismisses is GM-012 in a browser that blocks
// audio: the card shows, only its button turns sound on, and after a reload
// it asks to resume. ✕ keeps the board silent.
func TestSoundCardUnlocksAndDismisses(t *testing.T) {
	r := newRoom(t)
	board := r.openTab("board", r.server.URL+"/board", blockAudioJS)
	board.waitFor(`!document.getElementById("shell-sound").hidden && document.activeElement === document.querySelector("[data-sound-on]")`)
	r.waitStatus("Waiting for a tap")
	var title string
	board.eval(`document.querySelector("[data-sound-title]").textContent`, &title)
	if title != "Sound is off" {
		t.Fatalf("card title = %q", title)
	}

	// A tap elsewhere does not turn sound on.
	board.click("#shell-stage > :first-child")
	time.Sleep(300 * time.Millisecond)
	var state string
	board.eval(`grabbagAudio.board.audio.state`, &state)
	if state == "running" {
		t.Fatal("a tap on the board turned sound on")
	}

	board.click("[data-sound-on]")
	board.waitFor(`grabbagAudio.board.audio.state === "running" && document.getElementById("shell-sound").hidden`)
	r.waitStatus("<strong>On</strong>")

	reload, cancel := context.WithTimeout(board.ctx, 15*time.Second)
	defer cancel()
	if err := chromedp.Run(reload, chromedp.Reload()); err != nil {
		t.Fatal(err)
	}
	board.waitFor(`window.grabbagAudio && grabbagAudio.board && !document.getElementById("shell-sound").hidden`)
	board.eval(`document.querySelector("[data-sound-title]").textContent`, &title)
	if title != "Tap to resume sound" {
		t.Fatalf("card title after a reload = %q", title)
	}

	board.click("[data-sound-dismiss]")
	board.waitFor(`document.getElementById("shell-sound").hidden`)
	r.waitStatus("Turned off on the TV")
}

// TestAudioScheduling renders cues through an OfflineAudioContext and
// checks what is heard and when (GM-030 item 2).
func TestAudioScheduling(t *testing.T) {
	r := newRoom(t)
	board := r.open("board", "/board")
	board.waitFor(`!!window.grabbagAudio`)
	var results []struct {
		Name string `json:"name"`
		OK   bool   `json:"ok"`
		Msg  string `json:"msg"`
	}
	board.awaitJSON(schedulingHarness, &results)
	if len(results) != schedulingCases {
		t.Fatalf("%d scheduling cases ran, want %d: %+v", len(results), schedulingCases, results)
	}
	for _, res := range results {
		if !res.OK {
			t.Errorf("%s: %s", res.Name, res.Msg)
		}
	}
}

// schedulingCases is how many check() calls schedulingHarness makes.
const schedulingCases = 13

// schedulingHarness runs every case and returns [{name, ok, msg}] as JSON.
// render() suspends the offline context every 48 ms of render time, runs the
// case's actions due by then, and ticks the engine, since an offline
// context has no wall clock.
const schedulingHarness = `(async () => {
  const RATE = 8000;
  const FREQ = 500;
  const out = [];
  const near = (a, b, tol) => Math.abs(a - b) <= tol;

  // tone is a buffer of back-to-back segments, each a 500 Hz sine (or freq)
  // at its own amplitude. Phase runs from each segment's start, so stems that
  // start together line up.
  function tone(ctx, parts) {
    const total = parts.reduce((n, p) => n + p.secs, 0);
    const buf = ctx.createBuffer(1, Math.round(total * RATE), RATE);
    const data = buf.getChannelData(0);
    let at = 0;
    for (const p of parts) {
      const n = Math.round(p.secs * RATE);
      for (let i = 0; i < n; i++) {
        data[at + i] = p.amp * Math.sin(2 * Math.PI * (p.freq || FREQ) * i / RATE);
      }
      at += n;
    }
    return buf;
  }

  // amp is the sine amplitude heard between t1 and t2.
  function amp(data, t1, t2) {
    const a = Math.max(0, Math.floor(t1 * RATE));
    const b = Math.min(data.length, Math.floor(t2 * RATE));
    let sum = 0;
    for (let i = a; i < b; i++) {
      sum += data[i] * data[i];
    }
    return Math.sqrt(sum / Math.max(1, b - a)) * Math.SQRT2;
  }

  function seeded(seed) {
    let s = seed;
    return () => {
      s = (s * 1103515245 + 12345) % 2147483648;
      return s / 2147483648;
    };
  }

  async function render(seconds, plan, opts) {
    opts = opts || {};
    const ctx = new OfflineAudioContext(1, Math.ceil(seconds * RATE), RATE);
    const engine = grabbagAudio.create({ context: ctx, manual: true, random: seeded(opts.seed || 7) });
    engine.levels({ master: { volume: 100 }, music: { volume: 100 }, effects: { volume: 100 } });
    const root = document.createElement("div");
    const scope = engine.scope("case", root, "/static/");
    const actions = [];
    const env = { ctx, engine, root, audio: scope.api, scope, at: (t, fn) => actions.push({ t, fn }), log: [] };
    engine.tap((type, d) => env.log.push({ type, d, t: ctx.currentTime }));
    plan(env);
    const step = 384 / RATE;
    for (let t = step; t < seconds - step; t += step) {
      ctx.suspend(t).then(async () => {
        const now = ctx.currentTime;
        for (const a of actions) {
          if (!a.done && a.t <= now + 1e-9) {
            a.done = true;
            a.fn();
          }
        }
        await new Promise((r) => setTimeout(r, 0));
        engine.tick();
        await new Promise((r) => setTimeout(r, 0));
        ctx.resume();
      });
    }
    const rendered = await ctx.startRendering();
    env.data = rendered.getChannelData(0);
    return env;
  }

  async function check(name, fn) {
    try {
      const msg = await fn();
      out.push({ name, ok: !msg, msg: msg || "" });
    } catch (err) {
      out.push({ name, ok: false, msg: String(err && err.stack || err) });
    }
  }

  // Two parallel variants, 8 s loops at 120 bpm: calm 0.2, tense 0.6.
  function layered(ctx, extra) {
    const spec = {
      files: { m: tone(ctx, [{ secs: 8, amp: 0.2 }, { secs: 8, amp: 0.6 }, { secs: 8, amp: 0.1 }]) },
      regions: {
        calm: { file: "m", start: 0, end: 8 },
        tense: { file: "m", start: 8, end: 16 },
        drums: { file: "m", start: 16, end: 24 },
      },
      cues: {
        timer: {
          layer: "music", bpm: 120, beatsPerBar: 4,
          variants: { calm: ["calm"], tense: ["tense"] },
          channels: { drums: { region: "drums", volume: 0 } },
        },
      },
    };
    Object.assign(spec.cues.timer, extra || {});
    return spec;
  }

  await check("grid math", () => {
    const geo = { ls: 1, le: 9, len: 8, dur: 9 };
    const cases = [
      [10.3, 2, 12], [19.5, 2, 20], [12, 0, 19], [19.5, 0, 27], [18.1, 2, 19], [26.9, 0.5, 27],
    ];
    for (const [t, g, want] of cases) {
      const got = grabbagAudio.nextOnGrid(geo, 10, t, g);
      if (!near(got, want, 1e-6)) {
        return "nextOnGrid(" + t + ", " + g + ") = " + got + ", want " + want;
      }
    }
  });

  await check("variant switch lands on the next bar and keeps position", async () => {
    let started, landed;
    const env = await render(6, (e) => {
      e.audio.define(layered(e.ctx));
      e.at(0.1, () => { e.audio.cue("timer").start(); e.audio.cue("timer").started.then((t) => (started = t)); });
      e.at(1.0, () => e.audio.cue("timer").to("tense", { at: "bar", over: 0 }).then((t) => (landed = t)));
    });
    if (!(started > 0)) return "start never heard";
    if (!near(landed, started + 2, 0.002)) return "switch heard at " + landed + ", want bar 2 at " + (started + 2);
    const before = amp(env.data, landed - 0.4, landed - 0.02);
    const after = amp(env.data, landed + 0.02, landed + 0.4);
    if (!near(before, 0.2, 0.03) || !near(after, 0.6, 0.05)) return "levels around the switch " + before + " -> " + after;
  });

  await check("crossfade over a bar", async () => {
    let landed;
    const env = await render(6, (e) => {
      e.audio.define(layered(e.ctx));
      e.at(0.1, () => e.audio.cue("timer").start());
      e.at(1.0, () => e.audio.cue("timer").to("tense", { at: "bar", over: 2 }).then((t) => (landed = t)));
    });
    const mid = amp(env.data, landed + 0.95, landed + 1.05);
    const end = amp(env.data, landed + 2.1, landed + 2.5);
    if (!near(mid, 0.4, 0.05)) return "halfway through the crossfade heard " + mid + ", want 0.4";
    if (!near(end, 0.6, 0.05)) return "after the crossfade heard " + end;
  });

  await check("sequential variants wait for the loop and restart", async () => {
    let started, landed;
    const env = await render(7, (e) => {
      e.audio.define({
        files: { m: tone(e.ctx, [{ secs: 3, amp: 0.3 }, { secs: 5, amp: 0.5 }]) },
        regions: { a: { file: "m", start: 0, end: 3 }, b: { file: "m", start: 3, end: 8 } },
        cues: { song: { variants: { a: ["a"], b: ["b"] } } },
      });
      e.at(0.1, () => { e.audio.cue("song").start(); e.audio.cue("song").started.then((t) => (started = t)); });
      e.at(1.0, () => e.audio.cue("song").to("b", { at: "bar", over: 0 }).then((t) => (landed = t)));
    });
    if (!near(landed, started + 3, 0.002)) return "switch heard at " + landed + ", want the loop end " + (started + 3);
    const after = amp(env.data, landed + 0.05, landed + 1);
    if (!near(after, 0.5, 0.05)) return "after the switch heard " + after;
  });

  await check("end plays the outro on the next bar", async () => {
    let started, ended;
    const env = await render(7, (e) => {
      const spec = layered(e.ctx, { outro: "outro" });
      spec.files.o = tone(e.ctx, [{ secs: 1, amp: 0.45 }]);
      spec.regions.outro = { file: "o", start: 0, end: 1 };
      e.audio.define(spec);
      e.at(0.1, () => { e.audio.cue("timer").start(); e.audio.cue("timer").started.then((t) => (started = t)); });
      e.at(2.5, () => e.audio.cue("timer").end({ at: "bar" }).then((t) => (ended = t)));
    });
    if (!near(ended, started + 4, 0.002)) return "outro heard at " + ended + ", want " + (started + 4);
    const outro = amp(env.data, ended + 0.1, ended + 0.9);
    const after = amp(env.data, ended + 1.2, ended + 1.6);
    if (!near(outro, 0.45, 0.05) || after > 0.01) return "outro " + outro + ", after " + after;
    if (env.engine.inspect().playing.length) return "cue still listed after its outro";
  });

  await check("countdown catches up, then follows added time", async () => {
    const env = await render(9, (e) => {
      e.audio.define(layered(e.ctx, {
        timelines: {
          answer: { kind: "countdown", steps: {
            30: { to: "tense", over: 0, at: "now" },
            10: { fade: "drums", to: 1, over: 0, at: "now" },
          } },
        },
      }));
      const tick = (s) => e.root.dispatchEvent(new CustomEvent("grabbag:countdown-tick", { bubbles: true, detail: { seconds: s, kind: "answer" } }));
      e.at(0.1, () => e.audio.cue("timer").start({ timeline: "answer", countdown: { kind: "answer" }, seconds: 8 }));
      e.at(2.0, () => tick(40));
      e.at(5.0, () => tick(29));
    });
    const late = amp(env.data, 0.3, 1.5);
    const added = amp(env.data, 4.3, 4.9);
    const crossed = amp(env.data, 5.3, 6);
    if (!near(late, 0.7, 0.05)) return "a late start heard " + late + ", want tense plus drums 0.7";
    if (!near(added, 0.2, 0.05)) return "after time was added heard " + added + ", want calm 0.2";
    if (!near(crossed, 0.6, 0.05)) return "after crossing 30 again heard " + crossed + ", want tense 0.6";
  });

  await check("free timeline shuffles without repeats and holds", async () => {
    let held = false;
    const env = await render(40, (e) => {
      const spec = layered(e.ctx, {
        bpm: 240,
        timelines: {
          judging: { kind: "free", script: { 0: { to: "calm" } }, then: { from: 1, shuffle: [
            { to: "calm", bars: 1 }, { to: "tense", bars: 1 }, { fade: "drums", to: 0.5, bars: 1 },
          ] } },
        },
      });
      e.audio.define(spec);
      e.at(0.1, () => e.audio.cue("timer").start({ timeline: "judging", held: () => held }));
      e.at(20, () => (held = true));
      e.at(26, () => (held = false));
    }, { seed: 11 });
    const steps = env.log.filter((x) => x.type === "audio-step").map((x) => ({ at: x.d.at, key: JSON.stringify(x.d.step) }));
    const picks = steps.slice(1);
    if (picks.length < 25) return "only " + picks.length + " shuffle picks";
    for (let i = 1; i < picks.length; i++) {
      if (picks[i].key === picks[i - 1].key) return "the same step twice in a row at bar " + i;
    }
    if (new Set(picks.map((p) => p.key)).size !== 3) return "not every step was picked";
    if (steps.some((s) => s.at > 20.2 && s.at < 26)) return "a step ran while held";
  });

  await check("a step's sting and snapshot land on its bar", async () => {
    let started;
    const env = await render(4, (e) => {
      e.audio.define({
        files: {
          m: tone(e.ctx, [{ secs: 4, amp: 0.5, freq: 100 }]),
          fx: tone(e.ctx, [{ secs: 0.3, amp: 0.9, freq: 1500 }]),
        },
        regions: { a: { file: "m" }, blip: { file: "fx" } },
        stings: { blip: { region: "blip" } },
        cues: {
          song: {
            bpm: 120, variants: { a: ["a"] },
            timelines: { t: { kind: "free", script: { 1: { snapshot: "telephone", over: 0, sting: "blip" } } } },
          },
        },
      });
      e.at(0.1, () => { e.audio.cue("song").start({ timeline: "t" }); e.audio.cue("song").started.then((t) => (started = t)); });
    });
    const bar = started + 2;
    const before = amp(env.data, bar - 0.08, bar - 0.01);
    if (!near(before, 0.5, 0.05)) return "just before the bar heard " + before + ", want the unfiltered 0.5";
    let onset = -1;
    for (let i = Math.floor((bar - 0.3) * RATE); i < env.data.length; i++) {
      if (Math.abs(env.data[i]) > 0.7) { onset = i / RATE; break; }
    }
    if (!near(onset, bar, 0.005)) return "sting heard at " + onset + ", want the bar at " + bar;
  });

  await check("telephone snapshot filters the music layer", async () => {
    const env = await render(4, (e) => {
      e.audio.define({
        files: { m: tone(e.ctx, [{ secs: 4, amp: 0.5, freq: 100 }]) },
        regions: { a: { file: "m" } },
        cues: { song: { variants: { a: ["a"] } } },
      });
      e.at(0.1, () => e.audio.cue("song").start());
      e.at(1.0, () => e.audio.snapshot("telephone", 0));
      e.at(2.0, () => e.audio.snapshot(null, 0));
    });
    const before = amp(env.data, 0.5, 0.95);
    const on = amp(env.data, 1.3, 1.9);
    const off = amp(env.data, 2.4, 3);
    if (!near(before, 0.5, 0.05) || on > 0.25 || !near(off, 0.5, 0.05)) return "100 Hz before/on/off: " + [before, on, off].join(" / ");
  });

  await check("a new music cue crossfades the old one out", async () => {
    const env = await render(5, (e) => {
      e.audio.define({
        files: { m: tone(e.ctx, [{ secs: 2, amp: 0.4 }, { secs: 2, amp: 0.4, freq: 700 }]) },
        regions: { a: { file: "m", start: 0, end: 2 }, b: { file: "m", start: 2, end: 4 } },
        cues: { one: { variants: { a: ["a"] } }, two: { variants: { b: ["b"] } } },
      });
      e.at(0.1, () => e.audio.cue("one").start());
      e.at(1.0, () => e.audio.cue("two").start({ over: 1 }));
    });
    const after = amp(env.data, 2.3, 3);
    const playing = env.engine.inspect().playing.map((p) => p.cue);
    if (playing.join() !== "two") return "still playing: " + playing.join();
    if (!near(after, 0.4, 0.05)) return "after the crossfade heard " + after + ", want one cue at 0.4";
  });

  await check("beat, bar, and marker events", async () => {
    const markers = [];
    let started;
    const env = await render(5, (e) => {
      const spec = layered(e.ctx);
      spec.regions.calm.markers = { hit: 1.25 };
      e.audio.define(spec);
      e.root.addEventListener("grabbag:audio-marker", (ev) => markers.push(ev.detail.marker));
      e.at(0.1, () => { e.audio.cue("timer").start(); e.audio.cue("timer").started.then((t) => (started = t)); });
    });
    const beats = env.log.filter((x) => x.type === "grabbag:audio-beat");
    const bars = env.log.filter((x) => x.type === "grabbag:audio-bar");
    if (beats.length < 9 || beats.length > 10) return beats.length + " beats in about 4.8 s at 120 bpm";
    if (bars.length !== 3) return bars.length + " bars";
    if (markers.join() !== "hit") return "markers " + markers.join();
    const first = env.log.find((x) => x.type === "grabbag:audio-marker");
    if (!(first.t >= started + 1.25 - 0.001 && first.t < started + 1.25 + 0.06)) return "marker dispatched at " + first.t;
  });

  await check("stings stop at the voice cap", async () => {
    const env = await render(2, (e) => {
      e.audio.define({
        files: { fx: tone(e.ctx, [{ secs: 1.5, amp: 0.01 }]) },
        regions: { blip: { file: "fx" } },
        stings: { blip: { region: "blip" } },
      });
      e.at(0.1, () => { for (let i = 0; i < 40; i++) e.audio.sting("blip"); });
      e.at(0.5, () => (e.voices = e.engine.inspect().voices));
    });
    if (env.voices !== 32) return env.voices + " voices, want the cap of 32";
  });

  await check("missing files fail quietly", async () => {
    const errors = [];
    const saved = console.error;
    console.error = (...a) => errors.push(a.join(" "));
    let started = "pending", ended = "pending", changed = "pending";
    try {
      await render(1, (e) => {
        e.audio.define({ files: { x: "nope.ogg" }, regions: { a: { file: "x" } }, cues: { c: { variants: { a: ["a"] } } } });
        e.at(0.1, () => {
          e.audio.cue("c").start().started.then(() => (started = "ok"));
          e.audio.cue("c").to("b").then(() => (changed = "ok"));
          e.audio.cue("c").end().then(() => (ended = "ok"));
          e.audio.cue("missing").start();
          e.audio.sting("missing");
        });
      });
      await new Promise((r) => setTimeout(r, 300));
    } finally {
      console.error = saved;
    }
    if (started !== "ok" || ended !== "ok") return "promises " + started + " / " + ended;
    // A change queued while the cue loads settles when the load fails.
    let queued = "pending";
    await render(1, (e) => {
      e.audio.define({ files: { x: "nope.ogg" }, regions: { a: { file: "x" } }, cues: { c: { variants: { a: ["a"] } } } });
      e.at(0.1, () => {
        e.audio.cue("c").start();
        e.audio.cue("c").to("b").then(() => (queued = "ok"));
      });
    });
    await new Promise((r) => setTimeout(r, 300));
    if (changed !== "ok" || queued !== "ok") return "queued changes " + changed + " / " + queued;
    if (!errors.some((m) => m.includes("unknown cue missing"))) return "no console error for an unknown cue";
  });

  return JSON.stringify(out);
})()`
