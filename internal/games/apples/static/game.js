// Apples for Humanity tenant. The board cues a new judge and the favorite
// vote. The phone keeps the hand's scroll across repaints. Card layout and
// text size are CSS (game.css), and the shell tracks the visual viewport.
(function () {
  "use strict";

  function markVoteCue() {
    if (window.grabbagStatic) {
      return;
    }
    var cue = document.querySelector(".apples-tv .apples-vote-cue");
    if (!cue) {
      return;
    }
    var key = cue.getAttribute("data-vote-cue") || "";
    if (window.grabbagApplesVoteCue === key) {
      return;
    }
    window.grabbagApplesVoteCue = key;
    cue.hidden = false;
    cue.classList.add("is-on");
  }

  function markJudgeArrival() {
    var roster = document.querySelector(".apples-tv .apples-roster");
    if (!roster) {
      return;
    }
    var id = roster.getAttribute("data-judge") || "";
    var seat = roster.querySelector(".apples-judge-seat");
    if (!seat || !id) {
      return;
    }
    if (window.grabbagApplesJudge && window.grabbagApplesJudge !== id) {
      seat.classList.add("is-arriving");
    }
    window.grabbagApplesJudge = id;
  }

  function markBoard() {
    markJudgeArrival();
    markVoteCue();
  }

  grabbagShell.register("apples", {
    board: {
      mount: function (root, ctx) {
        ctx.on(document.body, "htmx:afterSwap", function (evt) {
          var el = evt.detail && evt.detail.elt;
          if (el && el.id === "apples-board") {
            markBoard();
          }
        });
        markBoard();
      },
    },
    phone: {
      mount: function (root, ctx) {
        var savedScroll = 0;
        // Keep the hand scrolled where it was across a phone repaint.
        ctx.on(document.body, "htmx:beforeSwap", function (evt) {
          var target = evt.detail && evt.detail.target;
          if (!target || target.id !== "apples-phone") {
            return;
          }
          var scroll = target.querySelector(".apples-hand-scroll") || target.querySelector(".apples-phone-scroll");
          if (scroll) {
            savedScroll = scroll.scrollTop;
          }
        });
        ctx.on(document.body, "htmx:afterSwap", function (evt) {
          var el = evt.detail && evt.detail.elt;
          if (!el || el.id !== "apples-phone") {
            return;
          }
          var scroll = el.querySelector(".apples-hand-scroll") || el.querySelector(".apples-phone-scroll");
          if (scroll) {
            scroll.scrollTop = savedScroll;
          }
        });
      },
    },
  });
})();
