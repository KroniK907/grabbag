# One-off reference

---

## When to use

| Path | Use when |
|------|----------|
| **one-off** | Map **To Do** ticket with **repo deliverables** (skill folder, code, docs in target repo) - one session or thin vertical slice |
| **define-bundle → create-tasks → implement-task** | Multiple GM rows, shared bundle branch, **Implementing** frontier, AFK eligibility |
| **Agent checklist or human** | Trivial map errands - no repo deliverables (retitle ticket, add label, post comment, edit map prose) |

**Scope gate:** Human declares one-off intent. No ecosystem checklist beyond "is this map-scoped repo work?"

---

## Wrong-entry redirect

**implement-task** stops on To Do tickets without an approved bundle parent:

| Symptom | Fix |
|---------|-----|
| User says "implement task #N" on a **To Do** row | Redirect to **one-off** |
| Startup fails: missing bundle parent, bundle Status not `approved` | Re-enter via **one-off** - do not patch implement-task gates |
| User wants bundle pipeline | Hand off to [define-bundle](../../actions/define-bundle/SKILL.md) |

Narrate: *This ticket is on map **To Do** with no bundle - use **one-off** for map-scoped implementation, or **define-bundle** when the work belongs in an approved bundle.*

---

## Ticket draft (chat-only)

Present before `gh issue create`. Pause for human review.

```markdown
## Draft one-off ticket

**Map:** [{FeatureName}:Map](map-url)
**Title:** Task: {short name}

## Question

<One line - what this session ships.>

## Done when

- [ ] <verifiable deliverable 1>
- [ ] <verifiable deliverable 2>

## Method

{skill-name}

<One line - why this Method (from `wayfinder/` or `wayfinder/actions/`; repo-root one-offs only when human sets explicitly).>

## Map

Parent: [{FeatureName}:Map](map-url)

**Type:** task | **Mode:** HITL

## Blocked by

<!-- Omit when none. Open blocker issues stop the run at startup. -->

- [#N Blocker title](url)

## Labels (on materialize)

`wf:todo` - `wf:task` - `wf:hitl` - `wf:approved`

## Status (on materialize)

`ready`

## Proposed branch

`one-off/{issue-num}-{slug}` - slug from ticket title, kebab-case, ≤4 words
```

End with: *Review the draft - reply **draft approved** to materialize and build, or request edits.*

---

## Ticket template

GitHub issue body after materialize. Title: `Task: {short name}`.

```markdown
**Status:** ready

**Branch:** one-off/{issue-num}-{slug}

## Question

<What this session ships.>

## Done when

- [ ] …

## Method

{skill-name}

## Map

Parent: [{FeatureName}:Map](map-url)

## Blocked by

<!-- Optional - open issues stop the run at startup -->
```

**No Parent bundle:** section. Ticket stays on map **To Do** for the entire run.

---

## implement-task gate waivers

