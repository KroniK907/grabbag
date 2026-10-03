(function () {
  "use strict";

  function runeLen(text) {
    return Array.from(text).length;
  }

  function syncVisualViewport() {
    var viewport = window.visualViewport;
    var height = viewport ? viewport.height : window.innerHeight;
    if (!height) {
      return;
    }
    document.documentElement.style.setProperty("--quips-visual-height", Math.round(height) + "px");
    document.documentElement.classList.toggle("quips-compact-height", height < 760);
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

  function markWinner() {
    if (window.grabbagStatic) {
      return;
    }
    var board = document.querySelector(".quips-final[data-winner]");
    if (!board) {
      return;
    }
    var key = board.getAttribute("data-winner") || "";
    if (window.grabbagQuipsWinner === key) {
      return;
    }
    window.grabbagQuipsWinner = key;
    board.classList.add("is-arriving");
  }

  function markLastClock() {
    if (window.grabbagStatic) {
      return;
    }
    var clock = document.querySelector(".quips-board-clock[data-last-clock]");
    if (!clock) {
      return;
    }
    var key = clock.getAttribute("data-last-clock") || "";
    if (window.grabbagQuipsLastClock === key) {
      return;
    }
    window.grabbagQuipsLastClock = key;
    clock.classList.add("is-arriving");
  }

  function markVoteClock() {
    if (window.grabbagStatic) {
      return;
    }
    var clock = document.querySelector(".quips-board-clock[data-vote-clock]");
    if (!clock) {
      return;
    }
    var key = clock.getAttribute("data-vote-clock") || "";
    if (window.grabbagQuipsVoteClock === key) {
      return;
    }
    window.grabbagQuipsVoteClock = key;
    clock.classList.add("is-fading");
  }

  function markMatchup() {
    if (window.grabbagStatic) {
      return;
    }
    var row = document.querySelector(".quips-quip-row.is-matchup");
    if (!row) {
      return;
    }
    var key = row.getAttribute("data-matchup") || "";
    if (window.grabbagQuipsMatchup === key) {
      return;
    }
    window.grabbagQuipsMatchup = key;
    row.classList.add("is-arriving");
  }

  if (!window.grabbagQuipsSwap) {
    window.grabbagQuipsSwap = true;
    document.body.addEventListener("htmx:afterSwap", function () {
      bindComposeCards();
      markMatchup();
      markVoteClock();
      markLastClock();
      markWinner();
    });
  }
  if (!window.grabbagQuipsViewport) {
    window.grabbagQuipsViewport = true;
    window.addEventListener("resize", syncVisualViewport);
    window.addEventListener("orientationchange", syncVisualViewport);
    if (window.visualViewport) {
      window.visualViewport.addEventListener("resize", syncVisualViewport);
    }
  }
  syncVisualViewport();
  bindComposeCards();
  markMatchup();
  markVoteClock();
  markLastClock();
  markWinner();
})();
