---
name: create-tasks
description: create-tasks, wf:bundle approved, bundle approved, scope approved, tasks approved, split bundle into tasks, implementation tasks from bundle, wayfinder Route create-tasks, wf:task draft
agent-config-sync: true
---

# Create tasks

Split an **approved** bundle issue (`wf:bundle`, Status `approved`) into **thin vertical slices** sized for one session or one PR as draft implementation issues, then on human approval sync map **Implementing** + **Decision coverage**. Does **not** implement product code unless the task itself is the work.

Runs on an approved bundle + parent map + decision log. After **`tasks approved`**, hand off to implementation; on ship, invoke [wayfinder](../SKILL.md) **Reconcile** to close the task and move coverage to **`implemented`**.

## Not this skill

| Skill | When instead |
|-------|----------------|
| [wayfinder](../../SKILL.md) | Chart, Materialize, Reconcile, Route only |
| [define-bundle](../define-bundle/SKILL.md) | Group GM rows into draft/approved bundles |
| [design-modules](../design-modules/SKILL.md) | Per-module design artifacts on bundle before splitting |
| [grill-me](../../ideation/grill-me/SKILL.md) | Resolve unknowns â†’ new binding GM rows |
| [write-a-prd](../../write-a-prd/SKILL.md) | Small map-free scope only |

## Prerequisites

- Approved bundle issue (`wf:bundle`, **Status:** `approved`)
- When bundle scope needs module shaping first, run [design-modules](../design-modules/SKILL.md) (HITL) and post module-design artifact comment(s) on the bundle - one per module when multiple apply; recommended, not required
- Parent map with **Implementing** table and **Decision coverage**
- `gh` authenticated on the target repo
- Labels `wf:task` or `wf:prototype`, `wf:hitl` or `wf:afk`, `wf:approved`

## Workflow

### 1. Load context

```text
gh issue view <bundle-num> --json body,title,url
gh issue view <map-num> --json body,title,url
```

From the bundle: **Decisions** (covered GM rows), **Constraints**, scope summary, boundaries, outcomes.

From the map: slug, decision log link, **Implementing**, **Decision coverage** for bundle-scoped GM IDs.

**Gate:** Bundle **Status** must be `approved`. If `draft`, hand off to [define-bundle](../define-bundle/SKILL.md) for **`bundle approved`**.

### 2. Propose split

Present to the user:

- **Task count** and titles (1-3 typical; more only when deliverables are clearly separable)
- **Per task:** what ships, which bundle outcomes it covers, HITL vs AFK, blocked-by if any
- **Rationale** - why this slice boundaries

