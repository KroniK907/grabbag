# Constrain fog reference

---

## Session state line (every assistant reply)

Emit one compact line after **Recap**:

`Session state: Phase [CLEANUP | CONSTRAIN | FULL_DISCOVERY] - Item [n/total or - ] - Next: [action]`

Examples:

- `Session state: Phase CLEANUP - Item - - Next: confirm numbered fog list`
- `Session state: Phase CONSTRAIN - Item 2/5 - Next: verify item 2 before item 3`
- `Session state: Phase FULL_DISCOVERY - Item 2/5 - Zone Behavior & correctness - Round 2/5 - Next: Boundaries & integration`
- `Session state: Phase CONSTRAIN - Item - - Next: finalize artifact - Status ready for reconcile`

**First reply:** Recap states map link, auto-created **`Constrain:`** ticket, and numbered fog list; Session state starts at CLEANUP.

---

## Fog resolution artifact

**Canonical store:** **`## Fog resolution`** section on the auto-created **`Constrain:`** ticket body (not a map comment). wayfinder **Reconcile** reads this artifact - **Materialize** remains for **`## Map discovery`** only.

Layout:

```markdown
## Fog resolution

**Map:** [{FeatureName}:Map](map-url)
**Status:** in progress | ready for reconcile | reconciled

### Session summary

<What was groomed - cleanup counts, items constrained, deferrals.>

### Cleanup

| # | Fog line (as on map) | Outcome |
|---|----------------------|---------|
| 1 | … | kept / deleted / out of scope |

**Deleted** - dropped with no map trace (typo, duplicate, noise).
**Out of scope** - conscious exclusion; Reconcile moves to map **Out of scope**.

### Per-item outcomes

| Source fog | Outcome | Notes |
|------------|---------|-------|
| … | ticket candidate / rewrite / split / deferred fog | … |

**Terminal fog outcomes:**

| Outcome | Reconcile action |
|---------|------------------|
| **Remove** | Delete line from **Not yet specified** |
| **Rewrite** | Replace line with sharper wording |
| **Split** | Replace one line with multiple fog lines or ticket candidates |
| **Deferred fog** | Keep or refine in **Not yet specified** |

### Remaining fog (rewrites/splits)

- …

### New ticket candidates

| Title | Type | Mode | Question | Source fog | Blocked by |
|-------|------|------|----------|------------|------------|
| **Research:** … / **Prototype:** … / **Grill:** … / **Ideate:** … | research / prototype / grilling / task | HITL | One sharp Question | # or line text | - |

Title prefix must match Type - see [wayfinder ticket title conventions](../../REFERENCE.md#ticket-title-conventions).

**To Do dedup:** Before adding a row, check map **To Do** and recent **Completed** for overlapping **Question**. Narrate: **skip** (duplicate), **merge** (extend existing ticket), or **supersede** (invalidation).

### Ticket invalidations

- **Close / retitle / merge:** [#N Title](link) - reason

Omit when none.

### Route hint

<Recommended next step after Reconcile - e.g. frontier ticket, another constrain-fog pass, define-bundle.>
```

### Status lifecycle

| Status | When |
|--------|------|
| `in progress` | Session active; partial updates after CLEANUP lock and each confirmed item |
| `ready for reconcile` | User confirms session complete; invoke wayfinder **Reconcile** |
| `reconciled` | Set by Reconcile after approval (optional; may rely on closed ticket instead) |

---

## Phases

### CLEANUP

1. Copy **Not yet specified** lines into a **numbered list** in chat and artifact **Cleanup** table (outcome `kept` initially).
2. Multi-round review - user may renumber, merge duplicates, or clarify wording.
3. For each line marked for removal, classify:
 - **Delete** - noise; no **Out of scope** trace
 - **Out of scope** - conscious exclusion from map target outcome
4. **Verify gate:** User confirms the cleaned list before entering **CONSTRAIN**. Update artifact; set rows to `deleted` / `out of scope` / `kept`.

Cleanup-only: if no `kept` lines remain for CONSTRAIN, finalize **Session summary**, set **Status:** `ready for reconcile`, hand off to Reconcile.

