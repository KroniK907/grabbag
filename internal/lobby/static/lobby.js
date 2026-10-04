// Lobby tenant. Most of the Lobby is markup and htmx, so mount adds Escape
// to close whatever panel is open (the host drawer or the game library),
// and on the board it plays the lobby music. ctx.on drops the listener and
// the shell releases the music when it unmounts the Lobby.
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

  // lobbyMusic loops one track on the Music layer. A Start fades it out with
  // the board; an End starts it again from the top, fading in.
  function lobbyMusic(ctx) {
    if (!ctx.audio) {
      return;
    }
    ctx.audio.define({
      files: { lobby: "audio/lobby.ogg" },
      regions: {
        lobby: { file: "lobby", start: 0, end: 16, loopStart: 0, loopEnd: 16 },
      },
      cues: {
        lobby: { layer: "music", bpm: 120, beatsPerBar: 4, variants: { main: ["lobby"] } },
      },
    });
    ctx.audio.cue("lobby").start({ over: 1.5 });
  }

  grabbagShell.register("lobby", {
    board: {
      mount: function (root, ctx) {
        closeOnEscape(root, ctx);
        lobbyMusic(ctx);
      },
    },
    phone: { mount: closeOnEscape },
  });
})();