**Split heuristics:** See [REFERENCE.md](REFERENCE.md#split-heuristics). Default **one task** when the bundle is meta/infra and fits one session (e.g. a single skill folder). Split when subsystems, dependencies, or session size clearly warrant it.

Wait for confirmation or edits before creating/updating task issues.

### 3. Draft task issues

Create early with `gh issue create` or update drafts in place.

| Field | Value |
|-------|--------|
| Title | `Task: {short name}` |
| Labels | `wf:task` or `wf:prototype` + `wf:hitl` or `wf:afk` |
| Body | Per [REFERENCE.md](REFERENCE.md#task-issue-template) - include **## Method** at draft |
| **Status** | `draft` |

**Parent bundle:** link in **Parent bundle** section (body link only; native sub-issues optional).

**Decisions:** copy bundle **Decisions** rows **verbatim** + relevant **Constraints** the task must honor.

Fill **What to build**, **## Method**, **Outcomes/stories covered**, **Done when**, **Blocked by**.

**Method (required at draft):** Propose **## Method** for every task when splitting - pick from `wayfinder/` or `wayfinder/actions/` skills (frontmatter `name`). Repo-root one-offs only when the human explicitly sets them. **AFK tasks** must have a valid **## Method** before **`wf:approved`**; [implement-task](../../orchestrators/implement-task/SKILL.md) stops without one. See [REFERENCE Â§ Method field](REFERENCE.md#method-field) and [implement-task Method validation](../../orchestrators/implement-task/REFERENCE.md#method-validation).

Post or narrate drafts; end with: *Review the tasks - reply **scope approved** when the split is accepted, or request edits.*

Add label **`wf:needs-review`** to each draft task issue.

**Default:** do not run **`scope approved`** writes without the explicit phrase.

### 4. On `scope approved`

When the user says **`scope approved`** (optionally naming task issues):

1. **Map Implementing** - one row per draft task (Ticket, Bundle link, Mode, Status `draft`, Blocked by)
2. **Map Decision coverage** - each bundle-scoped GM in bundle **Decisions** â†’ **`assigned`**, **Linked issue** â†’ task URL (when multiple tasks cover one GM, link the primary task or the task that completes that GM)
3. **Comment** on each task summarizing executed updates

Use `gh issue edit --body-file` for full map body replacements. Validate with [validate-map-body](../../utilities/scripts/validate-map-body.ps1) before upload - see [wayfinder REFERENCE Â§ Map body edits](../../REFERENCE.md#map-and-issue-body-edits-reconcile).

**Do not** add `wf:approved` or set Status `ready` yet. Keep **`wf:needs-review`** until **`tasks approved`**.

### 5. On `tasks approved`

When the user says **`tasks approved`**, **`task approved`**, or issue comment **`approved`** (per task or all):

1. **Task issue(s)** - set **Status:** `ready` in body for all approved tasks; remove label **`wf:needs-review`**
2. **`wf:approved`** - add **only when unblocked**; when multiple tasks are ready and unblocked, add to **one** eligible task per approval decision ([REFERENCE Â§ Deferred approval](REFERENCE.md#deferred-wayfinderapproved-wf-eco-gm-026) - use pick prompt)
3. **AFK pickup comment** - when step 2 adds **`wf:approved`** to a **`wf:afk`** task, post pickup comment with trigger phrase **`Approved - AFK implement`** per [implement-task afk-pickup-comment.md](../../orchestrators/implement-task/references/afk-pickup-comment.md) (HITL tasks: label only)
4. **Map Implementing** - update Status column to `ready` for approved tasks
5. **Comment** on each task - ready for implementation; note deferred label if blockers remain

**Default:** do not start implementation without **`wf:approved`** on the task. AFK pickup requires **## Method** populated and the pickup comment posted (v1 comment trigger; **`wf:approved`** is reviewer signal + implement-task gate).

### 6. Hand off

Tell the user:

- **Next:** implement from the ready task(s) with **`wf:approved`** - [implement-task](../../orchestrators/implement-task/SKILL.md) on bundle **Branch:** from [define-bundle](../define-bundle/REFERENCE.md#bundle-branch-wf-eco-gm-027)
- Deferred tasks: label + AFK pickup comment added when blockers clear (implement-task unblock scan) or on a later **`tasks approved`** pass with the one-eligible-task pick
- On completion: invoke wayfinder **Reconcile** with **`Approved - reconcile and close`** on the task issue

## Implementation Reconcile (after ship)

Owned by [wayfinder](../../SKILL.md) **Reconcile**, not create-tasks. On **`Approved - reconcile and close`** for an implementation task:

1. Close task issue; remove **`wf:approved`** and **`wf:needs-review`** labels
2. **Map Implementing** - move row gist to **Completed**
3. **Decision coverage** - bundle-scoped GMs fully shipped by this task â†’ **`implemented`**, linked issue stays task URL

See [REFERENCE.md](REFERENCE.md#implementation-reconcile).

## Interaction rules

1. **Approved bundle only** - never split draft bundles
2. **Planning To Do stays separate** - **Implementing** is the implementation frontier; do not move planning tickets
3. **Globals inherited** - copy bundle **Constraints** into each task **Decisions**; never mark constraint-only GMs **`assigned`**
4. **Draft early** - create task issues while split is still being refined; update in place; always include **## Method** at draft
5. **No bundle edits** - create-tasks does not change bundle Status or re-scope GM rows

## Quick start

User: "Split bundle #N into tasks on map #M."

Load bundle #N + map #M â†’ propose split â†’ create draft `wf:task` issue(s) â†’ user says **`scope approved`** â†’ sync Implementing + coverage â†’ user says **`tasks approved`** â†’ add `wf:approved` â†’ implement.

See [REFERENCE.md](REFERENCE.md) for task template, approval phrases, and coverage updates.