### CONSTRAIN

One **mini-discovery** reply per **kept** fog item (in list order unless user reprioritizes).

**Mini-discovery** reuses the [feature-discovery zone matrix](../feature-discovery/REFERENCE.md#map-discovery-artifact) at **triage depth** - not five full turns unless FULL_DISCOVERY triggers:

| Zone column | At triage depth |
|-------------|-----------------|
| Known | What is already decided or obvious from map context |
| Unknown / needs research | What remains fuzzy - one line per cell max |

**Primary outputs** (pick what fits the item):

| Output | When | Ticket prefix |
|--------|------|---------------|
| **Research** | Facts, prior art, constraints | **Research:** |
| **Ideate** | Scope/strategy shape | **Ideate:** |
| **Prototype** | Layout, interface, throwaway demo | **Prototype:** |
| **Grill** | Binding decision needing depth-first Q&A | **Grill:** |

Each candidate row includes **Source fog** traceability (line # or quoted text).

**Binding decisions** from CONSTRAIN → **Grill / Ideate** ticket seeds - **not** GM rows unless user explicitly requests decision-log append.

**Terminal fog** for the item: **remove** (resolved without ticket), **rewrite**, **split**, or **deferred fog** (stays fuzzy).

**Verify gate:** Before the next item, ask user to confirm the current item outcome (ticket candidates, fog terminal action, deferrals). Update artifact **Per-item outcomes** and **New ticket candidates**.

### FULL_DISCOVERY (optional inline)

**Soft triggers** - offer (do not force) when user says phrases like:

- "go deeper on this one"
- "full discovery on item N"
- "walk all zones for …"

When active for one item:

1. Run five zone rounds inline (same zones as [feature-discovery](../feature-discovery/SKILL.md))
2. Fold results into artifact **Per-item outcomes** and **New ticket candidates** - **not** a separate **`## Map discovery`** comment
3. Return to CONSTRAIN queue for remaining items

**Session state** during FULL_DISCOVERY includes zone and round (see examples above).

---

## Delete vs out of scope

| Action | Map effect after Reconcile | Trace |
|--------|---------------------------|-------|
| **Delete** | Line removed from **Not yet specified** | None - intentional drop |
| **Out of scope** | Line removed from **Not yet specified**; gist added to **Out of scope** | Conscious exclusion from target outcome |

Never silently delete a line that represents a rejected product direction - use **out of scope**.

---

## Reconcile handoff

Owned by [wayfinder](../../SKILL.md) **Reconcile**, not constrain-fog.

| Topic | constrain-fog behavior |
|-------|------------------------|
| Ticket materialize | **New ticket candidates** → child issues + **To Do** rows on approval |
| Fog edits | **Remaining fog**, rewrites, splits, removals from artifact |
| **Out of scope** | From CLEANUP **out of scope** rows |
| Decision log | **No GM rows** unless user explicitly requested during session |
| **Materialize** | **Not used** - constrain-fog output is not **`## Map discovery`** |
| Close **`Constrain:`** ticket | Human **`Approved - reconcile and close`** |

Resolution comment structure follows [wayfinder Reconcile template](../../REFERENCE.md#reconcile-resolution-template) - infer sections from **`## Fog resolution`** artifact.

---

## Route hint (for wayfinder)

After Reconcile on a constrain-fog session, typical hints:

- First new **To Do** ticket + skill from [routing table](../REFERENCE.md#routing-table)
- Another constrain-fog pass if **Not yet specified** remains and **To Do** is empty again
- [define-bundle](../../actions/define-bundle/SKILL.md) when **Decision coverage** cluster is build-ready

---

## Design defaults

| Topic | Default |
|-------|---------|
| Mode | HITL only - label `wf:hitl`; never `wf:afk` |
| Session ticket | **`Constrain:`** prefix; `wf:grilling`; not on map **To Do** until Reconcile |
| Map placement | Fog lines stay on map until Reconcile applies artifact |
| Portable docs | Generic placeholders in committed markdown - no live issue numbers |
