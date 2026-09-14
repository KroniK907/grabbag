---
name: constrain-fog
description: constrain fog, wayfinder Route, Not yet specified fog, empty To Do open fog, Constrain: ticket, fog-resolution artifact, groom map fog, ready for reconcile, wf:grilling, wf:hitl, invoke constrain fog on map
agent-config-sync: true
---

# Constrain fog

**HITL-only** orchestrator for grooming **Not yet specified** fog on **existing** maps (post Chart/Materialize). Runs session cleanup, per-item mini-discovery, optional inline full discovery, and a **`## Fog resolution`** artifact on an auto-created **`Constrain:`** ticket. Approved ticket candidates materialize via wayfinder **Reconcile**, not **Materialize**.

Detail: [REFERENCE.md](REFERENCE.md)

## Not this skill

| Skill | When instead |
|-------|----------------|
| [feature-discovery](../feature-discovery/SKILL.md) | Post-Chart whole-map breadth-first triage â†’ **`## Map discovery`** |
| [grill-me](../grill-me/SKILL.md) | Depth-first Q&A on an existing **`Grill:`** ticket **Question** |
| [strategic-ideation](../strategic-ideation/SKILL.md) | Scope/strategy expand â†’ tension â†’ prune on **`Ideate:`** tickets |
| [research](../../actions/research/SKILL.md) | Fact-gathering on existing **`Research:`** tickets |
| [wayfinder](../../SKILL.md) | Chart, Materialize, Reconcile, Route only |

## Prerequisites

- Parent map (`wf:map`) with **Not yet specified** section (may be empty after cleanup)
- `gh` authenticated on the target repo
- Human declares constrain-fog intent or accepts Route suggestion

## Orchestration checklist

Run in order. **Stop at first gate failure** - narrate blocker; do not edit map or create tickets beyond the session **`Constrain:`** issue.

### 1. Load context

```text
gh issue view <map-num> --json body,title,url
```

From the map: slug, decision log link, **To Do**, **Not yet specified**, **Out of scope**, optional **Dev branch:** line.

**Route auto-suggest gate:** wayfinder **Route** may suggest constrain-fog only when **To Do** is **empty** and **Not yet specified** is **non-empty**. User may **explicitly invoke** constrain-fog anytime.

### 2. Auto-create session ticket

At session start, create child issue:

- **Title:** `Constrain: {short fog topic or map slug}` - names the grooming session, not a single line
- **Labels:** `wf:grilling` + `wf:hitl` (no `wf:todo` - session ticket is not map frontier until Reconcile)
- **Body:** **Question** (one line - groom map fog), **Map** parent link, **Status:** `in progress`
- **Do not** append a **To Do** row on the map

Post initial **`## Fog resolution`** skeleton on the **`Constrain:`** ticket per [REFERENCE Â§ Fog resolution artifact](REFERENCE.md#fog-resolution-artifact).

### 3. Session phases

Every assistant reply: **Recap** + **Session state** per [REFERENCE Â§ Session state](REFERENCE.md#session-state-line).

| Phase | Purpose |
|-------|---------|
| **CLEANUP** | Numbered fog list from map; multi-round; **delete** vs **out of scope**; confirm ready before **CONSTRAIN** |
| **CONSTRAIN** | One **mini-discovery** reply per selected item; ticket candidates with **Source fog**; verify before next item |
| **FULL_DISCOVERY** | Optional inline per item - soft triggers only; five zone rounds; folds into **`## Fog resolution`**, not **`## Map discovery`** |

**Cleanup-only** sessions (no CONSTRAIN items selected) are valid - skip to artifact finalize + Reconcile handoff.

See [REFERENCE Â§ Phases](REFERENCE.md#phases) for gates, delete vs out of scope, terminal outcomes, and full-discovery triggers.

### 4. Persist artifact

Update **`## Fog resolution`** on the **`Constrain:`** ticket after:

- CLEANUP lock (deleted / out of scope / remaining list confirmed)
- Each confirmed CONSTRAIN item (and after optional FULL_DISCOVERY for that item)

**Status:** `in progress` during session â†’ `ready for reconcile` when user confirms session complete.

Replace content by editing the ticket body (or post follow-up comment only when body edit is impractical - prefer body).

### 5. Hand off

When artifact **Status:** `ready for reconcile`:

1. Tell the user to invoke [wayfinder](../../SKILL.md) **Reconcile** on the **`Constrain:`** ticket
2. Reconcile materializes approved **New ticket candidates** to map **To Do** - not Materialize
3. Reconcile applies fog rewrites/removals, **Out of scope** additions, and **Ticket invalidations**

**Binding decisions** from CONSTRAIN become **Grill / Ideate** ticket seeds - **not** GM rows unless the user explicitly requests a decision-log row.

## Interaction rules

1. **HITL only** - no AFK path; never add **`wf:afk`** or **`wf:afk-running`**
2. **One fog item per CONSTRAIN turn** - finish verify gate before advancing
3. **No map To Do edits** until Reconcile approval
4. **To Do dedup** - before proposing a ticket candidate, check open **To Do** and recent **Completed** for overlapping **Question**; narrate merge/skip/supersede
5. **Recap + Session state** - mandatory every reply
6. **Human closes the ticket** - requires **`Approved - reconcile and close`**

## Quick start

User: "Constrain fog on map #N."

Load map â†’ auto-create **`Constrain:`** ticket â†’ CLEANUP â†’ CONSTRAIN (mini-discovery per item) â†’ optional FULL_DISCOVERY â†’ **`Status: ready for reconcile`** â†’ Reconcile.

See [REFERENCE.md](REFERENCE.md) for artifact template, mini/full discovery rules, and terminal outcomes.
