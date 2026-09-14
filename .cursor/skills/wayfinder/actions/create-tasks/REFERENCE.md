# Create tasks reference

## Task issue template

Use for GitHub issue body. Title: `Task: {short name}`.

```markdown
# Task: {short name}

**Status:** draft | ready | awaiting-reconcile

## Parent bundle

[Bundle: {name}](bundle-url) - map [{FeatureName}:Map](map-url) - decision log [#N](log-url)

## What to build

{Concrete deliverables for this vertical slice; 1-2 paragraphs + bullet list when helpful.}

## Method

**Skill:** {skill-name}

{One line - why this Method for this slice. Propose from `wayfinder/` or `wayfinder/actions/`; repo-root one-offs only when human sets explicitly.}

## Decisions

*(Bundle Decisions verbatim + inherited Constraints this task must honor)*

**{MAP-SLUG}-GM-NNN** - …

## Outcomes / stories covered

N/A - meta/infra

- …

## Done when

- [ ] …

## Blocked by

 - 
```

For product tasks with user stories, replace the `N/A` line with story bullets and use **Done when** as acceptance criteria.

### Status lifecycle

| Status | Set by | Meaning |
|--------|--------|---------|
| `draft` | create-tasks | Task created; split not yet promoted to pickup |
| `ready` | create-tasks on **`tasks approved`** | Eligible for [implement-task](../../orchestrators/implement-task/SKILL.md) pickup |
| `awaiting-reconcile` | **implement-task only** | Work pushed; resolution posted; awaits human Reconcile |

create-tasks sets **`draft`** and **`ready`**. Only **implement-task** sets **`awaiting-reconcile`** - do not use that status when splitting or approving tasks.

### Method field

**Required at draft.** Propose a skill from the ecosystem Method pool when minting each task:

- Default pool: **`wayfinder/**/<name>/SKILL.md`** in the pinned skills pack (`wayfinder/actions/<name>/` for bundle build playbooks)
- **Default for `wf:task`:** propose **`write-code`** unless the slice is throwaway exploration (**`prototype`**) or the human sets another Method
- Repo-root one-offs (`tdd`, `commit`, `writing-for-agents`, …) valid **only** when the human explicitly sets them on **## Method**

**AFK pickup:** **## Method** must name a valid skill before **`wf:approved`**. Post pickup comment with **`Approved - AFK implement`** when adding the label - [implement-task](../../orchestrators/implement-task/SKILL.md) stops on missing or invalid Method for AFK tasks. Label **`wf:approved`** is human reviewer signal + startup gate; v1 automation trigger is the comment phrase ([afk-pickup-comment.md](../../orchestrators/implement-task/references/afk-pickup-comment.md)).

