(function () {
  "use strict";

  function clock(sec) {
    sec = Math.max(0, Math.floor(sec));
    return Math.floor(sec / 60) + ":" + String(sec % 60).padStart(2, "0");
  }

  function paint() {
    var now = Date.now() / 1000;
    document.querySelectorAll("[data-bt-timer]").forEach(function (el) {
      var left = Number(el.dataset.seconds) || 0;
      var paused = el.closest(".is-paused");
      if (!window.grabbagStatic && !paused) {
        left = (Number(el.dataset.end) || 0) - now;
      }
      var value = el.querySelector("[data-bt-timer-value]");
      if (value) {
        value.textContent = clock(Math.ceil(left));
      }
    });
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

  if (!window.btPhotoBound) {
    window.btPhotoBound = true;
    document.addEventListener("change", function (ev) {
      if (ev.target.matches && ev.target.matches("[data-bt-photo-input]")) {
        uploadPhoto(ev.target);
      }
    });
  }

  if (!window.btTimer) {
    window.btTimer = window.setInterval(paint, 250);
    document.body.addEventListener("htmx:afterSwap", function () {
      paint();
      settle();
    });
  }
  paint();
  settle();
})();
