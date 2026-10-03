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

  if (!window.grabbagApplesScroll) {
    window.grabbagApplesScroll = true;
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
  function boardGrid(count) {
    if (count < 6) {
      return { cols: count, rows: 1 };
    }
    var cols = Math.ceil(count / 2);
    if (cols > 7) {
      cols = 7;
    }
    return { cols: cols, rows: 2 };
  }

  function twoRowCardHeight(slots) {
    var style = getComputedStyle(slots);
    var pad = (parseFloat(style.paddingTop) || 0) + (parseFloat(style.paddingBottom) || 0);
    var gap = parseFloat(style.rowGap);
    if (!gap) {
      gap = parseFloat(style.gap) || 0;
    }
    return Math.max(0, Math.floor((slots.clientHeight - pad - gap) / 2));
  }

  function fitBoardSlot(slot) {
    var copy = slot.querySelector(".apples-slot-copy") || slot;
    var root = parseFloat(getComputedStyle(document.documentElement).fontSize) || 16;
    var minPx = Math.max(12, Math.round(root * 1.15));
    var maxPx = Math.round(root * 4);
    function overflows() {
      return copy.scrollHeight > copy.clientHeight + 1 || copy.scrollWidth > copy.clientWidth + 1;
    }
    function search() {
      var lo = minPx;
      var hi = maxPx;
      var best = minPx;
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
    }
    copy.style.overflowWrap = "normal";
    search();
    if (copy.scrollWidth > copy.clientWidth + 1) {
      copy.style.overflowWrap = "break-word";
    }
  }

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

  function layoutBoardCards() {
    var slots = document.querySelector(".apples-tv .apples-slots");
    if (!slots) {
      return;
    }
    var cards = slots.querySelectorAll(":scope > .apples-slot");
    var count = cards.length;
    if (!count) {
      return;
    }
    var grid = boardGrid(count);
    var cols = grid.cols;
    var rows = grid.rows;
    var avail = slots.clientWidth;
    var cap = 0;
    if (count === 1) {
      cap = Math.min(avail * 0.5, 780);
    } else if (count === 2) {
      cap = Math.min(avail * 0.38, 640);
    } else if (count <= 4) {
      cap = Math.min(avail * 0.3, 500);
    }
    if (cap > 0) {
      slots.style.gridTemplateColumns = "repeat(" + cols + ", minmax(0, " + Math.floor(cap) + "px))";
    } else {
      slots.style.gridTemplateColumns = "repeat(" + cols + ", minmax(0, 1fr))";
    }
    if (rows === 1) {
      slots.style.alignContent = "center";
      slots.style.gridTemplateRows = twoRowCardHeight(slots) + "px";
    } else {
      slots.style.alignContent = "stretch";
      slots.style.gridTemplateRows = "repeat(2, minmax(0, 1fr))";
    }
    cards.forEach(fitBoardSlot);
  }

  function watchBoardCards() {
    layoutBoardCards();
    var center = document.querySelector(".apples-tv .apples-board-center");
    if (!center || !window.ResizeObserver) {
      return;
    }
    if (window.grabbagApplesBoardObserver) {
      window.grabbagApplesBoardObserver.disconnect();
    }
    window.grabbagApplesBoardObserver = new ResizeObserver(layoutBoardCards);
    window.grabbagApplesBoardObserver.observe(center);
  }

  if (!window.grabbagApplesBoardLayout) {
    window.grabbagApplesBoardLayout = true;
    window.addEventListener("resize", layoutBoardCards);
    document.body.addEventListener("htmx:afterSwap", function (evt) {
      var el = evt.detail && evt.detail.elt;
      if (el && el.id === "apples-board") {
        watchBoardCards();
        markJudgeArrival();
        markVoteCue();
      }
    });
    if (document.fonts && document.fonts.ready) {
      document.fonts.ready.then(layoutBoardCards);
    }
  }
  syncVisualViewport();
  watchBoardCards();
  markJudgeArrival();
  markVoteCue();
})();
