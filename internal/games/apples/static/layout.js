// Apples for Humanity layout. game.css lays out the answer-card grid; this
// fits each card's text to its card, which CSS cannot do: the largest font
// size from 1.15rem to 4rem at which the copy does not overflow, breaking
// inside words only when one word is wider than the card. It only measures
// and sets styles, and the shell runs it (also in static previews).
(function () {
  "use strict";

  function fitCard(card) {
    var copy = card.querySelector(".apples-slot-copy") || card;
    var root = parseFloat(getComputedStyle(document.documentElement).fontSize) || 16;
    var lo = Math.max(12, Math.round(root * 1.15));
    var hi = Math.round(root * 4);
    var best = lo;
    function overflows() {
      return copy.scrollHeight > copy.clientHeight + 1 || copy.scrollWidth > copy.clientWidth + 1;
    }
    copy.style.overflowWrap = "normal";
    while (lo <= hi) {
      var mid = (lo + hi) >> 1;
      copy.style.fontSize = mid + "px";
      if (!overflows()) {
        best = mid;
        lo = mid + 1;
      } else {
        hi = mid - 1;
      }
    }
    copy.style.fontSize = best + "px";
    if (copy.scrollWidth > copy.clientWidth + 1) {
      copy.style.overflowWrap = "break-word";
    }
  }

  grabbagShell.layout("apples", {
    board: function (root) {
      root.querySelectorAll(".apples-tv .apples-slots > .apples-slot").forEach(fitCard);
    },
  });
})();
