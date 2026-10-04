// Testing registers with the shells. It is all markup and htmx, so both
// mounts are empty; they are here as the smallest working tenant.
(function () {
  "use strict";

  grabbagShell.register("testing", {
    board: { mount: function () {} },
    phone: { mount: function () {} },
  });
})();
