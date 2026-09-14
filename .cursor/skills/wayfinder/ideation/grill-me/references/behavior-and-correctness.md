# Behavior & correctness

## Quick triage (read every session)

Skip this zone when behavior is literally unchanged (pure rename/move with identical semantics), or the work is **only** documentation of existing behavior. Even “small” features usually belong here.

**Common skip signals**

- **No** new scenarios, rules, or state transitions - only rearranging code without changing outcomes.
- Purely **presentational** tweak with zero validation or side-effect changes (rare; verify).

If **any** **if/else**, **validation**, **status machine**, **retry**, **ordering**, or **consistency** rule is introduced or altered, this zone is **in scope**.

## Deep prompts (load when not N/A)

- **Happy paths:** enumerated scenarios from trigger to outcome (by role if needed).
- **Edge cases:** duplicates, conflicts, partial inputs, race timing, double-submit, stale data, idempotency.
- **Invariants:** what must always be true before/after; what breaks if violated.
- **Interactions with existing features:** overlaps, feature flags, deprecated paths, migration from old behavior.
- **Concurrency:** what happens with parallel requests/jobs; locking vs last-write-wins.
- **Failure behavior:** compensating actions, partial commits, user-visible vs silent failures.

### Web app overlay (use when relevant)

- **Client vs server** truth: who validates what; optimistic UI vs confirmed state.
- **Sessions/cookies** and multi-tab scenarios if they affect correctness.

### Non-web examples

- **Workers/cron:** at-least-once delivery, batch boundaries, poison messages.
- **Libraries:** public API contracts, error types, backward compatibility of outputs.
