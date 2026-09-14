---
name: define-bundle
description: define-bundle, build bundles, bundle approved, GM cluster ready, group decisions into bundle, wayfinder Route bundling, wayfinder Route define-bundle, wf:bundle draft
agent-config-sync: true
---

# Define bundle

Group **`open`** decision-log rows into a **draft bundle issue** (`wf:bundle`), then on human **`bundle approved`** promote it and sync map **Decision coverage** + log suffixes. Maps implement incrementally. Do **not** wait for empty To Do, a PRD, or cleared fog.

Runs on a wayfinder map + its decision log. After approval, hand off to [create-tasks](../create-tasks/SKILL.md) or implement directly from the bundle when splitting is unnecessary.

## Not this skill

| Skill | When instead |
|-------|----------------|
| [wayfinder](../../SKILL.md) | Chart, Materialize, Reconcile, Route only |
| [grill-me](../../ideation/grill-me/SKILL.md) | Resolve unknowns â†’ new binding GM rows |
| [design-modules](../design-modules/SKILL.md) | Shape one or more module interfaces from approved bundle before task split |
| [create-tasks](../create-tasks/SKILL.md) | Split an **approved** bundle into implementation tasks |
| [write-a-prd](../../write-a-prd/SKILL.md) | Small map-free scope only |

## Prerequisites

- Map issue (`wf:map`) with **Decision coverage** table and linked decision log
- `gh` authenticated on the target repo
- Label `wf:bundle` available

## Workflow

### 1. Load context

```text
gh issue view <map-num> --json body,title,url
gh issue view <log-num> --json body,title,url
```

From the map: slug, decision log link, **Decision coverage**, **To Do**, **Implementing**, **Notes**.

From the log body: all `{MAP-SLUG}-GM-*` rows. Parse scope:

- **`[global]`** in row text, or coverage status **`global`** â†’ Constraints only (never claimed)
- **`- bundled via [#N]`** suffix â†’ already scoped to a bundle
- Coverage **`open`** + no global tag â†’ bundle candidates

### 2. Propose cluster

Present to the user:

- **Bundle name** (short, build-oriented)
- **Covered GM IDs** (contiguous cluster, one build slice)
- **Rationale** - why these rows ship together
- **Excluded rows** - globals, already bundled, or still foggy

Wait for confirmation or edits before creating/updating the bundle issue.

**Cluster heuristics:** same subsystem or skill; one vertical slice; rows that share deliverables. Do not bundle rows already **`scoped`**, **`assigned`**, or **`implemented`**.

### 3. Draft bundle issue

Create early with `gh issue create` or update an existing draft in place.

| Field | Value |
|-------|--------|
| Title | `Bundle: {short name}` |
| Label | `wf:bundle` |
| Body | Per [REFERENCE.md](REFERENCE.md#bundle-issue-template) |
| **Status** | `draft` |

**Map section:** link parent map, slug, decision log - body link only (no GraphQL sub-issues).

**Decisions:** copy covered GM rows **verbatim** from the log body.

**Constraints:** copy every **`[global]`** log row verbatim - auto-included, not claimed.

Fill **Scope summary**, **Boundaries**, **Open questions**, **User stories or Outcomes** (`N/A - meta/infra` + bullet outcomes when no user stories).

**Branch (draft):** Propose the full **`Branch:`** line per [REFERENCE Â§ Bundle branch](REFERENCE.md#bundle-branch-wf-eco-gm-027) - pattern `afk/bundle-{issue-num}-{slug}`. Do not create the git branch until **`bundle approved`**.

Post or narrate the draft; end with: *Review the bundle - reply **bundle approved** when scope is accepted (optionally confirm or rename **Branch:**), or request edits.*

Add label **`wf:needs-review`** to the draft bundle issue.

**Default:** do not run **`bundle approved`** writes without the explicit phrase.

### 4. On `bundle approved`

When the user says **`bundle approved`** (optionally naming the bundle issue or confirming/editing **Branch:**):

1. **Bundle issue** - set **Status:** `approved` in body (`gh issue edit`); remove label **`wf:needs-review`**
2. **Git branch** - create and push the confirmed **`Branch:`** name per [REFERENCE Â§ Bundle branch](REFERENCE.md#bundle-branch-wf-eco-gm-027); persist **Branch:** on bundle body
3. **Map Decision coverage** - covered rows â†’ **`scoped`**, **Linked issue** â†’ bundle URL
4. **Decision log body** - append ` - bundled via [#N](url)` to each covered row (immutable paragraph text before suffix)
5. **Map Notes** - one-line approved-bundle link if helpful
6. **Comment** on bundle issue summarizing executed updates (include branch name)

Use `gh issue edit --body-file` for full body replacements. Requires `gh` auth.

**Do not** move rows to **Implementing** - that is [create-tasks](../create-tasks/SKILL.md).

### 5. Hand off

Tell the user:

- **Next (recommended when module shape is unclear):** [design-modules](../design-modules/SKILL.md) on the approved bundle (HITL; posts module-design artifact comment(s)) â†’ then [create-tasks](../create-tasks/SKILL.md)
- **Next (when shape is clear):** [create-tasks](../create-tasks/SKILL.md) with the approved bundle link, **or** implement directly from the bundle when a single session needs no task split
- Bundle **Branch:** is created and pushed - all bundle tasks commit on that branch via [implement-task](../../orchestrators/implement-task/SKILL.md)
- Planning **To Do** may stay open

## Interaction rules

1. **One bundle per build slice** - bundle-scoped GM rows are one-bundle-per-row (suffix on approval)
2. **Globals are inherited** - list in Constraints; never suffix or mark **`scoped`**
3. **Log body is authoritative** - binding prose lives in the log issue body only; comments are not binding
4. **Draft early** - create the bundle issue while scope is still being refined; update in place
5. **No implementation** - this skill bundles decisions; it does not write product code unless the bundle itself is meta/infra (then follow bundle deliverables)

## Quick start

User: "Bundle decision-log rows for a build slice on map #N."

Load map #N + decision log â†’ propose cluster â†’ create draft `wf:bundle` issue (with proposed **Branch:**) â†’ user says **`bundle approved`** â†’ create branch, sync coverage + suffixes â†’ suggest design-modules (when shape open) or create-tasks.

See [REFERENCE.md](REFERENCE.md) for bundle template, approval phrases, and global-row rules.
