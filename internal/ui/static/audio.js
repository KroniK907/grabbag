// grabbagAudio is the board's Web Audio engine. Only the /board shell loads
// it. Phones and static previews never do.
//
// The engine owns one AudioContext and a small mixer:
//
//   master <- music layer  <- cues on the music layer (one at a time)
//          <- effects layer <- stings, effects cues, the settings test sample
//
// Each layer and each cue channel has a strip: a snapshot gain, a high-pass
// and a low-pass filter. Snapshots (FMOD snapshots, Wwise States) ramp named
// sets of strip targets together.
//
// A tenant gets ctx.audio on its board mount. That is a scope: the files,
// regions, cues, stings, and snapshots it defined, and the cues it started.
// Unmount releases the scope: its cues fade to silence and its buffers go.
//
//   ctx.audio.define({files, regions, cues, stings, snapshots});
//   var cue = ctx.audio.cue("timer").start({timeline: "answer", countdown: el});
//   cue.to("tense", {at: "bar", over: 2});
//   cue.channel("drums").fadeTo(1, {at: "bar"});
//   cue.end({at: "now"}).then(showResults);
//   ctx.audio.sting("reveal");
//   ctx.audio.snapshot("telephone", 0.8);
//
// A cue is horizontal resequencing (intro, looping variants, outro) plus
// vertical layering (variants and channels that run in parallel). Changes
// can wait for a sync point: "now", "beat", "bar", {bars: N}, or "loop".
// Every quantized call returns a promise that resolves when the change is
// heard. Failures log to the console and resolve at once: a game never
// waits on audio.
//
// The engine fires grabbag:audio-beat, grabbag:audio-bar (cues with bpm),
// and grabbag:audio-marker (markers declared on regions) on the tenant root,
// corrected for output latency and dispatched on an animation frame.
//
// grabbagAudio.create({context, manual, random}) builds a detached engine.
// Tests pass an OfflineAudioContext with manual: true and call tick() at
// suspend points, because an offline context has no wall clock.
(function () {
  "use strict";

  if (window.grabbagAudio) {
    return;
  }

  // The shell's test mode wraps window timers and attributes them to the
  // tenant whose script is on the stack. The engine's own timers are not a
  // tenant's, so it keeps the originals.
  var native = {
    setInterval: window.setInterval.bind(window),
    clearInterval: window.clearInterval.bind(window),
    setTimeout: window.setTimeout.bind(window),
    raf: window.requestAnimationFrame ? window.requestAnimationFrame.bind(window) : null,
    add: EventTarget.prototype.addEventListener,
    remove: EventTarget.prototype.removeEventListener,
  };

  var LOOKAHEAD = 0.15; // seconds of events and timeline steps scheduled ahead
  var TICK_MS = 25;
  var START_DELAY = 0.04; // "now" lands this far ahead so the first samples are not late
  var EPS = 1e-6;
  var CROSSFADE = 1.5; // GM-027 default music crossfade
  var VOICE_CAP = 32; // GM-028 safety cap on overlapping stings
  var STEM_TOLERANCE = 0.005;
  var HP_OFF = 10;
  var LP_OFF = 20000;
  var Q_OFF = 1;

  var BUILTIN_SNAPSHOTS = {
    telephone: { music: { highpass: 300, lowpass: 3400 } },
  };

  var LAYERS = ["music", "effects"];

  function warn() {
    var args = Array.prototype.slice.call(arguments);
    args.unshift("grabbagAudio:");
    console.error.apply(console, args);
  }

  function resolved() {
    return Promise.resolve();
  }

  function num(v, fallback) {
    return typeof v === "number" && isFinite(v) ? v : fallback;
  }

  function keys(obj) {
    return obj ? Object.keys(obj) : [];
  }

  // ---- Param control ---------------------------------------------------------

  // Ctl is one AudioParam the engine automates. It remembers its last ramp,
  // so a new ramp can start from the value the old one reached, on browsers
  // without cancelAndHoldAtTime.
  function Ctl(param, value, exp) {
    this.param = param;
    this.exp = !!exp;
    this.v0 = value;
    this.v1 = value;
    this.t0 = 0;
    this.t1 = 0;
    param.value = value;
  }

  Ctl.prototype.valueAt = function (t) {
    if (t >= this.t1 || this.t1 <= this.t0) {
      return t >= this.t1 ? this.v1 : this.v0;
    }
    if (t <= this.t0) {
      return this.v0;
    }
    var f = (t - this.t0) / (this.t1 - this.t0);
    if (this.exp && this.v0 > 0 && this.v1 > 0) {
      return this.v0 * Math.pow(this.v1 / this.v0, f);
    }
    return this.v0 + (this.v1 - this.v0) * f;
  };

  // set moves to value, starting at time at and taking over seconds.
  Ctl.prototype.set = function (value, at, over) {
    var p = this.param;
    var from = this.valueAt(at);
    if (typeof p.cancelAndHoldAtTime === "function") {
      p.cancelAndHoldAtTime(at);
    } else {
      p.cancelScheduledValues(at);
    }
    p.setValueAtTime(from, at);
    if (over > 0) {
      if (this.exp && from > 0 && value > 0) {
        p.exponentialRampToValueAtTime(value, at + over);
      } else {
        p.linearRampToValueAtTime(value, at + over);
      }
    } else {
      p.setValueAtTime(value, at);
    }
    this.v0 = from;
    this.v1 = value;
    this.t0 = at;
    this.t1 = at + Math.max(0, over);
  };

  // ---- Strips ------------------------------------------------------------------

  // Strip is a fader into dest: fade gain -> snapshot gain -> high-pass ->
  // low-pass. Games fade the fade gain. Snapshots and filter calls own the
  // rest, so a snapshot never fights a variant crossfade.
  function Strip(audio, dest, volume) {
    this.input = audio.createGain();
    this.snap = audio.createGain();
    this.hp = audio.createBiquadFilter();
    this.lp = audio.createBiquadFilter();
    this.hp.type = "highpass";
    this.lp.type = "lowpass";
    // Stay under Nyquist: a biquad tuned right at it is unstable.
    var top = Math.min(LP_OFF, audio.sampleRate * 0.45);
    this.fade = new Ctl(this.input.gain, num(volume, 1));
    this.vol = new Ctl(this.snap.gain, 1);
    this.hpf = new Ctl(this.hp.frequency, HP_OFF, true);
    this.lpf = new Ctl(this.lp.frequency, top, true);
    this.hpq = new Ctl(this.hp.Q, Q_OFF);
    this.lpq = new Ctl(this.lp.Q, Q_OFF);
    this.top = top;
    this.input.connect(this.snap);
    this.snap.connect(this.hp);
    this.hp.connect(this.lp);
    this.lp.connect(dest);
  }

  // shape applies snapshot or filter values. Missing values go back to off
  // when reset is set, and are left alone otherwise.
  Strip.prototype.shape = function (v, at, over, reset) {
    v = v || {};
    var top = this.top;
    function pick(value, off) {
      if (typeof value === "number" && isFinite(value)) {
        return value;
      }
      return reset ? off : null;
    }
    var volume = pick(v.volume, 1);
    var hp = pick(v.highpass, HP_OFF);
    var lp = pick(v.lowpass, top);
    var q = pick(v.q, Q_OFF);
    if (volume !== null) {
      this.vol.set(Math.max(0, volume), at, over);
    }
    if (hp !== null) {
      this.hpf.set(Math.max(HP_OFF, Math.min(top, hp)), at, over);
    }
    if (lp !== null) {
      this.lpf.set(Math.max(HP_OFF, Math.min(top, lp)), at, over);
    }
    if (q !== null) {
      this.hpq.set(q, at, over);
      this.lpq.set(q, at, over);
    }
  };

  Strip.prototype.disconnect = function () {
    [this.input, this.snap, this.hp, this.lp].forEach(function (n) {
      try {
        n.disconnect();
      } catch (e) {}
    });
  };

  // ---- Region geometry --------------------------------------------------------

  // geometry resolves a region against its decoded buffer. Times in the
  // region definition are seconds; loopStart, loopEnd, and markers are
  // relative to the region start.
  function geometry(def, buffer) {
    var start = Math.max(0, num(def.start, 0));
    var end = Math.min(buffer.duration, num(def.end, buffer.duration));
    var dur = Math.max(0, end - start);
    var ls = Math.max(0, Math.min(dur, num(def.loopStart, 0)));
    var le = Math.max(ls, Math.min(dur, num(def.loopEnd, dur)));
    if (le - ls < 0.01) {
      ls = 0;
      le = dur;
    }
    return { buffer: buffer, start: start, dur: dur, ls: ls, le: le, len: le - ls, markers: def.markers || null };
  }

  // nextOnGrid is the first time at or after t where the playhead of a loop
  // that started at t0 sits on a multiple of g (seconds, measured from the
  // region start), or at the wrap point when g is 0 ("loop").
  function nextOnGrid(geo, t0, t, g) {
    var e = t - t0;
    if (e <= geo.le + EPS) {
      if (!g) {
        return t0 + geo.le;
      }
      return t0 + Math.min(Math.ceil(e / g - EPS) * g, geo.le);
    }
    var len = geo.len;
    if (!(len > 0)) {
      return t;
    }
    var k = Math.floor((e - geo.le) / len);
    var base = t0 + geo.le + k * len; // playhead just wrapped to ls here
    var pos = geo.ls + (e - geo.le - k * len);
    if (!g) {
      return base + len;
    }
    var c = Math.ceil(pos / g - EPS) * g;
    if (c > geo.le) {
      c = geo.le;
    }
    return base + (c - geo.ls);
  }

  // occurrences lists the times in (a, b] when a play crosses each position
  // (seconds from region start). A loop repeats [ls, le) after its first pass.
  function occurrences(play, positions, a, b, out) {
    var end = play.stopAt !== null ? Math.min(b, play.stopAt) : b;
    if (end <= a) {
      return;
    }
    var g = play.geo;
    positions.forEach(function (p) {
      var limit = play.loop ? g.le : g.dur;
      if (p.pos < limit) {
        var t = play.t0 + p.pos;
        if (t > a && t <= end) {
          out.push({ t: t, item: p });
        }
      }
      if (!play.loop || p.pos < g.ls || p.pos >= g.le || g.len <= 0) {
        return;
      }
      var first = play.t0 + g.le + (p.pos - g.ls);
      var k = Math.max(0, Math.ceil((a - first) / g.len - EPS));
      for (var t2 = first + k * g.len; t2 <= end; t2 += g.len) {
        if (t2 > a) {
          out.push({ t: t2, item: p });
        }
      }
    });
  }

  // ---- Engine ----------------------------------------------------------------------

  function Engine(opts) {
    opts = opts || {};
    var Ctor = window.AudioContext || window.webkitAudioContext;
    this.audio = opts.context || (Ctor ? new Ctor({ latencyHint: "interactive" }) : null);
    this.manual = !!opts.manual;
    this.random = typeof opts.random === "function" ? opts.random : Math.random;
    this.scopes = [];
    this.instances = [];
    this.voices = [];
    this.queue = [];
    this.music = null;
    this.snapshot = null; // {name, def, owner}
    this.taps = [];
    this.timer = null;
    this.framing = false;
    this.levelsState = {
      master: { volume: 80, muted: false },
      music: { volume: 60, muted: false },
      effects: { volume: 75, muted: false },
    };
    if (!this.audio) {
      return;
    }
    var a = this.audio;
    this.master = a.createGain();
    this.master.connect(a.destination);
    this.masterCtl = new Ctl(this.master.gain, 1);
    this.layers = {};
    this.levelCtl = {};
    var self = this;
    LAYERS.forEach(function (name) {
      var level = a.createGain();
      level.connect(self.master);
      self.levelCtl[name] = new Ctl(level.gain, 1);
      self.layers[name] = new Strip(a, level, 1);
    });
    this.applyLevels(0);
  }

  Engine.prototype.now = function () {
    return this.audio ? this.audio.currentTime : 0;
  };

  // heardTime is the context time now reaching the speakers.
  Engine.prototype.heardTime = function () {
    var a = this.audio;
    if (!a) {
      return Infinity;
    }
    if (this.manual) {
      return a.currentTime;
    }
    if (typeof a.getOutputTimestamp === "function") {
      var ts = a.getOutputTimestamp();
      if (ts && ts.contextTime > 0) {
        return ts.contextTime;
      }
    }
    return a.currentTime - (a.outputLatency || a.baseLatency || 0);
  };

  // ---- Levels --------------------------------------------------------------------

  function gainFor(level) {
    if (!level || level.muted) {
      return 0;
    }
    var v = Math.max(0, Math.min(100, num(level.volume, 0))) / 100;
    return v * v;
  }

  // levels applies the host's master, music, and effects settings.
  Engine.prototype.levels = function (value) {
    if (!value || typeof value !== "object") {
      return;
    }
    var self = this;
    ["master"].concat(LAYERS).forEach(function (name) {
      var v = value[name];
      if (v && typeof v === "object") {
        self.levelsState[name] = { volume: num(v.volume, self.levelsState[name].volume), muted: !!v.muted };
      }
    });
    this.applyLevels(0.08);
  };

  Engine.prototype.applyLevels = function (over) {
    if (!this.audio) {
      return;
    }
    var t = this.now();
    var s = this.levelsState;
    this.masterCtl.set(gainFor(s.master), t, over);
    var self = this;
    LAYERS.forEach(function (name) {
      self.levelCtl[name].set(gainFor(s[name]), t, over);
    });
  };

  // test plays the /settings sample on one layer: a short arpeggio on Music,
  // two blips on Effects. It needs no files.
  Engine.prototype.test = function (layer) {
    if (!this.audio || !this.layers[layer]) {
      return;
    }
    var a = this.audio;
    var t = this.now() + START_DELAY;
    var notes = layer === "music" ? [261.63, 329.63, 392, 523.25] : [880, 1318.5];
    var step = layer === "music" ? 0.22 : 0.12;
    var dest = this.layers[layer].input;
    this.emit(null, "audio-test", { layer: layer });
    notes.forEach(function (freq, i) {
      var osc = a.createOscillator();
      var env = a.createGain();
      osc.type = layer === "music" ? "triangle" : "square";
      osc.frequency.value = freq;
      var at = t + i * step;
      var len = layer === "music" ? 0.6 : 0.1;
      env.gain.setValueAtTime(0, at);
      env.gain.linearRampToValueAtTime(layer === "music" ? 0.3 : 0.15, at + 0.01);
      env.gain.exponentialRampToValueAtTime(0.0001, at + len);
      osc.connect(env);
      env.connect(dest);
      osc.start(at);
      osc.stop(at + len + 0.02);
      osc.onended = function () {
        env.disconnect();
      };
    });
  };

  // ---- Scheduler -------------------------------------------------------------------

  // wake starts the scheduler while something is playing or waiting.
  Engine.prototype.wake = function () {
    if (this.manual || !this.audio) {
      return;
    }
    this.frameLoop();
    if (this.timer) {
      return;
    }
    var self = this;
    this.timer = native.setInterval(function () {
      self.tick();
    }, TICK_MS);
  };

  Engine.prototype.idle = function () {
    return !this.queue.length && !this.instances.length;
  };

  // tick schedules events and timeline steps up to LOOKAHEAD ahead and
  // dispatches what is due. Tests call it by hand.
  Engine.prototype.tick = function () {
    var now = this.now();
    var horizon = now + LOOKAHEAD;
    var self = this;
    this.instances.slice().forEach(function (inst) {
      inst.scan(horizon);
    });
    this.flush();
    if (!this.manual && this.timer && this.idle()) {
      native.clearInterval(this.timer);
      this.timer = null;
    }
    return self;
  };

  Engine.prototype.frameLoop = function () {
    if (this.framing || !native.raf) {
      return;
    }
    this.framing = true;
    var self = this;
    function step() {
      self.flush();
      if (self.idle() || self.manual) {
        self.framing = false;
        return;
      }
      native.raf(step);
    }
    native.raf(step);
  };

  // at queues fn for when context time t is heard.
  Engine.prototype.at = function (t, fn) {
    var q = this.queue;
    var i = q.length;
    while (i > 0 && q[i - 1].t > t) {
      i--;
    }
    q.splice(i, 0, { t: t, fn: fn });
    this.wake();
  };

  // heard returns a promise that resolves with t when t reaches the
  // speakers.
  Engine.prototype.heard = function (t) {
    var self = this;
    if (!this.audio) {
      return resolved();
    }
    return new Promise(function (resolve) {
      self.at(t, function () {
        resolve(t);
      });
    });
  };

  Engine.prototype.flush = function () {
    var heard = this.heardTime();
    while (this.queue.length && this.queue[0].t <= heard + EPS) {
      var item = this.queue.shift();
      try {
        item.fn();
      } catch (err) {
        warn("event handler failed", err);
      }
    }
  };

  // emit sends a sync event to the bench panel taps and the tenant root.
  Engine.prototype.emit = function (root, type, detail) {
    this.taps.forEach(function (fn) {
      try {
        fn(type, detail);
      } catch (e) {}
    });
    if (root && type.indexOf("grabbag:") === 0) {
      root.dispatchEvent(new CustomEvent(type, { bubbles: true, detail: detail }));
    }
  };

  // tap adds a listener for every engine event, for the dev bench panel.
  Engine.prototype.tap = function (fn) {
    var taps = this.taps;
    taps.push(fn);
    return function () {
      var i = taps.indexOf(fn);
      if (i >= 0) {
        taps.splice(i, 1);
      }
    };
  };

  // ---- Snapshots -------------------------------------------------------------------

  // strips resolves a snapshot or filter target: "music" or "effects" (a
  // layer), "<cue>" (a playing cue's bus), or "<cue>.<channel>" (a variant or
  // channel of a playing cue).
  Engine.prototype.strips = function (target, scope) {
    if (this.layers && this.layers[target]) {
      return [this.layers[target]];
    }
    var parts = String(target).split(".");
    var out = [];
    this.instances.forEach(function (inst) {
      if (inst.name !== parts[0] || (scope && inst.scope !== scope)) {
        return;
      }
      if (parts.length === 1) {
        out.push(inst.bus);
      } else if (inst.strips[parts[1]]) {
        out.push(inst.strips[parts[1]]);
      }
    });
    return out;
  };

  // applySnapshot ramps every target the old and new snapshots touch over
  // seconds, starting now or at context time at. Targets the new one leaves
  // out go back to off.
  Engine.prototype.applySnapshot = function (name, seconds, scope, at) {
    if (!this.audio) {
      return resolved();
    }
    var def = null;
    if (name) {
      def = (scope && scope.snapshots[name]) || BUILTIN_SNAPSHOTS[name];
      if (!def) {
        warn("unknown snapshot", name);
        return resolved();
      }
    }
    var prev = this.snapshot ? this.snapshot.def : {};
    var next = def || {};
    var t = Math.max(this.now(), num(at, 0));
    var over = Math.max(0, num(seconds, 0.5));
    var self = this;
    var targets = {};
    keys(prev).concat(keys(next)).forEach(function (k) {
      targets[k] = true;
    });
    keys(targets).forEach(function (target) {
      self.strips(target, null).forEach(function (strip) {
        strip.shape(next[target], t, over, true);
      });
    });
    this.snapshot = def ? { name: name, def: def, owner: scope } : null;
    return this.heard(t + over);
  };

  // snapshotFor is the active snapshot's values for a new strip.
  Engine.prototype.snapshotFor = function (cue, part) {
    if (!this.snapshot) {
      return null;
    }
    var def = this.snapshot.def;
    return part ? def[cue + "." + part] : def[cue];
  };

  // ---- Stings ----------------------------------------------------------------------

  Engine.prototype.addVoice = function (src) {
    var voices = this.voices;
    voices.push(src);
    src.onended = function () {
      var i = voices.indexOf(src);
      if (i >= 0) {
        voices.splice(i, 1);
      }
      try {
        src.disconnect();
      } catch (e) {}
    };
    while (voices.length > VOICE_CAP) {
      var old = voices.shift();
      try {
        old.stop();
      } catch (e) {}
    }
  };

  // ---- Scopes --------------------------------------------------------------------

  // scope makes the ctx.audio a tenant gets for one mount. base is the
  // directory its file paths resolve against.
  Engine.prototype.scope = function (tenant, root, base) {
    var scope = new Scope(this, tenant, root, base);
    this.scopes.push(scope);
    return scope;
  };

  // leave fades every live scope's cues to silence over seconds. The shell
  // calls it when a transition starts fading the board out.
  Engine.prototype.leave = function (seconds) {
    this.scopes.forEach(function (scope) {
      scope.leave(seconds);
    });
  };

  Engine.prototype.inspect = function () {
    var self = this;
    return {
      state: this.audio ? this.audio.state : "unsupported",
      time: this.now(),
      levels: JSON.parse(JSON.stringify(this.levelsState)),
      snapshot: this.snapshot ? this.snapshot.name : null,
      voices: this.voices.length,
      scopes: this.scopes.map(function (scope) {
        return {
          tenant: scope.tenant,
          files: keys(scope.files).map(function (name) {
            return { name: name, state: scope.files[name].state };
          }),
          cues: keys(scope.cues).map(function (name) {
            var def = scope.cues[name];
            return {
              name: name,
              layer: def.layer || "music",
              variants: keys(def.variants),
              channels: keys(def.channels),
              timelines: keys(def.timelines).map(function (t) {
                return { name: t, kind: def.timelines[t].kind || "countdown" };
              }),
            };
          }),
          stings: keys(scope.stings),
          snapshots: keys(scope.snapshots).concat(keys(BUILTIN_SNAPSHOTS)),
        };
      }),
      playing: this.instances.map(function (inst) {
        return inst.report();
      }),
      current: self.scopes.length ? self.scopes[self.scopes.length - 1].tenant : null,
    };
  };

  // ---- Scope -----------------------------------------------------------------------

  function Scope(engine, tenant, root, base) {
    this.engine = engine;
    this.tenant = tenant;
    this.root = root;
    this.base = base || "";
    this.files = {};
    this.regions = {};
    this.cues = {};
    this.stings = {};
    this.snapshots = {};
    this.handles = {};
    this.instances = [];
    this.touched = {}; // layers this scope filtered directly
    this.musicStings = []; // playing music-layer stings, faded on leave
    this.leaving = false;
    this.released = false;
    this.api = this.makeAPI();
  }

  Scope.prototype.makeAPI = function () {
    var scope = this;
    return {
      define: function (spec) {
        scope.define(spec);
      },
      cue: function (name) {
        return scope.handle(name);
      },
      sting: function (name, opts) {
        return scope.sting(name, opts);
      },
      snapshot: function (name, seconds) {
        if (scope.released) {
          return resolved();
        }
        return scope.engine.applySnapshot(name, seconds, scope);
      },
      filter: function (target, values, opts) {
        return scope.filter(target, values, opts);
      },
      stop: function (opts) {
        scope.instances.slice().forEach(function (inst) {
          inst.stop(opts);
        });
      },
      // ready resolves when every defined file has loaded or failed.
      ready: function () {
        return Promise.all(keys(scope.files).map(function (n) {
          return scope.files[n].promise.catch(function () {});
        })).then(function () {});
      },
    };
  };

  Scope.prototype.define = function (spec) {
    if (this.released || !spec) {
      return;
    }
    var self = this;
    keys(spec.files).forEach(function (name) {
      self.loadFile(name, spec.files[name]);
    });
    Object.assign(this.regions, spec.regions || {});
    Object.assign(this.cues, spec.cues || {});
    Object.assign(this.stings, spec.stings || {});
    Object.assign(this.snapshots, spec.snapshots || {});
    keys(spec.regions).forEach(function (name) {
      var r = spec.regions[name];
      if (!r || !self.files[r.file]) {
        warn(self.tenant + ": region " + name + " names unknown file " + (r && r.file));
      }
    });
    keys(spec.cues).forEach(function (name) {
      self.checkCue(name, spec.cues[name]);
    });
    keys(spec.stings).forEach(function (name) {
      var s = spec.stings[name];
      if (!s || !self.regions[s.region]) {
        warn(self.tenant + ": sting " + name + " names unknown region " + (s && s.region));
      }
    });
  };

  // checkCue logs unknown regions now and stems of different lengths once
  // the files decode (GM-017).
  Scope.prototype.checkCue = function (name, def) {
    var self = this;
    var label = this.tenant + ": cue " + name;
    var names = this.cueRegions(def);
    names.forEach(function (r) {
      if (!self.regions[r]) {
        warn(label + " names unknown region " + r);
      }
    });
    if (!keys(def.variants).length) {
      warn(label + " has no variants");
      return;
    }
    this.load(names).then(function (ok) {
      if (!ok) {
        return;
      }
      var groups = keys(def.variants).map(function (v) {
        return [].concat(def.variants[v]).concat(keys(def.channels).map(function (c) {
          return def.channels[c].region;
        }));
      });
      groups.forEach(function (group) {
        var lens = group.map(function (r) {
          return self.geo(r).len;
        });
        var lo = Math.min.apply(null, lens);
        var hi = Math.max.apply(null, lens);
        if (hi - lo > STEM_TOLERANCE) {
          warn(label + ": stems that play together differ in loop length (" + group.join(", ") + ": " +
            lens.map(function (l) { return l.toFixed(3); }).join(", ") + ")");
        }
      });
    });
  };

  Scope.prototype.cueRegions = function (def) {
    var out = [];
    if (def.intro) {
      out.push(def.intro);
    }
    if (def.outro) {
      out.push(def.outro);
    }
    keys(def.variants).forEach(function (v) {
      out = out.concat(def.variants[v]);
    });
    keys(def.channels).forEach(function (c) {
      out.push(def.channels[c].region);
    });
    return out;
  };

  Scope.prototype.loadFile = function (name, src) {
    var engine = this.engine;
    var have = this.files[name];
    if (have && have.src === src && have.state !== "failed") {
      return; // a repeat define, e.g. after a same-tenant swap
    }
    var entry = { state: "loading", buffer: null, promise: null, src: src };
    this.files[name] = entry;
    var label = this.tenant + ": file " + name;
    if (!engine.audio) {
      entry.state = "failed";
      entry.promise = Promise.reject(new Error("no Web Audio"));
      entry.promise.catch(function () {});
      return;
    }
    var job;
    if (typeof AudioBuffer !== "undefined" && src instanceof AudioBuffer) {
      job = Promise.resolve(src);
    } else {
      var url = new URL(String(src), new URL(this.base || "./", location.href)).href;
      job = fetch(url, { credentials: "same-origin" })
        .then(function (res) {
          if (!res.ok) {
            throw new Error(url + " returned " + res.status);
          }
          return res.arrayBuffer();
        })
        .then(function (bytes) {
          return new Promise(function (resolve, reject) {
            engine.audio.decodeAudioData(bytes, resolve, reject);
          });
        });
    }
    entry.promise = job.then(
      function (buffer) {
        entry.state = "ready";
        entry.buffer = buffer;
        return buffer;
      },
      function (err) {
        entry.state = "failed";
        warn(label + " did not load", err);
        throw err;
      }
    );
    entry.promise.catch(function () {});
  };

  // load resolves true once every region's file is decoded, or false if
  // any region or file is missing.
  Scope.prototype.load = function (regionNames) {
    var self = this;
    var jobs = [];
    for (var i = 0; i < regionNames.length; i++) {
      var r = this.regions[regionNames[i]];
      var f = r && this.files[r.file];
      if (!f) {
        return Promise.resolve(false);
      }
      jobs.push(f.promise);
    }
    return Promise.all(jobs).then(
      function () {
        return !self.released;
      },
      function () {
        return false;
      }
    );
  };

  Scope.prototype.geo = function (regionName) {
    var r = this.regions[regionName];
    return geometry(r, this.files[r.file].buffer);
  };

  Scope.prototype.handle = function (name) {
    if (!this.handles[name]) {
      this.handles[name] = new CueHandle(this, name);
    }
    return this.handles[name];
  };

  Scope.prototype.sting = function (name, opts) {
    opts = opts || {};
    var engine = this.engine;
    var def = this.stings[name];
    if (this.released || this.leaving) {
      return resolved();
    }
    if (!def || !this.regions[def.region]) {
      warn(this.tenant + ": unknown sting " + name);
      return resolved();
    }
    var layer = engine.layers && engine.layers[def.layer || "effects"];
    if (!layer) {
      warn(this.tenant + ": sting " + name + " has unknown layer " + def.layer);
      return resolved();
    }
    var self = this;
    return this.load([def.region]).then(function (ok) {
      if (!ok || self.released) {
        return;
      }
      var a = engine.audio;
      var geo = self.geo(def.region);
      var t = Math.max(engine.now() + START_DELAY, num(opts.at, 0));
      var src = a.createBufferSource();
      src.buffer = geo.buffer;
      var gain = a.createGain();
      gain.gain.value = num(opts.volume, num(def.volume, 1));
      src.connect(gain);
      gain.connect(layer.input);
      src.start(t, geo.start, geo.dur);
      engine.addVoice(src);
      var voice = { src: src, gain: gain };
      var music = (def.layer || "effects") === "music";
      if (music) {
        self.musicStings.push(voice);
      }
      var ended = src.onended;
      src.onended = function () {
        ended();
        gain.disconnect();
        if (music) {
          var i = self.musicStings.indexOf(voice);
          if (i >= 0) {
            self.musicStings.splice(i, 1);
          }
        }
      };
      if (geo.markers) {
        keys(geo.markers).forEach(function (m) {
          engine.at(t + num(geo.markers[m], 0), function () {
            engine.emit(self.root, "grabbag:audio-marker", { sting: name, marker: m });
          });
        });
      }
      return engine.heard(t);
    });
  };

  Scope.prototype.filter = function (target, values, opts) {
    opts = opts || {};
    var engine = this.engine;
    if (this.released || !engine.audio) {
      return resolved();
    }
    var strips = engine.strips(target, this);
    if (!strips.length) {
      warn(this.tenant + ": nothing to filter at " + target);
      return resolved();
    }
    if (engine.layers[target]) {
      this.touched[target] = true;
    }
    var t = engine.now();
    var over = Math.max(0, num(opts.over, 0));
    strips.forEach(function (s) {
      s.shape(values, t, over, false);
    });
    return engine.heard(t);
  };

  Scope.prototype.leave = function (seconds) {
    if (this.released || this.leaving) {
      return;
    }
    this.leaving = true;
    var over = Math.max(0.05, num(seconds, CROSSFADE));
    this.instances.slice().forEach(function (inst) {
      inst.stop({ over: over });
    });
    this.fadeMusicStings(over);
  };

  // fadeMusicStings fades the music-layer stings still playing, so a one-shot
  // theme crossfades with the next tenant's music like a cue does. Effects
  // stings play out.
  Scope.prototype.fadeMusicStings = function (over) {
    var t = this.engine.now();
    this.musicStings.slice().forEach(function (voice) {
      var g = voice.gain.gain;
      g.cancelScheduledValues(t);
      g.setValueAtTime(g.value, t);
      g.linearRampToValueAtTime(0, t + over);
      try {
        voice.src.stop(t + over);
      } catch (e) {}
    });
  };

  // release runs on unmount: cues fade to silence (GM-027), timeline
  // listeners go, a snapshot this tenant set clears, and buffers drop.
  Scope.prototype.release = function () {
    if (this.released) {
      return;
    }
    this.released = true;
    var engine = this.engine;
    this.instances.slice().forEach(function (inst) {
      inst.stop({ over: CROSSFADE });
    });
    if (!this.leaving) {
      this.fadeMusicStings(CROSSFADE);
    }
    if (engine.snapshot && engine.snapshot.owner === this) {
      engine.applySnapshot(null, 0.5, null);
    }
    // Layers are shared. Put back what this tenant filtered by hand.
    var t = engine.now();
    keys(this.touched).forEach(function (layer) {
      var snap = engine.snapshot ? engine.snapshot.def[layer] : null;
      engine.layers[layer].shape(snap, t, 0.5, true);
    });
    this.files = {};
    this.regions = {};
    this.cues = {};
    this.stings = {};
    this.snapshots = {};
    var i = engine.scopes.indexOf(this);
    if (i >= 0) {
      engine.scopes.splice(i, 1);
    }
  };

  // ---- Cue handle ------------------------------------------------------------------

  // CueHandle is what ctx.audio.cue(name) returns. It drives the cue's
  // latest instance. Calls on a cue that is not playing resolve at once.
  function CueHandle(scope, name) {
    this.scope = scope;
    this.name = name;
    this.inst = null;
    this.started = resolved();
    this.finished = resolved();
  }

  CueHandle.prototype.start = function (opts) {
    var scope = this.scope;
    var def = scope.cues[this.name];
    if (scope.released || scope.leaving) {
      return this;
    }
    if (!def) {
      warn(scope.tenant + ": unknown cue " + this.name);
      return this;
    }
    if (!scope.engine.audio) {
      return this;
    }
    // A music cue crossfades from whatever holds the music slot, this cue
    // included. Other layers stop their previous instance here.
    if (this.inst && this.inst.live() && (def.layer || "music") !== "music") {
      this.inst.stop({ over: num(opts && opts.over, CROSSFADE) });
    }
    var inst = new Instance(scope, this.name, def, opts || {});
    this.inst = inst;
    this.started = inst.startedPromise;
    this.finished = inst.finishedPromise;
    return this;
  };

  CueHandle.prototype.to = function (variant, opts) {
    return this.inst ? this.inst.to(variant, opts || {}) : resolved();
  };

  CueHandle.prototype.end = function (opts) {
    return this.inst ? this.inst.end(opts || {}) : resolved();
  };

  CueHandle.prototype.stop = function (opts) {
    return this.inst ? this.inst.stop(opts || {}) : resolved();
  };

  CueHandle.prototype.channel = function (name) {
    var self = this;
    return {
      fadeTo: function (volume, opts) {
        return self.inst ? self.inst.fadeChannel(name, volume, opts || {}) : resolved();
      },
      filter: function (values, opts) {
        return self.inst ? self.inst.filterPart(name, values, opts || {}) : resolved();
      },
    };
  };

  CueHandle.prototype.filter = function (values, opts) {
    return this.inst ? this.inst.filterPart(null, values, opts || {}) : resolved();
  };

  Object.defineProperty(CueHandle.prototype, "playing", {
    get: function () {
      return !!(this.inst && this.inst.live());
    },
  });

  // ---- Instance --------------------------------------------------------------------

  // Instance is one playing cue. States: loading, playing, ending, stopping,
  // done.
  function Instance(scope, name, def, opts) {
    var engine = scope.engine;
    var self = this;
    this.scope = scope;
    this.engine = engine;
    this.name = name;
    this.def = def;
    this.opts = opts;
    this.layerName = def.layer || "music";
    this.state = "loading";
    this.sources = [];
    this.plays = [];
    this.strips = {};
    this.parts = {};
    this.variant = null;
    this.seg = null; // {geo, t0} the grid the sync points follow
    this.scanned = 0;
    this.timeline = null;
    this.step = null;
    this.stopAt = null;
    this.pending = []; // calls made while loading
    this.listeners = [];
    var finish;
    this.finishedPromise = new Promise(function (resolve) {
      finish = resolve;
    });
    this.finish = finish;
    var started;
    this.startedPromise = new Promise(function (resolve) {
      started = resolve;
    });
    engine.instances.push(this);
    scope.instances.push(this);
    var layer = engine.layers[this.layerName];
    if (!layer) {
      warn(scope.tenant + ": cue " + name + " has unknown layer " + this.layerName);
      this.done();
      started();
      return;
    }
    // A music cue takes the music slot now, so a second start before this
    // one loads still crossfades from it.
    var prev = null;
    if (this.layerName === "music") {
      prev = engine.music;
      engine.music = this;
    }
    scope.load(scope.cueRegions(def)).then(function (ok) {
      if (!ok || self.state !== "loading") {
        if (self.state === "loading") {
          self.done();
          if (prev && prev.live() && !engine.music) {
            engine.music = prev;
          }
        }
        started();
        return;
      }
      var t = self.begin(prev, layer);
      started(engine.heard(t));
    });
  }

  Instance.prototype.live = function () {
    return this.state === "loading" || this.state === "playing";
  };

  Instance.prototype.gridFor = function (at) {
    var def = this.def;
    if (at === "loop" || !def.bpm) {
      return 0;
    }
    var beat = 60 / def.bpm;
    var bar = beat * (def.beatsPerBar || 4);
    if (at === "beat") {
      return beat;
    }
    if (at === "bar") {
      return bar;
    }
    if (at && typeof at === "object" && at.bars > 0) {
      return bar * at.bars;
    }
    return 0;
  };

  // when is the context time of the next sync point for at.
  Instance.prototype.when = function (at) {
    var t = this.engine.now() + START_DELAY;
    if (!at || at === "now" || !this.seg) {
      return t;
    }
    if (t < this.seg.t0) {
      return this.seg.t0; // during the intro, changes land on the loop start
    }
    return nextOnGrid(this.seg.geo, this.seg.t0, t, this.gridFor(at));
  };

  Instance.prototype.barLength = function () {
    if (this.def.bpm) {
      return (60 / this.def.bpm) * (this.def.beatsPerBar || 4);
    }
    return this.seg ? this.seg.geo.len : 0;
  };

  Instance.prototype.newStrip = function (key, volume) {
    var strip = new Strip(this.engine.audio, this.bus.input, volume);
    var snap = this.engine.snapshotFor(this.name, key);
    if (snap) {
      strip.shape(snap, this.engine.now(), 0, true);
    }
    return strip;
  };

  Instance.prototype.source = function (geo, dest, t, loop) {
    var a = this.engine.audio;
    var src = a.createBufferSource();
    src.buffer = geo.buffer;
    if (loop) {
      src.loop = true;
      src.loopStart = geo.start + geo.ls;
      src.loopEnd = geo.start + geo.le;
      src.start(t, geo.start);
    } else {
      src.start(t, geo.start, geo.dur);
    }
    src.connect(dest);
    var rec = { src: src, stopAt: null };
    this.sources.push(rec);
    return rec;
  };

  Instance.prototype.addPlay = function (geo, t0, loop, beats, region) {
    var play = { geo: geo, t0: t0, loop: loop, beats: beats, region: region, stopAt: null };
    this.plays.push(play);
    return play;
  };

  // parallel reports whether every variant and channel can run at once:
  // same lead-in and loop length, so switching keeps the playhead.
  Instance.prototype.parallel = function () {
    var scope = this.scope;
    var def = this.def;
    var all = [];
    keys(def.variants).forEach(function (v) {
      all = all.concat(def.variants[v]);
    });
    keys(def.channels).forEach(function (c) {
      all.push(def.channels[c].region);
    });
    var first = scope.geo(all[0]);
    return all.every(function (r) {
      var g = scope.geo(r);
      return Math.abs(g.len - first.len) <= STEM_TOLERANCE && Math.abs(g.ls - first.ls) <= STEM_TOLERANCE;
    });
  };

  // begin schedules the cue. It returns the time it is first heard.
  Instance.prototype.begin = function (prev, layer) {
    var engine = this.engine;
    var scope = this.scope;
    var def = this.def;
    var opts = this.opts;
    var a = engine.audio;
    var crossing = prev && prev.live() && prev !== this;
    var t = crossing ? prev.when(opts.at) : engine.now() + START_DELAY;
    var over = Math.max(0, num(opts.over, crossing ? CROSSFADE : 0));
    this.state = "playing";
    this.startAt = t;
    this.scanned = t - EPS;

    this.out = a.createGain();
    this.outCtl = new Ctl(this.out.gain, over > 0 ? 0 : 1);
    if (over > 0) {
      this.outCtl.set(1, t, over);
    }
    this.out.connect(layer.input);
    this.bus = new Strip(a, this.out, 1);
    var busSnap = engine.snapshotFor(this.name, null);
    if (busSnap) {
      this.bus.shape(busSnap, engine.now(), 0, true);
    }
    if (crossing) {
      prev.stop({ over: Math.max(over, 0.05), at: t });
    }

    var loopStart = t;
    if (def.intro) {
      var ig = scope.geo(def.intro);
      var introStrip = this.newStrip("intro", 1);
      this.strips.intro = introStrip;
      this.source(ig, introStrip.input, t, false);
      this.addPlay(ig, t, false, true, def.intro);
      loopStart = t + ig.dur;
    }

    var names = keys(def.variants);
    var timeline = opts.timeline ? (def.timelines || {})[opts.timeline] : null;
    if (opts.timeline && !timeline) {
      warn(scope.tenant + ": cue " + this.name + " has no timeline " + opts.timeline);
    }
    // A countdown timeline that started late applies every step already
    // passed before the cue starts, without replaying the transitions.
    var plan = { variant: opts.variant || def.variant || names[0], channels: {} };
    keys(def.channels).forEach(function (c) {
      plan.channels[c] = num(def.channels[c].volume, 1);
    });
    var countdown = null;
    if (timeline && (timeline.kind || "countdown") === "countdown") {
      countdown = new Countdown(this, timeline, opts);
      countdown.plan(plan);
    }
    if (!def.variants[plan.variant]) {
      warn(scope.tenant + ": cue " + this.name + " has no variant " + plan.variant);
      plan.variant = names[0];
    }
    this.variant = plan.variant;
    this.mode = this.parallel() ? "parallel" : "sequential";
    this.startLoop(loopStart, plan);

    if (countdown) {
      this.timeline = countdown;
      countdown.listen();
    } else if (timeline && timeline.kind === "free") {
      this.timeline = new Free(this, timeline, opts);
    }
    this.timelineName = timeline ? opts.timeline : null;

    var pending = this.pending;
    this.pending = [];
    pending.forEach(function (p) {
      p.run();
    });
    engine.wake();
    return t;
  };

  // startLoop starts the looping variants and channels at t0. In parallel
  // mode every variant runs and only the current one is up.
  Instance.prototype.startLoop = function (t0, plan) {
    var self = this;
    var scope = this.scope;
    var def = this.def;
    var parts = {};
    var clockSet = false;
    function run(key, regions, volume) {
      var strip = self.newStrip(key, volume);
      self.strips[key] = strip;
      var part = { strip: strip, recs: [], plays: [] };
      regions.forEach(function (r) {
        var g = scope.geo(r);
        part.recs.push(self.source(g, strip.input, t0, true));
        // The first region of the current variant drives beats and bars.
        var clock = !clockSet && key === plan.variant;
        if (clock) {
          clockSet = true;
          self.seg = { geo: g, t0: t0 };
        }
        part.plays.push(self.addPlay(g, t0, true, clock, r));
      });
      parts[key] = part;
    }
    var variants = this.mode === "parallel" ? keys(def.variants) : [plan.variant];
    variants.forEach(function (v) {
      run(v, [].concat(def.variants[v]), v === plan.variant ? 1 : 0);
    });
    keys(def.channels).forEach(function (c) {
      run(c, [def.channels[c].region], plan.channels[c]);
    });
    this.parts = parts;
  };

  Instance.prototype.stopPart = function (part, t, over) {
    var end = t + Math.max(over, 0.01);
    part.strip.fade.set(0, t, Math.max(over, 0.01));
    part.recs.forEach(function (rec) {
      rec.src.stop(end + 0.02);
      rec.stopAt = end;
    });
    part.plays.forEach(function (p) {
      p.stopAt = end;
    });
    this.engine.at(end + 0.05, function () {
      part.strip.disconnect();
    });
  };

  // later runs fn now, or once the cue starts if it is still loading. If the
  // cue is dropped before it starts, the promise resolves with nothing.
  Instance.prototype.later = function (fn) {
    if (this.state === "loading") {
      var self = this;
      return new Promise(function (resolve) {
        self.pending.push({
          run: function () {
            resolve(fn());
          },
          skip: function () {
            resolve();
          },
        });
      });
    }
    if (this.state !== "playing") {
      return resolved();
    }
    return fn();
  };

  // to switches variant (GM-020). Parallel variants crossfade in place.
  // Otherwise the new variant starts from its beginning at the sync point.
  Instance.prototype.to = function (variant, opts) {
    var self = this;
    return this.later(function () {
      return self.switchTo(variant, opts.at || "bar", num(opts.over, 1), false);
    });
  };

  // switchTo lands on the sync point at, or on a context time when at is a
  // number. instant applies the change now with no fade.
  Instance.prototype.switchTo = function (variant, at, over, instant) {
    var def = this.def;
    if (!def.variants[variant]) {
      warn(this.scope.tenant + ": cue " + this.name + " has no variant " + variant);
      return resolved();
    }
    if (variant === this.variant) {
      return resolved();
    }
    var engine = this.engine;
    var t = instant ? engine.now() : typeof at === "number" ? at : this.when(at);
    if (instant) {
      over = 0;
    }
    var old = this.variant;
    this.variant = variant;
    if (this.mode === "parallel") {
      this.parts[old].strip.fade.set(0, t, over);
      this.parts[variant].strip.fade.set(1, t, over);
      var self = this;
      this.parts[old].plays.forEach(function (p) {
        p.beats = false;
      });
      this.parts[variant].plays.forEach(function (p, i) {
        p.beats = i === 0;
        if (i === 0) {
          self.seg = { geo: p.geo, t0: p.t0 };
        }
      });
      return engine.heard(t);
    }
    // Sequential: the new variant and the channels restart at t.
    var volumes = {};
    var parts = this.parts;
    var self2 = this;
    if (t < this.seg.t0) {
      t = this.seg.t0;
    }
    keys(parts).forEach(function (key) {
      if (def.channels && def.channels[key]) {
        volumes[key] = parts[key].strip.fade.valueAt(t);
      }
      self2.stopPart(parts[key], t, over);
    });
    keys(def.channels).forEach(function (c) {
      if (!(c in volumes)) {
        volumes[c] = num(def.channels[c].volume, 1);
      }
    });
    this.startLoop(t, { variant: variant, channels: volumes });
    if (over > 0) {
      keys(this.parts).forEach(function (key) {
        var s = self2.parts[key].strip;
        var target = s.fade.v1;
        s.fade.set(0, engine.now(), 0);
        s.fade.set(target, t, over);
      });
    }
    return engine.heard(t);
  };

  Instance.prototype.fadeChannel = function (name, volume, opts) {
    var self = this;
    return this.later(function () {
      var part = self.parts[name];
      if (!part || !self.def.channels || !self.def.channels[name]) {
        warn(self.scope.tenant + ": cue " + self.name + " has no channel " + name);
        return resolved();
      }
      var t = opts.instant ? self.engine.now() : self.when(opts.at || "now");
      part.strip.fade.set(Math.max(0, num(volume, 1)), t, opts.instant ? 0 : Math.max(0, num(opts.over, 1)));
      return self.engine.heard(t);
    });
  };

  // filterPart sets filters on one variant or channel, or the whole cue
  // when name is null.
  Instance.prototype.filterPart = function (name, values, opts) {
    var self = this;
    return this.later(function () {
      var strip = name ? self.strips[name] : self.bus;
      if (!strip) {
        warn(self.scope.tenant + ": cue " + self.name + " has no part " + name);
        return resolved();
      }
      var t = self.when(opts.at || "now");
      strip.shape(values, t, Math.max(0, num(opts.over, 0)), false);
      return self.engine.heard(t);
    });
  };

  // end moves to the outro at the sync point, or fades out when there is
  // none. The promise resolves when the ending is heard; finished resolves
  // when the cue is silent.
  Instance.prototype.end = function (opts) {
    var self = this;
    if (this.state === "loading") {
      this.done();
      return resolved();
    }
    if (this.state !== "playing") {
      return resolved();
    }
    var def = this.def;
    var at = opts.at || (def.end && def.end.at) || "bar";
    var engine = this.engine;
    var t = this.when(at);
    this.state = "ending";
    this.stopTimeline();
    if (engine.music === this) {
      engine.music = null;
    }
    var stopAt;
    if (def.outro && this.scope.regions[def.outro]) {
      var og = this.scope.geo(def.outro);
      var cut = 0.02;
      keys(this.parts).forEach(function (key) {
        self.stopPart(self.parts[key], t, cut);
      });
      if (this.strips.intro) {
        this.strips.intro.fade.set(0, t, cut);
      }
      var outro = this.newStrip("outro", 1);
      this.strips.outro = outro;
      this.source(og, outro.input, t, false);
      this.addPlay(og, t, false, true, def.outro);
      stopAt = t + og.dur;
    } else {
      var over = Math.max(0.02, num(opts.over, num(def.end && def.end.over, 0.5)));
      this.outCtl.set(0, t, over);
      stopAt = t + over;
    }
    this.finishAt(stopAt);
    return engine.heard(t);
  };

  // stop fades the whole cue to silence, starting now or at opts.at (a
  // context time, used by crossfades).
  Instance.prototype.stop = function (opts) {
    opts = opts || {};
    if (this.state === "loading") {
      this.done();
      return resolved();
    }
    if (this.state === "done" || this.state === "stopping") {
      return resolved();
    }
    var engine = this.engine;
    var t = typeof opts.at === "number" ? opts.at : engine.now();
    var over = Math.max(0.02, num(opts.over, CROSSFADE));
    if (this.state === "ending" && this.stopAt !== null && this.stopAt <= t + over) {
      return resolved();
    }
    this.state = "stopping";
    this.stopTimeline();
    if (engine.music === this) {
      engine.music = null;
    }
    this.outCtl.set(0, t, over);
    this.finishAt(t + over);
    return engine.heard(t);
  };

  Instance.prototype.finishAt = function (t) {
    var self = this;
    this.stopAt = t;
    this.sources.forEach(function (rec) {
      if (rec.stopAt === null || rec.stopAt > t) {
        rec.src.stop(t + 0.02);
        rec.stopAt = t;
      }
    });
    this.plays.forEach(function (p) {
      if (p.stopAt === null || p.stopAt > t) {
        p.stopAt = t;
      }
    });
    this.engine.at(t + 0.05, function () {
      self.done();
    });
  };

  Instance.prototype.stopTimeline = function () {
    if (this.timeline) {
      this.timeline.stop();
    }
  };

  // done disconnects the cue and drops it, which lets its buffers go once
  // the scope is released.
  Instance.prototype.done = function () {
    if (this.state === "done") {
      return;
    }
    this.state = "done";
    this.stopTimeline();
    var engine = this.engine;
    if (engine.music === this) {
      engine.music = null;
    }
    var self = this;
    keys(this.strips).forEach(function (k) {
      self.strips[k].disconnect();
    });
    if (this.bus) {
      this.bus.disconnect();
    }
    if (this.out) {
      try {
        this.out.disconnect();
      } catch (e) {}
    }
    this.sources = [];
    this.plays = [];
    [engine.instances, this.scope.instances].forEach(function (list) {
      var i = list.indexOf(self);
      if (i >= 0) {
        list.splice(i, 1);
      }
    });
    // Changes queued while loading settle as no-ops, so a caller waiting
    // on one is never left hanging.
    var pending = this.pending;
    this.pending = [];
    pending.forEach(function (p) {
      p.skip();
    });
    this.finish();
  };

  // scan queues beat, bar, and marker events and runs timeline steps up to
  // horizon.
  Instance.prototype.scan = function (horizon) {
    if (this.state === "loading" || this.state === "done") {
      return;
    }
    var from = this.scanned;
    if (horizon <= from) {
      return;
    }
    this.scanned = horizon;
    var engine = this.engine;
    var root = this.scope.root;
    var name = this.name;
    var def = this.def;
    var hits = [];
    var beat = def.bpm ? 60 / def.bpm : 0;
    var per = def.beatsPerBar || 4;
    this.plays.forEach(function (play) {
      var positions = [];
      if (play.beats && beat) {
        var limit = play.loop ? play.geo.le : play.geo.dur;
        for (var i = 0; i * beat < limit - EPS; i++) {
          positions.push({ kind: "beat", pos: i * beat, index: i });
        }
      }
      keys(play.geo.markers).forEach(function (m) {
        positions.push({ kind: "marker", pos: num(play.geo.markers[m], 0), marker: m });
      });
      if (!positions.length) {
        return;
      }
      var out = [];
      occurrences(play, positions, from, horizon, out);
      out.forEach(function (o) {
        o.play = play;
        hits.push(o);
      });
    });
    hits.sort(function (x, y) {
      return x.t - y.t;
    });
    hits.forEach(function (h) {
      var item = h.item;
      if (item.kind === "beat") {
        var detail = {
          cue: name,
          region: h.play.region,
          beat: (item.index % per) + 1,
          bar: Math.floor(item.index / per) + 1,
        };
        engine.at(h.t, function () {
          engine.emit(root, "grabbag:audio-beat", detail);
          if (detail.beat === 1) {
            engine.emit(root, "grabbag:audio-bar", detail);
          }
        });
      } else {
        var md = { cue: name, region: h.play.region, marker: item.marker };
        engine.at(h.t, function () {
          engine.emit(root, "grabbag:audio-marker", md);
        });
      }
    });
    if (this.timeline && this.state === "playing") {
      this.timeline.scan(horizon);
    }
  };

  // applyStep runs one timeline step. A step can carry several actions:
  //   {to: variant, over, at}         switch variant
  //   {fade: channel, to: vol, over}  fade a channel
  //   {snapshot: name|null, over}     apply or clear a snapshot
  //   {sting: name}                   fire a sting
  // at is a sync point, or a context time (free timelines pass their bar).
  Instance.prototype.applyStep = function (step, at, instant) {
    var self = this;
    var steps = Array.isArray(step) ? step : [step];
    steps.forEach(function (s) {
      if (!s) {
        return;
      }
      // Every action in one step lands at the same time: the free
      // timeline's bar, or the step's sync point. A variant switch waits
      // for the bar by default; other actions run now.
      var variant = !s.fade && typeof s.to === "string";
      var t = instant ? self.engine.now()
        : typeof at === "number" ? at
        : self.when(s.at || at || (variant ? "bar" : "now"));
      if (s.fade) {
        var part = self.parts[s.fade];
        if (!part) {
          warn(self.scope.tenant + ": cue " + self.name + " has no channel " + s.fade);
          return;
        }
        part.strip.fade.set(Math.max(0, num(s.to, 1)), t, instant ? 0 : Math.max(0, num(s.over, 1)));
      } else if (variant) {
        self.switchTo(s.to, t, num(s.over, 1), instant);
      }
      if (s.snapshot !== undefined && !instant) {
        self.engine.applySnapshot(s.snapshot, num(s.over, 0.5), self.scope, t);
      }
      if (s.sting && !instant) {
        self.scope.sting(s.sting, { at: t });
      }
    });
    this.step = steps;
    this.engine.emit(this.scope.root, "audio-step", {
      cue: this.name,
      timeline: this.timelineName,
      step: steps,
      at: typeof at === "number" ? at : null,
    });
  };

  Instance.prototype.report = function () {
    var self = this;
    var channels = {};
    keys(this.def.channels).forEach(function (c) {
      if (self.parts[c]) {
        channels[c] = Math.round(self.parts[c].strip.fade.valueAt(self.engine.now()) * 100) / 100;
      }
    });
    return {
      tenant: this.scope.tenant,
      cue: this.name,
      layer: this.layerName,
      state: this.state,
      mode: this.mode || "",
      variant: this.variant,
      channels: channels,
      timeline: this.timelineName || null,
      step: this.step ? JSON.stringify(this.step) : null,
      seconds: this.timeline && this.timeline.seconds !== undefined ? this.timeline.seconds : null,
      bars: this.timeline && this.timeline.count !== undefined ? this.timeline.count : null,
    };
  };

  // ---- Countdown timeline (GM-022) ------------------------------------------------

  // Countdown steps are keyed by seconds left on a ui-countdown. A step runs
  // when the countdown reaches its second. If time is added and the clock
  // rises back above a step, the cue goes back to the state the remaining
  // steps imply, and that step runs again when the clock crosses it again.
  function Countdown(inst, def, opts) {
    this.inst = inst;
    this.def = def;
    this.thresholds = keys(def.steps)
      .map(Number)
      .filter(function (n) {
        return isFinite(n);
      })
      .sort(function (a, b) {
        return b - a;
      });
    this.fired = {};
    this.match = matcher(opts.countdown, inst.scope.root);
    this.seconds = typeof opts.seconds === "number" ? opts.seconds : secondsLeft(opts.countdown, inst.scope.root);
    this.off = null;
  }

  function matcher(spec, root) {
    if (!spec) {
      return function () {
        return true;
      };
    }
    if (typeof spec === "string") {
      return function (target) {
        return !!(target && target.matches && target.matches(spec));
      };
    }
    if (spec.nodeType === 1) {
      var kind = spec.getAttribute("data-countdown-kind") || "";
      return function (target, detail) {
        if (target === spec) {
          return true;
        }
        // htmx swaps replace the element. Follow its kind once it is gone.
        return !spec.isConnected && !!kind && detail && detail.kind === kind;
      };
    }
    if (typeof spec === "object" && spec.kind) {
      return function (target, detail) {
        return !!detail && detail.kind === spec.kind;
      };
    }
    return function () {
      return true;
    };
  }

  // secondsLeft reads the time left off a countdown element, the same way
  // countdown.js paints it.
  function secondsLeft(spec, root) {
    var el = null;
    if (spec && spec.nodeType === 1) {
      el = spec;
    } else if (typeof spec === "string" && root) {
      el = root.querySelector(spec);
    }
    if (!el) {
      return null;
    }
    var end = Number(el.getAttribute("data-countdown-end")) || 0;
    if (!end || el.hasAttribute("data-countdown-frozen")) {
      var s = Number(el.getAttribute("data-countdown-seconds"));
      return isFinite(s) ? s : null;
    }
    return Math.max(0, Math.ceil((end - Date.now()) / 1000 - 0.0001));
  }

  // plan folds every step already passed into the starting state.
  Countdown.prototype.plan = function (plan) {
    var s = this.seconds;
    if (s === null || s === undefined) {
      return;
    }
    var self = this;
    this.thresholds.forEach(function (th) {
      if (s <= th) {
        self.fired[th] = true;
        fold(plan, self.def.steps[th]);
      }
    });
  };

  function fold(plan, step) {
    [].concat(step).forEach(function (s) {
      if (!s) {
        return;
      }
      if (s.fade) {
        plan.channels[s.fade] = num(s.to, 1);
      } else if (typeof s.to === "string") {
        plan.variant = s.to;
      }
    });
  }

  Countdown.prototype.listen = function () {
    var self = this;
    var root = this.inst.scope.root;
    if (!root) {
      return;
    }
    function onTick(ev) {
      if (self.match(ev.target, ev.detail)) {
        self.tick(Number(ev.detail && ev.detail.seconds));
      }
    }
    native.add.call(root, "grabbag:countdown-tick", onTick);
    this.off = function () {
      native.remove.call(root, "grabbag:countdown-tick", onTick);
    };
  };

  Countdown.prototype.tick = function (seconds) {
    if (!isFinite(seconds) || this.inst.state !== "playing") {
      return;
    }
    this.seconds = seconds;
    var self = this;
    var rose = false;
    this.thresholds.forEach(function (th) {
      if (self.fired[th] && seconds > th) {
        self.fired[th] = false;
        rose = true;
      }
    });
    if (rose) {
      this.restore();
    }
    this.thresholds.forEach(function (th) {
      if (!self.fired[th] && seconds <= th) {
        self.fired[th] = true;
        self.inst.applyStep(self.def.steps[th], "bar", false);
      }
    });
  };

  // restore moves the cue to the state the still-fired steps imply.
  Countdown.prototype.restore = function () {
    var inst = this.inst;
    var def = inst.def;
    var plan = { variant: inst.opts.variant || def.variant || keys(def.variants)[0], channels: {} };
    keys(def.channels).forEach(function (c) {
      plan.channels[c] = num(def.channels[c].volume, 1);
    });
    var self = this;
    this.thresholds.forEach(function (th) {
      if (self.fired[th]) {
        fold(plan, self.def.steps[th]);
      }
    });
    var steps = [{ to: plan.variant, over: 2, at: "bar" }];
    keys(plan.channels).forEach(function (c) {
      steps.push({ fade: c, to: plan.channels[c], over: 2, at: "bar" });
    });
    inst.applyStep(steps, "bar", false);
  };

  Countdown.prototype.scan = function () {};

  Countdown.prototype.stop = function () {
    if (this.off) {
      this.off();
      this.off = null;
    }
  };

  // ---- Free timeline (GM-023) -----------------------------------------------------

  // Free timelines are keyed in bars from the loop start. The script runs
  // first. Then each shuffle entry plays for its bars, picked at random but
  // never the same entry twice in a row. Bars do not count while held.
  function Free(inst, def, opts) {
    this.inst = inst;
    this.def = def;
    this.script = def.script || {};
    this.count = 0;
    this.next = inst.seg ? inst.seg.t0 : inst.startAt;
    this.last = -1;
    var shuffle = (def.then && def.then.shuffle) || [];
    this.shuffle = shuffle;
    var marks = keys(this.script).map(Number).filter(isFinite);
    var lastMark = marks.length ? Math.max.apply(null, marks) : -1;
    var tail = lastMark >= 0 ? num([].concat(this.script[lastMark])[0].bars, 8) : 0;
    this.shuffleAt = num(def.then && def.then.from, lastMark >= 0 ? lastMark + tail : 0);
    this.nextPick = this.shuffleAt;
    var root = inst.scope.root;
    this.held = typeof opts.held === "function" ? opts.held : function () {
      return !!(root && root.querySelector && root.querySelector("[data-audio-hold], [data-countdown-frozen]"));
    };
    this.stopped = false;
  }

  Free.prototype.scan = function (horizon) {
    var bar = this.inst.barLength();
    if (this.stopped || !(bar > 0)) {
      return;
    }
    while (this.next <= horizon) {
      var t = this.next;
      this.next += bar;
      var held = false;
      try {
        held = !!this.held();
      } catch (e) {}
      if (held) {
        continue;
      }
      this.bar(t);
      this.count++;
    }
  };

  Free.prototype.bar = function (t) {
    var n = this.count;
    var inst = this.inst;
    if (this.script[n] !== undefined) {
      inst.applyStep(this.script[n], t, false);
    }
    if (this.shuffle.length && n >= this.shuffleAt && n === this.nextPick) {
      var i = this.pick();
      this.last = i;
      var entry = this.shuffle[i];
      this.nextPick = n + Math.max(1, num(entry.bars, 8));
      inst.applyStep(entry, t, false);
    }
  };

  Free.prototype.pick = function () {
    var n = this.shuffle.length;
    if (n === 1) {
      return 0;
    }
    var i = Math.floor(this.inst.engine.random() * (n - 1));
    if (this.last >= 0 && i >= this.last) {
      i++;
    }
    return Math.min(i, n - 1);
  };

  Free.prototype.stop = function () {
    this.stopped = true;
  };

  // ---- Board boot and the sound card (GM-012) -------------------------------------

  var SOUND_ON_KEY = "grabbagSoundOn";
  var DISMISS_KEY = "grabbagSoundDismissed";

  function storageGet(store, key) {
    try {
      return store.getItem(key);
    } catch (e) {
      return null;
    }
  }

  function storageSet(store, key, value) {
    try {
      store.setItem(key, value);
    } catch (e) {}
  }

  // boot starts the board engine, shows the sound card when the browser
  // blocks audio, and reports the board's sound state to the host.
  function boot(levels) {
    var engine = new Engine({});
    engine.levels(levels);
    api.board = engine;
    var a = engine.audio;
    var card = document.getElementById("shell-sound");
    var reported = "";
    var dismissed = storageGet(sessionStorage, DISMISS_KEY) === "1";
    var checked = false;

    function state() {
      if (a && a.state === "running") {
        return "unlocked";
      }
      return dismissed ? "dismissed" : "waiting";
    }

    function report(s) {
      if (s === reported) {
        return;
      }
      reported = s;
      fetch("/board/audio", {
        method: "POST",
        credentials: "same-origin",
        headers: { "Content-Type": "application/x-www-form-urlencoded" },
        body: "state=" + encodeURIComponent(s),
      }).catch(function () {});
    }

    function update() {
      var s = state();
      if (s === "unlocked") {
        storageSet(localStorage, SOUND_ON_KEY, "1");
      }
      if (card) {
        var show = s === "waiting" && checked;
        if (show && card.hidden) {
          var resume = storageGet(localStorage, SOUND_ON_KEY) === "1";
          var title = card.querySelector("[data-sound-title]");
          if (title) {
            title.textContent = resume ? "Tap to resume sound" : "Sound is off";
          }
          card.hidden = false;
          var button = card.querySelector("[data-sound-on]");
          if (button) {
            button.focus({ preventScroll: true });
          }
        } else if (!show) {
          card.hidden = true;
        }
      }
      if (checked) {
        report(s);
      }
    }

    if (!a) {
      report("waiting");
      return engine;
    }
    a.onstatechange = update;
    if (card) {
      var on = card.querySelector("[data-sound-on]");
      var close = card.querySelector("[data-sound-dismiss]");
      if (on) {
        native.add.call(on, "click", function () {
          dismissed = false;
          storageSet(sessionStorage, DISMISS_KEY, "");
          var p = a.resume();
          if (p && p.then) {
            p.then(update, update);
          }
        });
      }
      if (close) {
        native.add.call(close, "click", function () {
          dismissed = true;
          storageSet(sessionStorage, DISMISS_KEY, "1");
          update();
        });
      }
    }
    // A browser that allows autoplay resumes at once. Give it a moment
    // before showing the card, so it never flashes.
    try {
      var p = a.resume();
      if (p && p.catch) {
        p.catch(function () {});
      }
    } catch (e) {}
    native.setTimeout(function () {
      checked = true;
      update();
    }, 400);
    return engine;
  }

  var api = {
    create: function (opts) {
      return new Engine(opts);
    },
    boot: boot,
    board: null,
    // Exposed for the scheduling tests.
    nextOnGrid: nextOnGrid,
  };
  window.grabbagAudio = api;
})();