When entered via **one-off**, apply these overrides to [implement-task startup gates](../implement-task/REFERENCE.md#startup-gates). **Waivers apply only on the one-off path** - implement-task skill files are not edited.

| Gate | Standard implement-task | One-off waiver |
|------|-------------------------|----------------|
| Bundle parent | Required; Status `approved` | **Waived** - no bundle |
| Label **`wf:approved`** | Required at pickup | **Expected at materialize** - one-off adds it before build tail |
| Bundle branch | Checkout `afk/bundle-*` from bundle **Branch:** | **Waived** - use ticket **Branch:** (`one-off/{issue-num}-{slug}`) |
| **Blocked by** | Stops on open blockers | **Kept** - no waiver |
| AFK serial | AFK only | **N/A** - HITL only |
| Method validation | Per implement-task | **Kept** |
| Code review | After Method | **Kept** |
| Close task / remove **`wf:approved`** | Never | **Kept** |
| Reconcile phrases | Never posted by agent | **Kept** |

Run the full implement-task tail: Method → code-review → push → resolution → **`awaiting-reconcile`**.

---

## Git branch

| Rule | Detail |
|------|--------|
| Pattern | `one-off/{issue-num}-{slug}` |
| Base | Map **Dev branch:** line when present; else remote `dev`, then `develop`, then default branch |
| **Branch:** line | Write on ticket body at branch creation |
| Never | Commit directly to `main` / dev base; never use `afk/bundle-*` |

```powershell
git fetch origin
git checkout <dev-base>          # or main when no dev branch
git pull origin <dev-base>
git checkout -b one-off/<num>-<slug>
git push -u origin one-off/<num>-<slug>
```

---

## Resolution comment (one-off variant)

Post on the one-off ticket at end-of-run. Same structure as [implement-task success template](../implement-task/references/resolution-comment.md#success-template) with these deltas:

- **Task** line only - omit **Bundle:** (no parent bundle)
- **Next** - review diff on ticket **Branch:**, not bundle branch
- All other sections unchanged: Summary, Method, Code review, Commits, Done when, Reconcile

After posting: set **Status:** `awaiting-reconcile`; add **`wf:needs-review`**; keep **`wf:approved`**.

**Blocked runs:** Use implement-task [Blocked template](../implement-task/references/resolution-comment.md#blocked-template) - omit bundle URL; do not set **`awaiting-reconcile`**.

---

## Reconcile handoff

Owned by [wayfinder](../../SKILL.md) **Reconcile**, not one-off.

| Topic | One-off behavior |
|-------|------------------|
| Map table | **To Do → Completed** gist - ticket never entered **Implementing** |
| Close ticket | Human **`Approved - reconcile and close`** |
| **`wf:approved`** | Removed by Reconcile on close |
| Decision coverage | **No `implemented` updates** unless ticket body explicitly references GM rows |
| Resolution type | Holistic Reconcile (session close) - same approval phrases as other map tickets |
| Map body edits | Follow [wayfinder REFERENCE § Map body edits](../../REFERENCE.md#map-and-issue-body-edits-reconcile) - draft file, validate, `--body-file` |

After **`awaiting-reconcile`**, suggest wayfinder **Reconcile** - not another implementation pass unless human resets **Status** to **`ready`**.

---

## Worked example (generalized)

**Situation:** Map `{FeatureName}:Map` has a **To Do** row - ship a new sibling skill folder while planning tickets remain open.

1. **Declare** - Human: "One-off: add `{skill-name}` skill on `{FeatureName}:Map`."
2. **Draft** - Agent posts chat draft (Question, Done when, **## Method** `{skill-name}` or `writing-for-agents` for meta skills, proposed branch `one-off/{N}-{skill-name}`).
3. **Approve** - Human: **`draft approved`**.
4. **Materialize** - `gh issue create` with labels; **Status:** `ready`; **To Do** row on map; **`wf:approved`** added.
5. **Branch** - `git checkout -b one-off/{N}-{skill-name}`; persist **Branch:** on ticket.
6. **Build** - Follow task **## Method** skill; record pre-Method SHA.
7. **Code review** - implement-task mode on diff since pre-Method SHA.
8. **Push** - commit on one-off branch; push.
9. **Resolve** - post resolution comment; **Status:** `awaiting-reconcile`; **`wf:needs-review`**.
10. **Reconcile** - Human **`Approved - reconcile and close`** → Completed gist; close ticket.

**Not this example:** Trivial "add a Note line to the map" - use agent checklist, not one-off.

---

## Route trigger (for wayfinder)

Suggest **one-off** when:

- Map **To Do** row is type **`task`** with repo deliverables
- User declares one-off intent or says "ship X on this map" without bundling
- User hit implement-task gates on a To Do ticket - redirect here

Do **not** suggest one-off for:

- **`Implementing`** rows - use [implement-task](../implement-task/SKILL.md)
- Trivial checklist errands
- AFK pickup (one-off is HITL-only permanently)

---

## Design defaults

| Topic | Default |
|-------|---------|
| Mode | HITL only - labels `wf:hitl`; never `wf:afk` |
| Map placement | **To Do** for entire lifecycle |
| Bundle pipeline | Skipped - no `wf:bundle`, no **Implementing** row |
| implement-task files | Unchanged - waivers documented here only |
| Skill docs | Generalized placeholders - no live issue numbers or decision-log IDs in committed markdown |
