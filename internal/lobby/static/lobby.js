// Lobby tenant. Most of the Lobby is markup and htmx, so mount only adds
// Escape to close whatever panel is open (the host drawer or the game
// library). ctx.on drops the listener when the shell unmounts the Lobby.
(function () {
  "use strict";

  function closeOnEscape(root, ctx) {
    ctx.on(document, "keydown", function (ev) {
      if (ev.key !== "Escape") {
        return;
      }
      root.querySelectorAll(".ui-drawer.is-open, .ui-game-library.is-open").forEach(function (el) {
        el.classList.remove("is-open");
      });
    });
  }

  grabbagShell.register("lobby", {
    board: { mount: closeOnEscape },
    phone: { mount: closeOnEscape },
  });
})();
