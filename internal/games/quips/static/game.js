(function () {
  "use strict";

  function runeLen(text) {
    return Array.from(text).length;
  }

  function bindComposeCards() {
    document.querySelectorAll(".quips-compose-card textarea").forEach(function (ta) {
      if (ta.dataset.quipsComposeBound === "1") {
        return;
      }
      ta.dataset.quipsComposeBound = "1";
      ta.addEventListener("input", function () {
        var card = ta.closest(".quips-compose-card");
        if (card) {
          card.classList.remove("dup-outline");
        }
        var cap = card && card.querySelector(".quips-cap");
        var max = Number(ta.getAttribute("maxlength")) || 0;
        if (cap && max > 0) {
          var remain = Math.max(0, max - runeLen(ta.value));
          cap.textContent = remain + " / " + max;
        }
        var scroll = document.querySelector("#quips-phone .quips-phone-scroll");
        if (scroll && scroll.querySelector(".quips-compose-card")) {
          var alert = scroll.querySelector(".ui-alert");
          if (alert) {
            alert.remove();
          }
        }
      });
    });
  }

  function paintTimers() {
    document.querySelectorAll("[data-quips-timer]").forEach(function (timer) {
      var total = Number(timer.dataset.timerTotal) || 0;
      var left;
      if (timer.dataset.timerPaused === "1") {
        left = Number(timer.dataset.timerSeconds) || 0;
      } else {
        var end = Number(timer.dataset.timerEnd) || 0;
        left = end > 0 ? Math.max(0, (end * 1000 - Date.now()) / 1000) : 0;
      }
      var display = Math.max(0, Math.ceil(left - 0.0001));
      var value = timer.querySelector("[data-timer-value]");
      if (value) {
        value.textContent = Math.floor(display / 60) + ":" + String(display % 60).padStart(2, "0");
      }
    });
  }

  if (!window.grabbagQuipsTimer) {
    window.grabbagQuipsTimer = window.setInterval(paintTimers, 250);
    document.body.addEventListener("htmx:afterSwap", function () {
      paintTimers();
      bindComposeCards();
    });
  }
  paintTimers();
  bindComposeCards();
})();
