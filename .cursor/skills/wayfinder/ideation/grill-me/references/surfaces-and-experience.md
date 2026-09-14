# Surfaces & experience

## Quick triage (read every session)

Skip this zone when there is no human-visible or operator-visible touchpoint (pure library internals, batch-only with no UI/CLI/reporting changes, invisible refactor). Still confirm nothing downstream renders errors or admin views.

**Common skip signals**

- Change is intentionally **invisible** to users and operators, and **no** docs, notifications, or exports change.
- **Headless** integration (API/machine-only) with **no** dashboard, email, or PDF impact.
- You are only moving code **within** a layer that already has a stable external contract.

If **any** new or changed **screen, message, document, command, or help text** exists, this zone is **in scope** - do not mark N/A without stating what the surface is.

## Mandatory layout pass (UI/UX - when not N/A)

This zone **includes UI/UX**: composition, hierarchy, and layout - not only copy and states.

Before treating **Surfaces & experience** as settled for product UI, the session **must** capture a **rough layout for every layout-bearing surface** in scope: each **page/route**, **modal/dialog**, **drawer/sheet**, **side panel**, **wizard step** (each step counts as its own surface if layout differs), **full-screen takeover**, or **similar chrome-heavy container**. For each, the user should describe **where** the main blocks live (header, body, rails, footers), **where** primary vs secondary actions sit, **what** scrolls vs stays fixed, and any **structural** decision that would change implementation (e.g. split view vs single column, table above fold vs below filters).

Accept **ASCII**, **labeled zones**, or **tight prose** - precision is about **spatial commitments**, not visuals. If a surface is **explicitly deferred**, record **which** surface and **what default layout** you will assume until designed.

Only after **each** such surface is described or deferred should you move on to other deep prompts in this zone (states, flows, a11y, etc.), unless you are depth-first on **one** surface’s layout and temporarily postponing siblings - still **one question per turn**.

## Deep prompts (load when not N/A)

- **Who** sees what, in which contexts (roles, admin vs self-serve, internal vs external)?
- **States:** loading, empty, success, validation errors, permission denied, partial failure, offline/slow - what should each surface show?
- **Flows:** entry points, exit points, and how this alters **existing** UX paths (dead ends, extra steps, changed affordances).
- **Information architecture:** where does this live in navigation; does it need discoverability work?
- **Accessibility & usability:** keyboard, focus, semantics, contrast, motion; error message clarity.
- **Localization:** copy constraints, pluralization, variable order, RTL if relevant.

### Web app overlay (use when relevant)

- **Enumerate layout-bearing surfaces** first (pages, modals, drawers, etc.); then **rough layout per surface** - see **Mandatory layout pass**.
- **Routes/pages** affected; **new vs reused** components; layout constraints (grids, modals, drawers).
- **Responsive:** mobile vs tablet vs desktop - what differs (information priority, actions, breakpoints)?
- **SSR/CSR** or **hydration** assumptions that affect UX or SEO snippets.

### Non-web examples

- **CLI:** flags, defaults, `--help`, progressive output, non-interactive mode.
- **Docs/help center:** what must be updated so users/operators succeed.
