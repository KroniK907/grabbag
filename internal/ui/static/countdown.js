// Shared countdown for game timers. Server markup comes from the ui-countdown
// template: data-countdown-end is the end in Unix ms, data-countdown-seconds
// is the time left at render, and data-countdown-frozen holds the clock (host
// pause, a reshuffle overlay). Static previews always show the render value.
//
// Each paint writes m:ss into [data-countdown-value] children and sets
// --countdown-left (1 to 0) and --countdown-turn (for conic rings) on the
// countdown element. When the shown second changes on a running countdown it
// fires grabbag:countdown-tick, and once it reaches zero it fires
// grabbag:countdown-done. Both bubble from the countdown element with
// detail {seconds, kind}. Countdowns are keyed by their end, so an SSE swap
// that repaints the same countdown fires nothing new, and a page that loads
// after the end fires nothing at all.
(function () {
  "use strict";

  if (window.grabbagCountdown) {
    return;
  }

  var shown = {};
  var done = {};

  function clock(sec) {
    return Math.floor(sec / 60) + ":" + String(sec % 60).padStart(2, "0");
  }

  function fire(el, name, seconds) {
    el.dispatchEvent(new CustomEvent(name, {
      bubbles: true,
      detail: { seconds: seconds, kind: el.dataset.countdownKind || "" },
    }));
  }

  function paintOne(el, now) {
    var total = Number(el.dataset.countdownTotal) || 0;
    var end = Number(el.dataset.countdownEnd) || 0;
    var running = end > 0 && !el.hasAttribute("data-countdown-frozen") && !window.grabbagStatic;
    var left = running ? Math.max(0, (end - now) / 1000) : Number(el.dataset.countdownSeconds) || 0;
    var seconds = Math.max(0, Math.ceil(left - 0.0001));
    el.querySelectorAll("[data-countdown-value]").forEach(function (value) {
      value.textContent = clock(seconds);
    });
    var share = total > 0 ? Math.min(1, left / total) : 0;
    el.style.setProperty("--countdown-left", String(share));
    el.style.setProperty("--countdown-turn", share + "turn");
    if (!running) {
      return;
    }
    var key = String(end);
    var prev = shown[key];
    shown[key] = seconds;
    if (prev === undefined || prev === seconds) {
      return;
    }
    fire(el, "grabbag:countdown-tick", seconds);
    if (seconds === 0 && !done[key]) {
      done[key] = true;
      fire(el, "grabbag:countdown-done", 0);
    }
  }

  function paint() {
    var now = Date.now();
    document.querySelectorAll("[data-countdown]").forEach(function (el) {
      paintOne(el, now);
    });
  }

  window.grabbagCountdown = { paint: paint };
  window.setInterval(paint, 250);
  document.addEventListener("DOMContentLoaded", paint);
  document.addEventListener("htmx:afterSwap", paint);
})();
