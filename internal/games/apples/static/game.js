(function () {
  "use strict";

  var savedScroll = 0;

  function syncVisualViewport() {
    var viewport = window.visualViewport;
    var height = viewport ? viewport.height : window.innerHeight;
    if (!height) {
      return;
    }
    document.documentElement.style.setProperty("--apples-visual-height", Math.round(height) + "px");
    document.documentElement.classList.toggle("apples-compact-height", height < 760);
  }

  function paintTimers() {
    document.querySelectorAll("[data-apples-timer]").forEach(function (timer) {
      var total = Number(timer.dataset.timerTotal) || 0;
      var left;
      if (timer.dataset.timerPaused === "1") {
        left = Number(timer.dataset.timerSeconds) || 0;
      } else {
        var end = Number(timer.dataset.timerEnd) || 0;
        left = end > 0 ? Math.max(0, (end - Date.now()) / 1000) : 0;
      }
      var display = Math.max(0, Math.ceil(left - 0.0001));
      var value = timer.querySelector("[data-timer-value]");
      if (value) {
        value.textContent = Math.floor(display / 60) + ":" + String(display % 60).padStart(2, "0");
      }
      timer.style.setProperty("--apples-timer-turn", total > 0 ? String(left / total) + "turn" : "0turn");
    });
  }

  if (!window.grabbagApplesTimer) {
    window.grabbagApplesTimer = window.setInterval(paintTimers, 250);
    document.body.addEventListener("htmx:beforeSwap", function (evt) {
      var target = evt.detail && evt.detail.target;
      if (!target || target.id !== "apples-phone") {
        return;
      }
      var scroll = target.querySelector(".apples-hand-scroll") || target.querySelector(".apples-phone-scroll");
      if (scroll) {
        savedScroll = scroll.scrollTop;
      }
    });
    document.body.addEventListener("htmx:afterSwap", function (evt) {
      var el = evt.detail && evt.detail.elt;
      if (!el || el.id !== "apples-phone") {
        return;
      }
      var scroll = el.querySelector(".apples-hand-scroll") || el.querySelector(".apples-phone-scroll");
      if (scroll) {
        scroll.scrollTop = savedScroll;
      }
      paintTimers();
    });
  }
  if (!window.grabbagApplesViewport) {
    window.grabbagApplesViewport = true;
    window.addEventListener("resize", syncVisualViewport);
    window.addEventListener("orientationchange", syncVisualViewport);
    if (window.visualViewport) {
      window.visualViewport.addEventListener("resize", syncVisualViewport);
    }
  }
  syncVisualViewport();
  paintTimers();
})();
