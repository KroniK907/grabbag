// Quick Quips tenant. The board marks a new matchup, clock, or winner as it
// arrives. The phone keeps the quip character counters live while typing.
// The shell tracks the visual viewport (--shell-visual-height).
(function () {
  "use strict";

  function runeLen(text) {
    return Array.from(text).length;
  }

  function bindComposeCards(root) {
    root.querySelectorAll(".quips-compose-card textarea").forEach(function (ta) {
      if (ta.dataset.quipsComposeBound === "1") {
        return;
      }
      ta.dataset.quipsComposeBound = "1";
      // The textarea goes away with its swap, and its listener with it.
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
        var scroll = root.querySelector("#quips-phone .quips-phone-scroll");
        if (scroll && scroll.querySelector(".quips-compose-card")) {
          var alert = scroll.querySelector(".ui-alert");
          if (alert) {
            alert.remove();
          }
        }
      });
    });
  }

  // arrive adds cls to the first el matching selector the first time its
  // key attribute takes a new value, so a repaint of the same moment does
  // not replay the animation.
  var seen = {};
  function arrive(root, selector, attr, cls) {
    var el = root.querySelector(selector);
    if (!el) {
      return;
    }
    var key = el.getAttribute(attr) || "";
    if (seen[selector] === key) {
      return;
    }
    seen[selector] = key;
    el.classList.add(cls);
  }

  function markBoard(root) {
    if (window.grabbagStatic) {
      return;
    }
    arrive(root, ".quips-quip-row.is-matchup", "data-matchup", "is-arriving");
    arrive(root, ".quips-board-clock[data-vote-clock]", "data-vote-clock", "is-fading");
    arrive(root, ".quips-board-clock[data-last-clock]", "data-last-clock", "is-arriving");
    arrive(root, ".quips-final[data-winner]", "data-winner", "is-arriving");
  }

  grabbagShell.register("quips", {
    board: {
      mount: function (root, ctx) {
        ctx.on(document.body, "htmx:afterSwap", function () {
          markBoard(root);
        });
        markBoard(root);
      },
    },
    phone: {
      mount: function (root, ctx) {
        ctx.on(document.body, "htmx:afterSwap", function () {
          bindComposeCards(root);
        });
        bindComposeCards(root);
      },
    },
  });
})();
