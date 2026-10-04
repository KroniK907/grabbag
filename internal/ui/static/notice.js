// Host toasts. The server sends notice JSON {target, type, message, duration}
// on the room stream. A page shows a notice only when its target is in the
// space-separated data-notice-targets on #notice-targets. duration is seconds:
// 0 stays until closed, negative is 3s.
//
// window.grabbagEnqueueNotice(msg) queues a room notice (target filtered).
// window.grabbagShowNotice(msg) queues a local notice with no target check.
// window.grabbagHideNotice(key) drops a local notice queued with that key.
(function () {
  "use strict";

  if (window.grabbagEnqueueNotice) {
    return;
  }

  var waiting = [];
  var showing = null;
  var timer = null;

  function targets() {
    var el = document.getElementById("notice-targets");
    if (!el) return [];
    return (el.getAttribute("data-notice-targets") || "").split(/\s+/).filter(Boolean);
  }

  function lookFor(type) {
    if (type === "error" || type === "warning" || type === "info") return type;
    return "info";
  }

  function hide() {
    var root = document.getElementById("notice-root");
    if (root) root.hidden = true;
    showing = null;
    if (timer) {
      clearTimeout(timer);
      timer = null;
    }
  }

  function paint(msg) {
    var root = document.getElementById("notice-root");
    var toast = document.getElementById("notice-toast");
    var text = document.getElementById("notice-message");
    if (!root || !toast || !text) return;
    showing = msg;
    toast.setAttribute("data-notice-type", msg.type || "info");
    toast.setAttribute("data-notice-look", lookFor(msg.type));
    toast.setAttribute("role", lookFor(msg.type) === "error" ? "alert" : "status");
    text.textContent = msg.message || "";
    root.hidden = false;
    if (timer) clearTimeout(timer);
    timer = null;
    var sec = +msg.duration;
    if (sec < 0 || isNaN(sec)) sec = 3;
    if (sec > 0) {
      timer = setTimeout(function () {
        hide();
        showNext();
      }, sec * 1000);
    }
  }

  function showNext() {
    if (waiting.length === 0) return;
    paint(waiting.shift());
  }

  function queue(msg) {
    if (!showing) {
      paint(msg);
      return;
    }
    waiting.push(msg);
    if (waiting.length > 4) waiting.shift();
  }

  window.grabbagEnqueueNotice = function (msg) {
    if (!msg || !msg.target || !msg.message) return;
    if (targets().indexOf(msg.target) < 0) return;
    queue(msg);
  };

  window.grabbagShowNotice = function (msg) {
    if (!msg || !msg.message) return;
    queue(msg);
  };

  window.grabbagHideNotice = function (key) {
    waiting = waiting.filter(function (msg) {
      return msg.key !== key;
    });
    if (showing && showing.key === key) {
      hide();
      showNext();
    }
  };

  function start() {
    var closeBtn = document.getElementById("notice-close");
    if (closeBtn) {
      closeBtn.addEventListener("click", function () {
        hide();
        showNext();
      });
    }
    var match = document.cookie.match(/(?:^|; )grabbag_notice=([^;]*)/);
    if (match) {
      document.cookie = "grabbag_notice=; Max-Age=0; Path=/";
      try {
        window.grabbagEnqueueNotice(JSON.parse(decodeURIComponent(match[1])));
      } catch (e) {}
    }
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", start);
  } else {
    start();
  }
})();