**Validation at pickup:** see [implement-task REFERENCE § Method validation](../../orchestrators/implement-task/REFERENCE.md#method-validation).

---

## Approval phrases

| User says | Agent may |
|-----------|-----------|
| **scope approved** | Add map **Implementing** rows; set Decision coverage **`assigned`** + task links for bundle-scoped GMs; keep **`wf:needs-review`** until **`tasks approved`** |
| **tasks approved** / **task approved** / issue comment **approved** | Set task Status `ready`; remove **`wf:needs-review`**; add **`wf:approved`** when **unblocked** (see deferred approval below); for **`wf:afk`** tasks also post AFK pickup comment **`Approved - AFK implement`** ([afk-pickup-comment.md](../../orchestrators/implement-task/references/afk-pickup-comment.md)); update Implementing Status → `ready` |
| (edits requested) | Update draft task bodies in place; keep Status `draft`; keep **`wf:needs-review`**; no `wf:approved` |
| (no approval) | Narrate or post drafts only; add **`wf:needs-review`** on each draft task; **do not** write Implementing or coverage |

Synonyms accepted if unambiguous: "approve the tasks", "approve task #N", "approved" on a specific task thread.

**Separate from Reconcile:** `scope approved` / `tasks approved` are owned by **create-tasks**. wayfinder **Reconcile** owns implementation task close + coverage **`implemented`**.

### Deferred **`wf:approved`** (WF-ECO-GM-026)

On **`tasks approved`**, set **Status:** `ready` for **all** approved tasks. Add **`wf:approved`** only when **Blocked by** is empty or every listed blocker is **closed** or **`awaiting-reconcile`**.

| Blocker state | Label action |
|---------------|--------------|
| No blockers (or **Blocked by:** ` - `) | Add **`wf:approved`** per one-eligible-task rule below; **AFK:** post pickup comment **`Approved - AFK implement`** |
| One or more blockers still **open** | **Defer** label and pickup comment - task stays `ready` without **`wf:approved`** |
| Blocker closed or **`awaiting-reconcile`** | Eligible for label add (+ AFK pickup comment) - [implement-task](../../orchestrators/implement-task/SKILL.md) may add when unblocking dependents |

When blockers remain, defer the label and pickup comment - [implement-task](../../orchestrators/implement-task/SKILL.md) adds **`wf:approved`** (+ AFK pickup comment) when blockers clear ([REFERENCE § Unblock and handoff](../../orchestrators/implement-task/REFERENCE.md#unblock-and-handoff)).

#### One eligible task per approval decision

When multiple tasks are **`ready`** and unblocked after **`tasks approved`**, add **`wf:approved`** to **one** task only - the next logical slice in build order.

**Pick prompt** (narrate to human or AFK operator after approval):

```text
Eligible tasks (ready, unblocked): [#N title], [#M title], …
Recommended next: #N - {one line: rollout order, dependency, or bundle scope summary rationale}
Add wf:approved to #N only; defer others until #N ships or reaches awaiting-reconcile.
For wf:afk on #N: post AFK pickup comment with Approved - AFK implement (see implement-task references/afk-pickup-comment.md).
```

Heuristics for the pick:

1. **Rollout order** - when bundle body lists an Implementing frontier order, pick the first unblocked item
2. **Blocked-by chain** - downstream tasks stay deferred until upstream clears
3. **Dependency / foundation first** - infra or shared contract before consumers
4. **Single remaining task** - add label to that task

Do **not** add **`wf:approved`** to every unblocked task in one approval pass - serial pickup (especially AFK) expects one frontier task at a time.

---

## Split heuristics

| Signal | Split |
|--------|-------|
| Meta/infra bundle (single skill folder + doc updates) | **One task** - skip lengthy split debate |
| Distinct subsystems or deploy order | **2-3 tasks** - one vertical slice each |
| Task would exceed one focused agent session | Propose smaller slices |
| Hard dependency between slices | Earlier slice first; **Blocked by** on downstream task |

Default cap: **3 tasks per bundle session** unless user asks for more. When unsure, propose fewer larger slices and let the user split further.

**Single-task bundles:** Still create a task issue - implements approval gates and **Implementing** tracking even when split is obvious.

---

## Decision coverage updates

### On `scope approved`

For each GM ID listed in bundle **Decisions** (not **Constraints**):

```markdown
| {MAP-SLUG}-GM-NNN | assigned | [#task](task-url) |
```

Linked issue moves from bundle (`scoped`) to task when work is assigned.

When multiple tasks cover one bundle, all share the same GM row until implementation Reconcile marks **`implemented`**. Link the task that will complete that GM, or the first task in build order.

### On implementation Reconcile

When wayfinder **Reconcile** closes a shipped task (`Approved - reconcile and close`):

```markdown
| {MAP-SLUG}-GM-NNN | implemented | [#task](task-url) |
```

Remove **`wf:approved`** from the closed task. Move **Implementing** row gist to **Completed**.

| Status | Meaning | Linked issue |
|--------|---------|--------------|
| `scoped` | In approved bundle | Bundle issue |
| `assigned` | Implementation task exists | Task issue |
| `implemented` | Shipped | Task issue (closed) |

---

## Map Implementing table

Add on **`scope approved`**:

```markdown
| [Task: {name}](task-url) | [#bundle](bundle-url) | HITL / AFK | draft | - |
```

Update Status to **`ready`** on **`tasks approved`**.

Remove row and add **Completed** gist on implementation Reconcile.

---

## Parent linkage

Default (matches [define-bundle](../define-bundle/REFERENCE.md#bundle-issue-template)):

- **Required:** **Parent bundle** section in task body with markdown links to bundle, map, decision log
- **Optional:** native GitHub sub-issues (bundle → task); not required for workflow

---

## Design defaults

| Topic | Default |
|-------|---------|
| Split heuristics | 1-3 tasks; single task for meta/infra one-session bundles |
| Reconcile ownership | create-tasks documents gates; wayfinder Reconcile owns post-ship close + **`implemented`** |
| Sub-issues vs body links | Body **Parent bundle** link required; sub-issues optional |
| Single-task bundles | Still mint a task issue; skip split debate when obvious |
| Global re-tag pass | **Deferred** - separate Reconcile pass; not part of create-tasks |
| Map-free PRD workflow | **Constraint only** - write-a-prd Route stays in wayfinder REFERENCE; not bundled here |

---

## Implementation Reconcile

After an agent or human ships a **`wf:approved`** task:

1. Post resolution comment on the task issue (what shipped, checklist against **Done when**)
2. Human: **`Approved - reconcile and close`**
3. wayfinder **Reconcile** executes:
 - Close task; remove **`wf:approved`**
 - **Implementing** → **Completed** gist
 - Decision coverage **`implemented`** for GMs fully delivered by this task

create-tasks does **not** run implementation Reconcile - remind the user to invoke wayfinder when ready.

---

## Route heuristics (for wayfinder)

Suggest **create-tasks** when:

- User explicitly asks to split or implement from an approved bundle
- Map has an approved **`wf:bundle`** with Status `approved` and empty or stale **Implementing** frontier
- **`define-bundle`** just completed **`bundle approved`** handoff

Prefer **define-bundle** when GM rows are still **`open`** and need bundling first.

After **`tasks approved`**, suggest picking up a **`wf:approved`** task for implementation.
