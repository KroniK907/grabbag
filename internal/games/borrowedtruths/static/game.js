// Borrowed Truths tenant: the host's elapsed clock, slap animations that
// settle on a repaint of the same moment, the opening theme and intro on the
// board, the writing music, and phone photo uploads.
(function () {
  "use strict";

  function clock(sec) {
    sec = Math.max(0, Math.floor(sec));
    return Math.floor(sec / 60) + ":" + String(sec % 60).padStart(2, "0");
  }

  // paint runs the host's elapsed clock. Phase timers are the shared
  // countdown script.
  function paint() {
    var now = Date.now() / 1000;
    document.querySelectorAll("[data-bt-elapsed]").forEach(function (el) {
      var start = Number(el.dataset.start) || 0;
      el.textContent = start > 0 && !window.grabbagStatic ? clock(now - start) : "";
    });
  }

  // settle stops the slap animations when a swap repaints the same moment,
  // so a vote lock does not re-slap the card.
  function settle() {
    var board = document.getElementById("bt-board");
    if (!board) {
      return;
    }
    var key = board.dataset.moment || "";
    if (window.grabbagStatic || window.btMoment === key) {
      board.classList.add("is-settled");
    }
    window.btMoment = key;
  }

  // The intro runs once, as the game opens, timed to the theme. The facts
  // phase repaints the board on every submit, so the overlay lives outside
  // #bt-board. It is keyed by the phase start so a remount does not replay it.
  // The theme is 110 bpm in 4/4 and opens on beat 3 of bar 1.
  var introSec = 22.9;
  var introWindow = 15;

  // Four scenes on one spot: the rules, a truth, a lie, the question.
  // game.css times each piece in seconds of the theme.
  var introHTML =
    '<div class="bt-intro-scene is-rules">' +
    '<div class="bt-intro-logo"><span class="bt-intro-in">BORROWED</span><span class="bt-intro-in">TRUTHS</span></div>' +
    '<div class="bt-intro-cards">' +
    '<div class="bt-intro-card bt-intro-in is-true">TRUTH</div>' +
    '<div class="bt-intro-card bt-intro-in is-true">TRUTH</div>' +
    '<div class="bt-intro-card bt-intro-in is-lie">LIE</div>' +
    "</div>" +
    '<div class="bt-tape bt-intro-tape bt-intro-in">ONE OF THEM IS BORROWED</div>' +
    "</div>" +
    '<div class="bt-intro-scene is-truth">' +
    '<div class="bt-card bt-intro-fact bt-intro-in"><p>I once got stuck in a revolving door for an hour.</p>' +
    '<b class="bt-intro-stamp bt-intro-in is-true">TRUTH</b></div>' +
    "</div>" +
    '<div class="bt-intro-scene is-lie">' +
    '<div class="bt-card bt-intro-fact bt-intro-in"><p>I can name every country in Africa.</p>' +
    '<b class="bt-intro-stamp bt-intro-in is-lie">LIE</b></div>' +
    "</div>" +
    '<div class="bt-intro-scene is-ask">' +
    '<div class="bt-intro-ask bt-intro-in">Who\'s telling the truth?</div>' +
    '<div class="bt-intro-ask bt-intro-in is-lie">Who\'s lying?</div>' +
    "</div>";

  // intro returns a promise that resolves when the theme has finished, or
  // at once when there is no intro to play.
  function intro(root, ctx) {
    var board = document.getElementById("bt-board");
    var started = board ? Number(board.dataset.intro) || 0 : 0;
    if (!started || Date.now() / 1000 - started > introWindow || window.btIntro === started) {
      return Promise.resolve();
    }
    window.btIntro = started;
    var el = document.createElement("div");
    el.className = "bt-intro bt-wall";
    el.style.setProperty("--bt-intro", introSec + "s");
    el.innerHTML = introHTML;
    var shown = false;
    var show = function () {
      if (shown) {
        return;
      }
      shown = true;
      root.appendChild(el);
      ctx.after(introSec * 1000, function () {
        el.remove();
      });
    };
    if (!ctx.audio) {
      show();
      return Promise.resolve();
    }
    ctx.audio.define({
      files: { theme: "audio/theme.ogg" },
      regions: { theme: { file: "theme" } },
      stings: { theme: { region: "theme", layer: "music" } },
    });
    // Start the picture with the sound, but a slow decode does not hold the
    // intro back for more than a moment.
    var done = ctx.audio.ready().then(function () {
      ctx.audio.sting("theme");
      show();
      return new Promise(function (resolve) {
        ctx.after(introSec * 1000, resolve);
      });
    });
    ctx.after(1500, show);
    return done;
  }

  // The writing music plays while players write their facts. Five stems of
  // one 8-bar loop at 100 bpm; each variant is the set of stems heard. The
  // free timeline walks the arrangement and repeats it every 138 bars.
  // "break" is 2 bars of drums alone. It and the variant after it restart
  // every stem from the top of the loop, so the next section lands on bar 1.
  var writingSpec = {
    files: {
      drums: "audio/timer-drums.ogg",
      bass: "audio/timer-bass.ogg",
      low: "audio/timer-guitar-low.ogg",
      riff: "audio/timer-guitar-riff.ogg",
      high: "audio/timer-guitar-high.ogg",
    },
    regions: {
      drums: { file: "drums" },
      bass: { file: "bass" },
      low: { file: "low" },
      riff: { file: "riff" },
      high: { file: "high" },
      drumBreak: { file: "drums", start: 0, end: 4.8 },
    },
    cues: {
      writing: {
        layer: "music", bpm: 100, beatsPerBar: 4, end: { at: "bar" },
        variants: {
          groove: ["drums", "bass"],
          low: ["drums", "bass", "low"],
          riff: ["drums", "bass", "low", "riff"],
          "break": ["drumBreak"],
          full: ["drums", "bass", "low", "riff", "high"],
          quiet: ["bass", "low"],
        },
        timelines: {
          writing: {
            kind: "free",
            loop: 138,
            script: {
              0: { to: "groove", over: 0.05 },
              16: { to: "low", over: 0.05 },
              32: { to: "riff", over: 0.05 },
              48: { to: "break", over: 0.05 },
              50: { to: "full", over: 0.05 },
              82: { to: "quiet", over: 0.05 },
              90: { to: "low", over: 0.05 },
              98: { to: "riff", over: 0.05 },
              114: { to: "full", over: 0.05 },
              130: { to: "low", over: 0.05 },
            },
          },
        },
      },
    },
  };

  // writingMusic starts the writing music in the facts phase, after the
  // theme, and ends it on the next bar when the phase moves on. While the
  // game is paused the music sounds muffled, and the arrangement keeps going
  // so it stays on the loop.
  function writingMusic(root, ctx, themeDone) {
    if (!ctx.audio) {
      return;
    }
    ctx.audio.define(writingSpec);
    var cue = ctx.audio.cue("writing");
    var wanted = false;
    var muffled = false;
    var held = function () {
      return false;
    };
    var sync = function () {
      var board = document.getElementById("bt-board");
      var on = !!board && board.classList.contains("phase-facts");
      var paused = !!board && board.classList.contains("is-paused");
      if (paused !== muffled) {
        muffled = paused;
        ctx.audio.snapshot(paused ? "telephone" : null, 0.4);
      }
      if (on && !wanted) {
        wanted = true;
        themeDone.then(function () {
          if (wanted && !cue.playing) {
            cue.start({ timeline: "writing", held: held });
          }
        });
      } else if (!on) {
        wanted = false;
        if (cue.playing) {
          cue.end({ at: "bar" });
        }
      }
    };
    // The board's new classes land at settle. At afterSwap htmx still shows
    // the old ones for its transitions.
    ctx.on(document.body, "htmx:afterSettle", sync);
    sync();
  }

  // Photos are downscaled to 1600px on the long edge and re-encoded as JPEG
  // before upload. That keeps them small on room Wi-Fi and strips EXIF.
  var maxEdge = 1600;
  var maxBytes = 2 * 1024 * 1024;

  function encode(img, quality) {
    var scale = Math.min(1, maxEdge / Math.max(img.naturalWidth, img.naturalHeight));
    var canvas = document.createElement("canvas");
    canvas.width = Math.round(img.naturalWidth * scale);
    canvas.height = Math.round(img.naturalHeight * scale);
    canvas.getContext("2d").drawImage(img, 0, 0, canvas.width, canvas.height);
    return new Promise(function (resolve) {
      canvas.toBlob(resolve, "image/jpeg", quality);
    });
  }

  function uploadPhoto(input) {
    var card = input.closest("[data-bt-photo]");
    var status = card.querySelector("[data-bt-photo-status]");
    var preview = card.querySelector("[data-bt-photo-preview]");
    var file = input.files && input.files[0];
    if (!file) {
      return;
    }
    status.textContent = "Shrinking the photo…";
    var url = URL.createObjectURL(file);
    var img = new Image();
    img.onerror = function () {
      status.textContent = "That file is not a photo this phone can open.";
      URL.revokeObjectURL(url);
    };
    img.onload = function () {
      encode(img, 0.85)
        .then(function (blob) {
          return blob && blob.size > maxBytes ? encode(img, 0.6) : blob;
        })
        .then(function (blob) {
          if (!blob) {
            throw new Error("This phone could not read that photo.");
          }
          status.textContent = "Uploading…";
          var body = new FormData();
          body.append("photo", blob, "photo.jpg");
          return fetch("/play/photo", { method: "POST", body: body, credentials: "same-origin" });
        })
        .then(function (res) {
          if (res.ok) {
            status.textContent = "Photo added.";
            card.querySelector(".bt-photo-pick").firstChild.textContent = "Change photo";
            preview.src = url;
            preview.hidden = false;
            return;
          }
          return res.text().then(function (text) {
            throw new Error(text.trim() || "Could not upload the photo.");
          });
        })
        .catch(function (err) {
          status.textContent = err.message;
        })
        .finally(function () {
          input.value = "";
        });
    };
    img.src = url;
  }

  // Both surfaces run the host's elapsed clock and settle the slap
  // animations. The phone also takes photo uploads.
  function mount(root, ctx) {
    ctx.every(250, paint);
    ctx.on(document.body, "htmx:afterSwap", function () {
      paint();
      settle();
    });
    paint();
    settle();
  }

  grabbagShell.register("borrowedtruths", {
    board: {
      mount: function (root, ctx) {
        mount(root, ctx);
        writingMusic(root, ctx, intro(root, ctx));
      },
    },
    phone: {
      mount: function (root, ctx) {
        ctx.on(document, "change", function (ev) {
          if (ev.target.matches && ev.target.matches("[data-bt-photo-input]")) {
            uploadPhoto(ev.target);
          }
        });
        mount(root, ctx);
      },
    },
  });
})();
