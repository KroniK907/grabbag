// grabbagShell is the persistent /board and / runtime. The page loads once.
// After that the shell swaps tenants (the Lobby, a game, the locked board)
// in and out of #shell-stage without a reload.
//
// The shell owns the document, the one SSE connection, theme, notices, the
// connection overlay, and the heartbeat. Tenants register from their JS:
//
//   grabbagShell.register("quips", {
//     board: { mount: function (root, ctx) {}, unmount: function () {} },
//     phone: { mount: function (root, ctx) {} },
//   });
//
// ctx gives a tenant scoped helpers that clean up on unmount: signal, on,
// every, after, frame, and cleanup. On the board, ctx.audio is the tenant's
// scope in the audio engine (audio.js). Unmount releases it, and a Lobby to
// game swap fades its music out with the board.
//
// The shell refetches its tenant on the round and tenant SSE events and on
// an HX-Trigger of grabbag:tenant. Transitions run one at a time, the latest
// wins, and stale generations are dropped. Only two things reload the page,
// both through hardReload: a kick and a host restart (a new boot ID).
//
// The shell tracks the visible viewport for every tenant: --shell-visual-height
// on <html> is the visual viewport height (it shrinks when a phone keyboard
// opens), and html.shell-short is set below 760px.
//
// On / the shell owns the phone frame around the stage: the toolbar, Help,
// Leave, the Help sheet, and the claimed host's panel. Each tenant fetch says
// whether to show Help (help) and Leave (player), and carries the host panel
// (host), which is swapped outside the stage so tenant fades never touch it.
// live.css picks phone or gutter mode from the frame width; the shell copies
// it to data-frame on #shell-frame for its own controls and for tests.
//
// Layout that CSS cannot do, such as fitting text to a box, goes in a
// tenant's layout.js:
//
//   grabbagShell.layout("apples", { board: function (root) {}, phone: ... });
//
// Layout functions only measure and set styles inside root. The shell runs
// them after mount, after every swap, on resize, and once fonts load, so they
// add no listeners or timers of their own. A static preview
// (window.grabbagStatic) sets the viewport and runs layout, and nothing else:
// no stream and no mount.
//
// Test mode (window.grabbagShellTest set before this file runs) records
// mounts, SSE event names, reloads, errors, and leaks: document and window
// listeners, timers, and animation frames a tenant script started and did
// not stop by unmount.
(function () {
  "use strict";

  if (window.grabbagShell) {
    return;
  }

  var testMode = !!window.grabbagShellTest;
  var proto = EventTarget.prototype;
  var native = {
    add: proto.addEventListener,
    remove: proto.removeEventListener,
    setTimeout: window.setTimeout,
    clearTimeout: window.clearTimeout,
    setInterval: window.setInterval,
    clearInterval: window.clearInterval,
    raf: window.requestAnimationFrame,
    caf: window.cancelAnimationFrame,
  };

  var tenantTrigger = "grabbag:tenant";
  var baseNames = ["round", "tenant", "roster", "theme", "notice"];
  var boardNames = ["audio", "audio-test"];
  var retryNoticeKey = "shell-retry";

  var stage = null;
  var frameEl = null;
  var hostHTML = null;
  var helpOpen = false;
  var helpFetch = 0;
  var hostPanelKey = "grabbag:host-panel";
  var surface = "";
  var boot = "";
  var registry = {};
  var layouts = {};
  var layoutQueued = false;
  var layoutObserver = null;
  var loadedJS = {};
  var scriptOwners = {};
  var mounted = null;
  var audio = null;
  var busy = false;
  var dirty = false;
  var forceNext = false;
  var left = false;
  var reloading = false;

  var source = null;
  var streamURL = "";
  var names = {};
  var opened = false;
  var switching = false;
  var down = false;
  var downTimer = null;
  var retryTimer = null;
  var retryDelay = 1000;

  function later(ms, fn) {
    return native.setTimeout.call(window, fn, ms);
  }

  function sleep(ms) {
    return new Promise(function (resolve) {
      later(ms, resolve);
    });
  }

  function listen(target, type, fn, opts) {
    native.add.call(target, type, fn, opts);
  }

  // Test-mode records. The chromedp suite reads these.
  var record = {
    mounts: [],
    events: [],
    errors: [],
    leaks: [],
  };

  function pushReload(reason) {
    try {
      var list = JSON.parse(sessionStorage.getItem("grabbagShellReloads") || "[]");
      list.push(reason);
      sessionStorage.setItem("grabbagShellReloads", JSON.stringify(list));
    } catch (e) {}
  }

  // ---- Leak tracking (test mode) ------------------------------------------

  var tracked = [];

  // owner names the tenant whose script made the current call, from the
  // stack. Calls from the shell, htmx, and countdown.js are not tracked.
  function owner() {
    var stack = String(new Error().stack || "").split("\n");
    for (var i = 0; i < stack.length; i++) {
      for (var url in scriptOwners) {
        if (stack[i].indexOf(url) >= 0) {
          return { tenant: scriptOwners[url], site: stack[i].trim() };
        }
      }
    }
    return null;
  }

  function capture(opts) {
    return typeof opts === "boolean" ? opts : !!(opts && opts.capture);
  }

  function wrapListeners(target, label) {
    var add = target.addEventListener;
    var remove = target.removeEventListener;
    target.addEventListener = function (type, fn, opts) {
      var who = fn && !(opts && opts.once) ? owner() : null;
      if (who) {
        var entry = {
          tenant: who.tenant, kind: label + " " + type + " listener", site: who.site, live: true,
          target: target, type: type, fn: fn, capture: capture(opts),
        };
        var signal = opts && typeof opts === "object" ? opts.signal : null;
        if (signal && signal.aborted) {
          entry.live = false;
        } else if (signal) {
          native.add.call(signal, "abort", function () {
            entry.live = false;
          });
        }
        tracked.push(entry);
      }
      return add.call(this, type, fn, opts);
    };
    target.removeEventListener = function (type, fn, opts) {
      var c = capture(opts);
      tracked.forEach(function (e) {
        if (e.live && e.target === target && e.type === type && e.fn === fn && e.capture === c) {
          e.live = false;
        }
      });
      return remove.call(this, type, fn, opts);
    };
  }

  function stopTracked(id) {
    tracked.forEach(function (e) {
      if (e.id === id) {
        e.live = false;
      }
    });
  }

  function wrapTimers() {
    window.setTimeout = function (fn, ms) {
      var who = owner();
      var args = Array.prototype.slice.call(arguments, 2);
      if (!who || typeof fn !== "function") {
        return native.setTimeout.apply(window, arguments);
      }
      var entry = { tenant: who.tenant, kind: "setTimeout", site: who.site, live: true };
      entry.id = native.setTimeout.call(window, function () {
        entry.live = false;
        fn.apply(window, args);
      }, ms);
      tracked.push(entry);
      return entry.id;
    };
    window.clearTimeout = function (id) {
      stopTracked(id);
      return native.clearTimeout.call(window, id);
    };
    window.setInterval = function () {
      var who = owner();
      var id = native.setInterval.apply(window, arguments);
      if (who) {
        tracked.push({ tenant: who.tenant, kind: "setInterval", site: who.site, live: true, id: id });
      }
      return id;
    };
    window.clearInterval = function (id) {
      stopTracked(id);
      return native.clearInterval.call(window, id);
    };
    window.requestAnimationFrame = function (fn) {
      var who = owner();
      if (!who) {
        return native.raf.call(window, fn);
      }
      var entry = { tenant: who.tenant, kind: "requestAnimationFrame", site: who.site, live: true };
      entry.id = native.raf.call(window, function (t) {
        entry.live = false;
        fn(t);
      });
      entry.frame = true;
      tracked.push(entry);
      return entry.id;
    };
    window.cancelAnimationFrame = function (id) {
      tracked.forEach(function (e) {
        if (e.frame && e.id === id) {
          e.live = false;
        }
      });
      return native.caf.call(window, id);
    };
  }

  // leakCheck reports what tenant id left running after its unmount.
  function leakCheck(id) {
    if (!testMode || !id) {
      return;
    }
    tracked = tracked.filter(function (e) {
      if (e.tenant !== id) {
        return true;
      }
      if (e.live) {
        var leak = { tenant: id, kind: e.kind, site: e.site };
        record.leaks.push(leak);
        console.error("grabbagShell leak", leak);
      }
      return false;
    });
  }

  if (testMode) {
    wrapListeners(document, "document");
    wrapListeners(window, "window");
    wrapTimers();
    try {
      var loads = Number(sessionStorage.getItem("grabbagShellLoads") || "0") + 1;
      sessionStorage.setItem("grabbagShellLoads", String(loads));
    } catch (e) {}
  }

  // ---- Errors --------------------------------------------------------------

  function report(tenant, phase, err) {
    var message = String((err && (err.stack || err.message)) || err);
    console.error("grabbagShell " + phase + " " + tenant, err);
    if (testMode) {
      record.errors.push({ tenant: tenant, phase: phase, message: message });
    }
    try {
      fetch("/shell/error", {
        method: "POST",
        credentials: "same-origin",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ surface: surface, tenant: tenant, phase: phase, message: message.slice(0, 2000) }),
      }).catch(function () {});
    } catch (e) {}
  }

  // ---- ctx -----------------------------------------------------------------

  function makeCtx(root, tenant, base) {
    var abort = new AbortController();
    var timers = [];
    var frames = [];
    var cleanups = [];
    var stopped = false;
    // The audio scope is released last, after the tenant's own cleanups,
    // which may still call it.
    var sound = audio ? audio.scope(tenant, root, base) : null;
    if (sound) {
      cleanups.push(function () {
        sound.release();
      });
    }
    var ctx = {
      root: root,
      surface: surface,
      tenant: tenant,
      signal: abort.signal,
      audio: sound ? sound.api : undefined,
      on: function (target, type, fn, opts) {
        var o = typeof opts === "object" && opts ? Object.assign({}, opts) : { capture: !!opts };
        o.signal = abort.signal;
        native.add.call(target, type, fn, o);
        return function off() {
          native.remove.call(target, type, fn, o);
        };
      },
      every: function (ms, fn) {
        var id = native.setInterval.call(window, fn, ms);
        timers.push(id);
        return function cancel() {
          native.clearInterval.call(window, id);
        };
      },
      after: function (ms, fn) {
        var id = native.setTimeout.call(window, fn, ms);
        timers.push(id);
        return function cancel() {
          native.clearTimeout.call(window, id);
        };
      },
      frame: function (fn) {
        var slot = { id: 0, done: false };
        frames.push(slot);
        function step(t) {
          if (stopped || slot.done) {
            return;
          }
          if (fn(t) === false) {
            slot.done = true;
            return;
          }
          slot.id = native.raf.call(window, step);
        }
        slot.id = native.raf.call(window, step);
        return function cancel() {
          slot.done = true;
          native.caf.call(window, slot.id);
        };
      },
      cleanup: function (fn) {
        cleanups.push(fn);
      },
    };
    ctx.teardown = function () {
      stopped = true;
      abort.abort();
      timers.forEach(function (id) {
        native.clearInterval.call(window, id);
      });
      frames.forEach(function (slot) {
        slot.done = true;
        native.caf.call(window, slot.id);
      });
      for (var i = cleanups.length - 1; i >= 0; i--) {
        try {
          cleanups[i]();
        } catch (err) {
          report(tenant, "cleanup", err);
        }
      }
      cleanups = [];
    };
    return ctx;
  }

  // ---- Mount ----------------------------------------------------------------

  function mountCurrent() {
    var id = mounted.tenant;
    var def = registry[id];
    var life = def && def[surface];
    mounted.ctx = null;
    mounted.life = null;
    if (testMode) {
      record.mounts.push({ tenant: id, generation: mounted.generation, surface: surface });
    }
    if (!life || typeof life.mount !== "function") {
      if (mounted.needsJS && !def) {
        report(id, "register", "tenant has scripts but did not call grabbagShell.register");
      }
      return;
    }
    var ctx = makeCtx(stage, id, mounted.base);
    try {
      life.mount(stage, ctx);
      mounted.ctx = ctx;
      mounted.life = life;
    } catch (err) {
      report(id, "mount", err);
      ctx.teardown();
    }
  }

  // unmountCurrent follows the contract order: abort the signal, stop timers
  // and frames, run cleanups newest first, then the tenant's unmount.
  function unmountCurrent() {
    if (!mounted) {
      return;
    }
    var id = mounted.tenant;
    if (mounted.ctx) {
      mounted.ctx.teardown();
      if (typeof mounted.life.unmount === "function") {
        try {
          mounted.life.unmount();
        } catch (err) {
          report(id, "unmount", err);
        }
      }
    }
    mounted.ctx = null;
    mounted.life = null;
    leakCheck(id);
  }

  function register(id, def) {
    if (!id || !def) {
      return;
    }
    registry[id] = def;
  }

  function registerLayout(id, fns) {
    if (!id || !fns) {
      return;
    }
    layouts[id] = fns;
    scheduleLayout();
  }

  // runLayout calls the mounted tenant's layout for this surface. A throw is
  // reported and the fragment keeps whatever styles it had.
  function runLayout() {
    layoutQueued = false;
    if (!stage) {
      return;
    }
    syncFrame();
    var id = stage.getAttribute("data-tenant");
    var fn = layouts[id] && layouts[id][surface];
    if (typeof fn !== "function") {
      return;
    }
    try {
      fn(stage);
    } catch (err) {
      report(id, "layout", err);
    }
  }

  // observeStage refits when a top-level tenant element changes size. Window
  // resize alone is not enough: viewport units such as dvh can settle after
  // the resize event, and a tenant can resize itself.
  function observeStage() {
    if (!window.ResizeObserver || !stage) {
      return;
    }
    if (!layoutObserver) {
      layoutObserver = new ResizeObserver(scheduleLayout);
    }
    layoutObserver.disconnect();
    Array.prototype.forEach.call(stage.children, function (el) {
      layoutObserver.observe(el);
    });
  }

  // scheduleLayout coalesces layout requests into one run. It uses a timer,
  // not a frame: a background or headless tab may not draw frames for a while.
  function scheduleLayout() {
    if (layoutQueued || !stage) {
      return;
    }
    layoutQueued = true;
    later(0, runLayout);
  }

  // ---- Assets ---------------------------------------------------------------

  // scriptBase is the folder of a tenant's first script. Audio file paths
  // in ctx.audio.define resolve against it.
  function scriptBase(list) {
    var src = list && list[0];
    if (!src) {
      return "";
    }
    var path = absolute(src).split(/[?#]/)[0];
    return path.slice(0, path.lastIndexOf("/") + 1);
  }

  function absolute(url) {
    try {
      return new URL(url, location.href).href;
    } catch (e) {
      return url;
    }
  }

  // addLink adds a stylesheet and settles once it loads. A sheet that fails
  // is removed, so the next transition tries it again instead of finding a
  // dead <link> and skipping it.
  function addLink(href, attr, value) {
    return new Promise(function (resolve) {
      var link = document.createElement("link");
      link.rel = "stylesheet";
      link.href = href;
      link.setAttribute(attr, value);
      link.onload = resolve;
      link.onerror = function () {
        link.remove();
        resolve();
      };
      document.head.appendChild(link);
    });
  }

  function addScript(src, tenant, attr) {
    if (!attr) {
      scriptOwners[absolute(src)] = tenant;
    }
    return new Promise(function (resolve, reject) {
      var script = document.createElement("script");
      script.src = src;
      script.async = false;
      script.setAttribute(attr || "data-shell-js", tenant);
      script.onload = resolve;
      script.onerror = function () {
        script.remove();
        reject(new Error("could not load " + src));
      };
      document.head.appendChild(script);
    });
  }

  function hasLink(attr, href) {
    return Array.prototype.some.call(document.querySelectorAll("link[" + attr + "]"), function (l) {
      return l.getAttribute("href") === href;
    });
  }

  function loadAssets(frame) {
    var assets = frame.assets || {};
    var jobs = [];
    (assets.external || []).forEach(function (href) {
      if (!hasLink("data-shell-external", href)) {
        jobs.push(addLink(href, "data-shell-external", ""));
      }
    });
    (assets.css || []).forEach(function (href) {
      if (!hasLink("data-shell-css", href)) {
        jobs.push(addLink(href, "data-shell-css", frame.tenant));
      }
    });
    (assets.js || []).forEach(function (src) {
      var key = frame.tenant + " " + src;
      if (!loadedJS[key]) {
        // A failed load is forgotten, so a reconnect or the next Start
        // fetches the script again.
        loadedJS[key] = addScript(src, frame.tenant).catch(function (err) {
          delete loadedJS[key];
          throw err;
        });
      }
      jobs.push(loadedJS[key]);
    });
    (assets.layout || []).forEach(function (src) {
      var key = "layout " + frame.tenant + " " + src;
      if (!loadedJS[key]) {
        loadedJS[key] = addScript(src, frame.tenant, "data-shell-layout");
      }
      jobs.push(loadedJS[key]);
    });
    return Promise.all(jobs);
  }

  function dropOldCSS(frame) {
    var keep = (frame.assets && frame.assets.css) || [];
    document.querySelectorAll("link[data-shell-css]").forEach(function (link) {
      if (keep.indexOf(link.getAttribute("href")) < 0) {
        link.remove();
      } else {
        link.setAttribute("data-shell-css", frame.tenant);
      }
    });
  }

  // ---- Theme and notices ----------------------------------------------------

  function applyTheme(value) {
    if (value === "neon-light" || value === "neon-dark") {
      document.documentElement.setAttribute("data-theme", value);
    }
  }

  function setTargets(value) {
    var el = document.getElementById("notice-targets");
    if (el && typeof value === "string") {
      el.setAttribute("data-notice-targets", value);
    }
  }

  function refetchTargets() {
    fetch("/lobby/partials/notice-targets", { credentials: "same-origin", cache: "no-store" })
      .then(function (res) {
        return res.ok ? res.text() : "";
      })
      .then(function (html) {
        var m = /data-notice-targets="([^"]*)"/.exec(html || "");
        if (m) {
          setTargets(m[1]);
        }
      })
      .catch(function () {});
  }

  // ---- Transitions ------------------------------------------------------------

  function reduceMotion() {
    return !!(window.matchMedia && window.matchMedia("(prefers-reduced-motion: reduce)").matches);
  }

  function isGame(id) {
    return !!id && id !== "lobby" && id !== "locked";
  }

  // timing is the fade for one swap. Lobby to game and back on the TV is the
  // long fade; other TV swaps are a quick crossfade; phones are quicker.
  function timing(from, to) {
    if (reduceMotion()) {
      return { out: 0, in: 0 };
    }
    if (surface === "phone") {
      return { out: 150, in: 150 };
    }
    if ((from === "lobby" && isGame(to)) || (isGame(from) && to === "lobby")) {
      return { out: 1500, in: 500 };
    }
    return { out: 200, in: 200 };
  }

  function reflow() {
    return document.body.offsetWidth;
  }

  function fadeOut(ms) {
    stage.style.setProperty("--shell-fade-out", ms + "ms");
    if (!ms) {
      return Promise.resolve();
    }
    stage.classList.add("is-fading");
    reflow();
    stage.classList.add("is-out");
    return sleep(ms);
  }

  function fadeIn(ms) {
    stage.style.setProperty("--shell-fade-in", ms + "ms");
    if (!stage.classList.contains("is-out")) {
      return Promise.resolve();
    }
    reflow();
    stage.classList.remove("is-out");
    return sleep(ms).then(function () {
      stage.classList.remove("is-fading");
    });
  }

  function spinner(on) {
    var el = document.getElementById("shell-spin");
    if (el) {
      el.hidden = !on;
    }
  }

  function tenantURL() {
    return surface === "board" ? "/board/tenant" : "/tenant";
  }

  // fetchFrame keeps the current tenant on screen and retries a failed fetch
  // at 1, 2, 4, then every 10 seconds. After about 10 seconds it says so.
  function fetchFrame() {
    var delay = 1000;
    var warnTimer = null;
    var warned = false;
    function attempt() {
      return fetch(tenantURL(), {
        credentials: "same-origin",
        cache: "no-store",
        headers: { Accept: "application/json" },
      })
        .then(function (res) {
          if (!res.ok) {
            throw new Error("tenant fetch returned " + res.status);
          }
          return res.json();
        })
        .then(function (frame) {
          if (warnTimer) {
            native.clearTimeout.call(window, warnTimer);
          }
          if (warned && window.grabbagHideNotice) {
            window.grabbagHideNotice(retryNoticeKey);
          }
          return frame;
        })
        .catch(function () {
          if (reloading) {
            return null;
          }
          if (!warnTimer) {
            warnTimer = later(10000, function () {
              warned = true;
              if (window.grabbagShowNotice) {
                window.grabbagShowNotice({
                  key: retryNoticeKey, type: "error", duration: 0,
                  message: "Couldn't load the next screen. Retrying...",
                });
              }
            });
          }
          var wait = delay;
          delay = delay >= 4000 ? 10000 : delay * 2;
          return sleep(wait).then(attempt);
        });
    }
    return attempt();
  }

  function request(force) {
    if (force) {
      forceNext = true;
    }
    if (reloading) {
      return;
    }
    if (busy) {
      dirty = true;
      return;
    }
    busy = true;
    run();
  }

  function run() {
    dirty = false;
    var force = forceNext;
    forceNext = false;
    step(force)
      .catch(function (err) {
        report(mounted ? mounted.tenant : "", "transition", err);
      })
      .then(function () {
        if ((dirty || forceNext) && !reloading) {
          run();
          return;
        }
        busy = false;
      });
  }

  function step(force) {
    return fetchFrame().then(function (frame) {
      if (!frame || reloading) {
        return;
      }
      if (frame.boot !== boot) {
        hardReload("host-restarted");
        return;
      }
      applyTheme(frame.theme);
      setTargets(frame.notices);
      if (audio && frame.audio) {
        audio.levels(frame.audio);
      }
      if (frame.generation < mounted.generation) {
        return;
      }
      if (frame.kicked && mounted.player && !left) {
        hardReload("kicked");
        return;
      }
      if (frame.stream && frame.stream !== streamURL) {
        switching = true;
        connect(frame.stream);
      }
      applyChrome(frame);
      var same = frame.tenant === mounted.tenant;
      if (!force && same && (frame.generation === mounted.generation || frame.html === mounted.html)) {
        mounted.generation = frame.generation;
        mounted.player = !!frame.player;
        return;
      }
      return swap(frame, force && same);
    });
  }

  function swap(frame, instant) {
    var fade = instant ? { out: 0, in: 0 } : timing(mounted.tenant, frame.tenant);
    var ready = loadAssets(frame).catch(function (err) {
      report(frame.tenant, "assets", err);
    });
    // The outgoing tenant's music fades with the board (GM-013). With no
    // fade, unmount releases it with the default crossfade.
    if (audio && fade.out) {
      audio.leave(fade.out / 1000);
    }
    return fadeOut(fade.out)
      .then(function () {
        var spin = later(400, function () {
          spinner(true);
        });
        return ready.then(function () {
          native.clearTimeout.call(window, spin);
          spinner(false);
        });
      })
      .then(function () {
        if (frame.tenant !== mounted.tenant) {
          closeHelp();
        }
        unmountCurrent();
        dropOldCSS(frame);
        if (window.htmx) {
          htmx.swap(stage, frame.html, { swapStyle: "innerHTML", swapDelay: 0, settleDelay: 0 });
        } else {
          stage.innerHTML = frame.html;
        }
        stage.setAttribute("data-tenant", frame.tenant);
        stage.setAttribute("data-generation", String(frame.generation));
        stage.toggleAttribute("data-player", !!frame.player);
        // A phone that just joined is marked disconnected until it beats.
        // Before the shell, the reload after Join sent that beat at once.
        if (frame.player && !mounted.player) {
          beat();
        }
        mounted = {
          tenant: frame.tenant,
          generation: frame.generation,
          html: frame.html,
          player: !!frame.player,
          needsJS: !!(frame.assets && frame.assets.js && frame.assets.js.length),
          base: scriptBase(frame.assets && frame.assets.js),
        };
        scanNames(stage);
        mountCurrent();
        observeStage();
        runLayout();
        return fadeIn(fade.in);
      });
  }

  function hardReload(reason) {
    if (reloading) {
      return;
    }
    reloading = true;
    if (testMode) {
      pushReload(reason);
    }
    location.reload();
  }

  // ---- SSE ----------------------------------------------------------------------

  // triggerNames lists the sse:<name> triggers in an hx-trigger value.
  function triggerNames(value) {
    var out = [];
    String(value || "").split(",").forEach(function (part) {
      var token = part.trim().split(/\s+/)[0] || "";
      if (token.indexOf("sse:") === 0) {
        out.push(token.slice(4).split("[")[0]);
      }
    });
    return out;
  }

  function triggerOf(el) {
    return el.getAttribute("hx-trigger") || el.getAttribute("data-hx-trigger") || "";
  }

  function scanNames(root) {
    if (!root || !root.querySelectorAll) {
      return;
    }
    var els = Array.prototype.slice.call(root.querySelectorAll("[hx-trigger*='sse:'], [data-hx-trigger*='sse:']"));
    if (root.matches && root.matches("[hx-trigger*='sse:'], [data-hx-trigger*='sse:']")) {
      els.push(root);
    }
    els.forEach(function (el) {
      triggerNames(triggerOf(el)).forEach(subscribe);
    });
  }

  function subscribe(name) {
    if (!source || names[name]) {
      return;
    }
    names[name] = true;
    native.add.call(source, name, function (ev) {
      onMessage(name, ev);
    });
  }

  // dispatch fires sse:<name> on every element that listens for it, the way
  // the htmx SSE extension does, so hx-trigger="sse:<name>" keeps working.
  function dispatch(name, data) {
    if (!window.htmx) {
      return;
    }
    document.querySelectorAll("[hx-trigger*='sse:" + name + "'], [data-hx-trigger*='sse:" + name + "']").forEach(function (el) {
      if (triggerNames(triggerOf(el)).indexOf(name) >= 0) {
        htmx.trigger(el, "sse:" + name, { data: data });
      }
    });
  }

  function onMessage(name, ev) {
    var data = typeof ev.data === "string" ? ev.data.trim() : "";
    if (testMode) {
      record.events.push(name);
    }
    switch (name) {
      case "theme":
        applyTheme(data);
        break;
      case "notice":
        try {
          var msg = JSON.parse(data);
          if (msg && window.grabbagEnqueueNotice) {
            window.grabbagEnqueueNotice(msg);
          }
        } catch (e) {}
        break;
      case "round":
      case "tenant":
        refetchTargets();
        request(false);
        break;
      case "roster":
        refetchTargets();
        checkPresence();
        break;
      case "audio":
        if (audio) {
          try {
            audio.levels(JSON.parse(data));
          } catch (e) {}
        }
        break;
      case "audio-test":
        if (audio) {
          audio.test(data);
        }
        break;
    }
    dispatch(name, data);
  }

  function overlay(on) {
    var el = document.getElementById("connection-overlay");
    if (el) {
      el.hidden = !on;
    }
  }

  function connect(url) {
    if (retryTimer) {
      native.clearTimeout.call(window, retryTimer);
      retryTimer = null;
    }
    if (source) {
      source.close();
    }
    streamURL = url;
    names = {};
    source = new EventSource(url);
    source.onopen = onOpen;
    source.onerror = onError;
    baseNames.forEach(subscribe);
    if (surface === "board") {
      boardNames.forEach(subscribe);
    }
    scanNames(document.body);
  }

  function onOpen() {
    var recovered = down;
    down = false;
    retryDelay = 1000;
    if (downTimer) {
      native.clearTimeout.call(window, downTimer);
      downTimer = null;
    }
    overlay(false);
    // The first open is quiet unless the stream failed before it: then the
    // host may have restarted during that outage, so reconcile like any
    // reconnect.
    if ((!opened && !recovered) || switching) {
      opened = true;
      switching = false;
      if (window.htmx) {
        htmx.trigger(document.body, "htmx:sseOpen", {});
      }
      return;
    }
    opened = true;
    // A reconnect: the host may have restarted, and events may be lost.
    beat();
    request(true);
  }

  // onError shows the overlay only once the stream has stayed down for 3s,
  // so a blip that reconnects at once does not alarm the room.
  function onError() {
    if (!down) {
      down = true;
      downTimer = later(3000, function () {
        if (down) {
          overlay(true);
        }
      });
    }
    if (source && source.readyState === EventSource.CLOSED && !retryTimer) {
      var wait = retryDelay;
      retryDelay = Math.min(retryDelay * 2, 10000);
      retryTimer = later(wait, function () {
        retryTimer = null;
        connect(streamURL);
      });
    }
  }

  // ---- Presence and heartbeat ---------------------------------------------------

  // checkPresence asks whether this phone's player still exists. 410 means
  // the host kicked it (or cleared the room), which resets the phone fully.
  function checkPresence() {
    if (surface !== "phone" || !mounted || !mounted.player || left) {
      return;
    }
    fetch("/lobby/presence", { credentials: "same-origin", cache: "no-store" })
      .then(function (res) {
        if (res.status === 410 && mounted.player && !left) {
          hardReload("kicked");
        }
      })
      .catch(function () {});
  }

  function beat() {
    fetch("/lobby/heartbeat", { method: "POST", credentials: "same-origin" }).catch(function () {});
  }

  // ---- Viewport ---------------------------------------------------------------------

  // syncViewport publishes the visible height. Phone tenants size against it
  // so the keyboard does not push their controls off screen.
  function syncViewport() {
    var viewport = window.visualViewport;
    var height = viewport ? viewport.height : window.innerHeight;
    if (!height) {
      return;
    }
    var root = document.documentElement;
    root.style.setProperty("--shell-visual-height", Math.round(height) + "px");
    root.classList.toggle("shell-short", height < 760);
    scheduleLayout();
  }

  // ---- Phone frame --------------------------------------------------------------------

  // syncFrame copies live.css's frame mode to data-frame: gutter when the
  // column has its gutters, phone when it fills the width.
  function syncFrame() {
    if (!frameEl) {
      return;
    }
    var column = frameEl.querySelector(".shell-column");
    var gutter = column && getComputedStyle(column).getPropertyValue("--shell-gutter").trim() === "1";
    var mode = gutter ? "gutter" : "phone";
    if (frameEl.getAttribute("data-frame") !== mode) {
      frameEl.setAttribute("data-frame", mode);
    }
  }

  // applyChrome shows Help and Leave for the fetched tenant and swaps the
  // host panel when its markup changed. An open drawer stays open.
  function applyChrome(frame) {
    if (!frameEl) {
      return;
    }
    var help = document.getElementById("shell-help-button");
    if (help) {
      help.hidden = !frame.help;
    }
    if (!frame.help) {
      closeHelp();
    }
    var leave = document.getElementById("shell-leave");
    if (leave) {
      leave.hidden = !frame.player;
    }
    var html = frame.host || "";
    if (html === hostHTML) {
      return;
    }
    hostHTML = html;
    var host = document.getElementById("shell-host");
    if (!host) {
      return;
    }
    var open = !!host.querySelector(".ui-drawer.is-open");
    if (window.htmx) {
      htmx.swap(host, html, { swapStyle: "innerHTML", swapDelay: 0, settleDelay: 0 });
    } else {
      host.innerHTML = html;
    }
    var drawer = host.querySelector(".ui-drawer");
    if (open && drawer) {
      drawer.classList.add("is-open");
    }
    scanNames(host);
  }

  // openHelp fetches the mounted tenant's help from GET /help and opens the
  // sheet over it. The tenant stays mounted. A static preview already has
  // its sheet body and only opens it.
  function openHelp() {
    var sheet = document.getElementById("shell-help");
    var body = document.getElementById("shell-help-body");
    if (!sheet || !body) {
      return;
    }
    if (window.grabbagStatic) {
      showHelp(sheet);
      return;
    }
    var ticket = ++helpFetch;
    fetch("/help", { credentials: "same-origin", cache: "no-store" })
      .then(function (res) {
        if (!res.ok) {
          throw new Error("help returned " + res.status);
        }
        return res.text();
      })
      .then(function (html) {
        if (ticket !== helpFetch) {
          return;
        }
        if (window.htmx) {
          htmx.swap(body, html, { swapStyle: "innerHTML", swapDelay: 0, settleDelay: 0 });
        } else {
          body.innerHTML = html;
        }
        body.scrollTop = 0;
        showHelp(sheet);
      })
      .catch(function () {
        if (window.grabbagShowNotice) {
          window.grabbagShowNotice({ type: "error", message: "Couldn't load the help. Try again.", duration: -1 });
        }
      });
  }

  // setBackgroundInert takes the frame behind the Help sheet (toolbar,
  // column, host panel) out of focus and pointer reach while the sheet is
  // open, so Tab cannot land on a tenant control or Leave behind it. The
  // tenant stays mounted.
  function setBackgroundInert(on) {
    if (!frameEl) {
      return;
    }
    frameEl.querySelectorAll(":scope > .shell-main, :scope > #shell-host").forEach(function (el) {
      el.inert = on;
      el.toggleAttribute("inert", on);
    });
  }

  function showHelp(sheet) {
    setBackgroundInert(true);
    sheet.hidden = false;
    helpOpen = true;
    var close = sheet.querySelector(".shell-sheet-close");
    if (close) {
      close.focus();
    }
  }

  function closeHelp() {
    helpFetch++;
    var sheet = document.getElementById("shell-help");
    if (!sheet || sheet.hidden) {
      helpOpen = false;
      return;
    }
    sheet.hidden = true;
    setBackgroundInert(false);
    if (helpOpen) {
      var button = document.getElementById("shell-help-button");
      if (button && !button.hidden) {
        button.focus();
      }
    }
    helpOpen = false;
  }

  // toggleHostPanel collapses or reopens the wide-screen host panel and
  // remembers the choice in this browser.
  function toggleHostPanel() {
    if (!frameEl) {
      return;
    }
    var collapsed = frameEl.getAttribute("data-host-panel") !== "collapsed";
    if (collapsed) {
      frameEl.setAttribute("data-host-panel", "collapsed");
    } else {
      frameEl.removeAttribute("data-host-panel");
    }
    try {
      if (collapsed) {
        localStorage.setItem(hostPanelKey, "collapsed");
      } else {
        localStorage.removeItem(hostPanelKey);
      }
    } catch (e) {}
    scheduleLayout();
  }

  // ---- Start ------------------------------------------------------------------------

  function requestPath(evt) {
    var d = evt.detail || {};
    var path = (d.pathInfo && d.pathInfo.requestPath) || (d.requestConfig && d.requestConfig.path) || "";
    return String(path).split("?")[0];
  }

  function start() {
    stage = document.getElementById("shell-stage");
    if (!stage) {
      return;
    }
    surface = stage.getAttribute("data-surface");
    boot = stage.getAttribute("data-boot");
    frameEl = document.getElementById("shell-frame");
    if (frameEl) {
      listen(document, "keydown", function (evt) {
        if (evt.key === "Escape" && document.getElementById("shell-help") && !document.getElementById("shell-help").hidden) {
          closeHelp();
        }
      });
    }
    // Sizing listeners come before the static return: the preview gallery
    // resizes a preview's frame in place, and that must refit it too.
    syncViewport();
    listen(window, "resize", syncViewport);
    listen(window, "orientationchange", syncViewport);
    if (window.visualViewport) {
      listen(window.visualViewport, "resize", syncViewport);
    }
    if (document.fonts && document.fonts.ready) {
      document.fonts.ready.then(scheduleLayout);
    }
    observeStage();
    if (window.grabbagStatic) {
      runLayout();
      return;
    }
    // Process swapped markup in the same tick as the swap. With htmx's 20ms
    // settle delay a tap in that gap submits an unprocessed form natively,
    // and a Join form would put the host password in the URL.
    if (window.htmx) {
      htmx.config.defaultSettleDelay = 0;
    }
    document.querySelectorAll("script[data-shell-layout]").forEach(function (script) {
      loadedJS["layout " + script.getAttribute("data-shell-layout") + " " + script.getAttribute("src")] = Promise.resolve();
    });
    var firstJS = [];
    document.querySelectorAll("script[data-shell-js]").forEach(function (script) {
      var tenant = script.getAttribute("data-shell-js");
      loadedJS[tenant + " " + script.getAttribute("src")] = Promise.resolve();
      scriptOwners[script.src] = tenant;
      firstJS.push(script.src);
    });
    mounted = {
      tenant: stage.getAttribute("data-tenant"),
      generation: Number(stage.getAttribute("data-generation")) || 0,
      html: null,
      player: stage.hasAttribute("data-player"),
      needsJS: firstJS.length > 0,
      base: scriptBase(firstJS),
    };
    if (surface === "board" && window.grabbagAudio) {
      var levels = null;
      try {
        levels = JSON.parse(stage.getAttribute("data-audio") || "null");
      } catch (e) {}
      audio = grabbagAudio.boot(levels);
    }

    listen(document, tenantTrigger, function () {
      request(false);
    });
    listen(document.body, "htmx:beforeRequest", function (evt) {
      var path = requestPath(evt);
      if (path === "/lobby/leave") {
        left = true;
      } else if (path === "/lobby/join") {
        left = false;
      }
      // htmx fires HX-Trigger events on the element that sent the request.
      // A roster refetch can replace that element (the Join form) while the
      // POST is in flight, and then the event never reaches document. So
      // read the tenant trigger off the response itself.
      var xhr = evt.detail && evt.detail.xhr;
      if (xhr) {
        native.add.call(xhr, "loadend", function () {
          var trigger = xhr.getResponseHeader("HX-Trigger") || "";
          if (trigger.indexOf(tenantTrigger) >= 0) {
            request(false);
          }
        });
      }
    });
    listen(document.body, "htmx:afterSettle", function (evt) {
      scanNames(evt.target);
      observeStage();
      runLayout();
    });
    listen(document.body, "htmx:responseError", function (evt) {
      var xhr = evt.detail && evt.detail.xhr;
      var text = String((xhr && xhr.responseText) || "").trim();
      if (!text || text.length > 200 || text.charAt(0) === "<") {
        text = "Something went wrong. Try again.";
      }
      if (window.grabbagShowNotice) {
        window.grabbagShowNotice({ type: "error", message: text, duration: -1 });
      }
    });

    connect(stage.getAttribute("data-stream") || "/lobby/events");
    beat();
    native.setInterval.call(window, beat, 2000);
    mountCurrent();
    runLayout();
  }

  window.grabbagShell = {
    register: register,
    layout: registerLayout,
    hardReload: hardReload,
    openHelp: openHelp,
    closeHelp: closeHelp,
    toggleHostPanel: toggleHostPanel,
    // current reports the mounted tenant and the stream: its URL and
    // whether it is open now.
    current: function () {
      if (!mounted) {
        return null;
      }
      return {
        tenant: mounted.tenant,
        generation: mounted.generation,
        surface: surface,
        stream: streamURL,
        live: !!source && source.readyState === 1,
      };
    },
    record: testMode ? record : undefined,
  };

  // Tenant scripts are deferred after this one. DOMContentLoaded fires once
  // they have run and registered.
  if (document.readyState === "complete") {
    start();
  } else {
    listen(document, "DOMContentLoaded", start);
  }
})();
