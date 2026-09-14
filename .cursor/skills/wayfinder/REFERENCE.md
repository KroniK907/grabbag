# Wayfinder reference

## Map slug and decision-log prefix

**Map title:** `{FeatureName}:Map` - PascalCase feature name, literal `:Map` suffix (GitHub issue title, map heading).

**Local filenames:** Use a dot, not a colon - `{FeatureName}.Map.md` and `{FeatureName}.Decision-Log.md`. Colons are invalid in paths on Windows.

**Map slug:** Derive from the feature name for stable IDs:

1. Take the part before `:Map` (e.g. `CommandPalette`).
2. Split on non-alphanumeric boundaries; take significant tokens.
3. Join with hyphens, uppercase, max ~12 chars: `CMD-PAL`, `WF-ECO`, `SEARCH-PANEL`.

**Decision-log prefix:** `{MAP-SLUG}-GM-` + three-digit sequence: `CMD-PAL-GM-001`.

**Subfeature maps:** Child slug **extends** parent when nested: parent `CMD-PAL`, child search UI → `CMD-PAL-SEARCH-GM-001`. Sibling subfeatures under the same parent share the parent prefix segment but distinct suffix: `CMD-PAL-ICONS`, `CMD-PAL-SEARCH`.

**Rules:**

- One authoritative log per map (GitHub issue labelled `wf:decision-log` or section in local map file).
- `grill-me`, **Reconcile**, and ticket resolutions **append** rows; do not renumber existing rows.
- Wayfinder maps implement via [define-bundle](actions/define-bundle/SKILL.md) + [create-tasks](actions/create-tasks/SKILL.md); map-free work may still use `write-a-prd` consolidation.

### Decision-log row format

```markdown
**CMD-PAL-GM-012** - One paragraph: binding decision, constraints, pointers to appendices or tickets.
```

Optional source link: `(from [Palette IA grilling](#123))`

---

## Map body template

Use for GitHub issue body or `wayfinder/utilities/plans/{FeatureName}.Map.md`.

```markdown
# {FeatureName}:Map

**Phase:** charting | deciding | consolidating | implementing | complete
**Map slug:** `{MAP-SLUG}`
**Decision log:** [#NNN or path](link) - prefix `{MAP-SLUG}-GM-`

## Target outcome

<What done looks like for this map - usually a buildable PRD. One or two lines.>

## Notes

<Domain; sibling skills to consult; AFK automation repo; preferences for this effort.>

## Subfeatures

<!-- Child maps - integration boundary, not implementation detail store -->

- [{ChildName}:Map](link) - one-line scope

## To Do

<!-- Frontier lives here + open GitHub sub-issues. Agent-maintained table; human verifies on close. -->

| Ticket | Type | Mode | Assignee | Blocked by |
|--------|------|------|----------|------------|
| [Title](issue-link) | research / prototype / grilling / task | HITL / AFK | @user or unclaimed | - |

## Implementing

<!-- Minted implementation tasks from approved bundles. Separate from planning To Do frontier. -->

| Ticket | Bundle | Mode | Status | Blocked by |
|--------|--------|------|--------|------------|

## Completed

<!-- Row moves here after approved reconcile closes the ticket. One-line gist each. -->

- [Title](link) - gist of outcome

## Not yet specified

<!-- In-scope fog - not sharp enough to ticket yet -->

## Out of scope

<!-- Consciously excluded from this map's target outcome -->

## Decision coverage

<!-- Last section on the map body. Operational GM lifecycle index; binding prose lives in decision log only. -->

| GM ID | Status | Linked issue |
|-------|--------|--------------|
| {MAP-SLUG}-GM-001 | open | - |
```

**Section order:** **Decision coverage** is always the **last section** on the map body (after **Out of scope**). **Implementing** sits below **To Do**; planning and implementation frontiers stay separate.

**Index rule:** The map **lists** and **gists**; detail lives in ticket threads and the decision log. Do not paste full `GM-xx` paragraphs into the map body or Decision coverage table.

---

## Ticket types

| Label | Mode typical | Resolved by | Produces |
|-------|--------------|-------------|----------|
| `wf:research` | HITL (v1) | [research](actions/research/SKILL.md) | Structured findings comment; non-binding Proposed tracker updates |
| `wf:prototype` | HITL | Stub code, outline, or `design-modules` | Asset link → comment |
| `wf:grilling` | HITL | `grill-me` or `strategic-ideation` when Question is scope/strategy | `{MAP-SLUG}-GM-xx` rows in decision log |
| `wf:task` | HITL or AFK | Agent checklist or human errand | Done-work record → comment |

Every To Do ticket is a **child issue** of the map, labelled `wf:todo`.

### Ticket title conventions

Issue **titles** are the first signal agents and humans see in map **To Do** rows, Route suggestions, and chat citations. Use a **type prefix** so the title names the **process** (for planning tickets) or the **deliverable** (for implementation tickets) - not the other way around.

| Prefix | Type label (typical) | Route to skill(s) | Title names |
|--------|----------------------|-------------------|-------------|
| **Grill:** | `wf:grilling` | [grill-me](ideation/grill-me/SKILL.md) | The decision or contract to stress-test - depth-first Q&A |
| **Ideate:** | `wf:grilling` | [strategic-ideation](ideation/strategic-ideation/SKILL.md) - [feature-ideation](feature-ideation/SKILL.md) (stub → strategic-ideation) | Scope/strategy or feature shape to expand → tension → prune |
| **Constrain:** | `wf:grilling` | [constrain-fog](ideation/constrain-fog/SKILL.md) | A **Not yet specified** fog line or cluster to sharpen |
| **Research:** | `wf:research` | [research](actions/research/SKILL.md) | The investigation - facts, prior art, survey |
| **Prototype:** | `wf:prototype` | [prototype](actions/prototype/SKILL.md) - [design-modules](actions/design-modules/SKILL.md) on planning **To Do** | What to explore - throwaway demo, layout, interface variants |
| **Task:** | `wf:task` | [one-off](orchestrators/one-off/SKILL.md) - [implement-task](orchestrators/implement-task/SKILL.md) - [define-bundle](actions/define-bundle/SKILL.md) | What to ship or bundle - repo deliverable, implementation run, or GM cluster |
| **Organize:** | `wf:task` | [wayfinder](SKILL.md) - [one-off](orchestrators/one-off/SKILL.md) | Tracker/map housekeeping - fog sort, map sync, labels, tables, trivial edits |

**Promoted bundle issues** (output of define-bundle, not frontier entry titles) stay **`Bundle: {short name}`** per [define-bundle REFERENCE](actions/define-bundle/REFERENCE.md). **Implementing** tasks from create-tasks stay **`Task: {short name}`**.

### Route by prefix (when several skills share a prefix)

| Prefix | Pick skill when… |
|--------|------------------|
| **Constrain:** | Map-scoped fog grooming → **constrain-fog** (auto-creates session ticket; artifact on ticket body). |
| **Ideate:** | Scope/strategy expand → tension → prune → **strategic-ideation** (default). Legacy **feature-ideation** invoke resolves to the same skill. |
| **Prototype:** | Planning **To Do** exploration → **design-modules** or inline stub. **`wf:approved`** on **Implementing** → **implement-task** → Method **prototype**. |
| **Task:** | Map **To Do** repo deliverable → **one-off**. **`wf:approved`** on **Implementing** → **implement-task**. User wants to group GM cluster → **define-bundle**. |
| **Organize:** | Chart / Materialize / Reconcile / map table or **Not yet specified** edits → **wayfinder**. Trivial checklist only → **one-off**. |

**Rules:**

1. **Planning titles = process, not artifact** - `Grill:` / `Ideate:` / `Constrain:` / `Research:` / `Prototype:` titles describe the **session**; put binding deliverables in **## Question** or a follow-on **`Task:`** ticket after Reconcile.
2. **Implementation titles = deliverable** - `Task:` titles name what lands in the repo, bundle, or tracker when done.
3. **Prefix matches Type column** - map **To Do** Type must agree with the title prefix; fix mismatches via retitle or retype.
4. **Materialize and Reconcile apply prefixes** - when creating issues from **Ticket candidates**, set the title from the Type column using this table (do not copy the Question verbatim as the title).
5. **Noun after prefix is short** - topic or subsystem name; details live in the issue body.

**Examples:**

| Weak title | Strong title | Why |
|------------|--------------|-----|
| Design constrain-fog skill for map fog resolution | **Grill:** constrain-fog skill design | "Design … skill" reads like implementation; `Grill:` signals Q&A first |
| Specify research ticket workflow | **Grill:** research ticket workflow | "Specify" is ambiguous; grilling resolves the contract |
| Cursor cloud automations for AFK pickup | **Research:** Cursor cloud automations for AFK pickup | Names the investigation |
| Subfeature map worked example in REFERENCE | **Prototype:** subfeature map worked example | Names exploration, not a shipped doc yet |
| implement create-tasks skill | **Task:** implement create-tasks skill | Deliverable prefix |
| Group GM-012-015 into first bundle | **Task:** define-bundle for palette shell | Bundling work; Route → define-bundle |
| Decision coverage backfill | **Organize:** decision coverage backfill | Tracker errand; Route → wayfinder or one-off |
| Clear routing-table fog lines | **Organize:** routing table fog | Tracker sort; Route → wayfinder |

**Agent cue:** When the user cites a map ticket by `#N` or title, read the **prefix** first - it narrows the skill set. When the prefix maps to **one** skill, start there. When it maps to **several**, use **## Question** and map context (To Do vs Implementing, fog vs deliverable vs map sync) per the table above - do not treat body prose as permission to skip the prefix family (e.g. `Grill:` → implement).

### Ticket body template (grilling, prototype, task)

```markdown
## Question

<Single decision or investigation this ticket resolves - one session of work.>

## Map

Parent: [{FeatureName}:Map](#parent-issue-number)
```

### Research ticket template

For `wf:research` tickets - full template in [research REFERENCE](actions/research/REFERENCE.md#research-ticket-template). Required: **Question**, **Done when**, **Map**; optional: **Source hints**, **Perspectives**. Materialize and feature-discovery use this shape.

Script template: [scripts/issue-bodies/research.md](scripts/issue-bodies/research.md).

### Implementation task template

For tasks minted by [create-tasks](actions/create-tasks/SKILL.md) from approved bundles - full template in [create-tasks REFERENCE](actions/create-tasks/REFERENCE.md#task-issue-template). Labels: `wf:task` or `:prototype` + `:hitl` or `:afk`; add **`wf:approved`** when **Status:** `ready`.

---

## Map-discovery artifact

**Default:** [feature-discovery](ideation/feature-discovery/SKILL.md) posts a comment on the **map issue** whose body starts with `## Map discovery`. Not a separate issue - an creation-time artifact tied to the map.

| Field | Value |
|-------|--------|
| Created by | feature-discovery (on completion; optional partial comments while in progress) |
| Read by | wayfinder **Materialize** |
| Template | [feature-discovery REFERENCE - map-discovery artifact](ideation/feature-discovery/REFERENCE.md#map-discovery-artifact) |

**Local fallback:** `wayfinder/utilities/plans/{FeatureName}.Map-Discovery.md` when GitHub is unavailable.

Set **Status:** `ready for materialize` in the comment when discovery is complete.

**Materialize lookup:** `gh issue view <map-num> --comments` - use the latest comment containing `## Map discovery` with **Status:** `ready for materialize`, unless the user points at chat output or a specific comment.

---

## Materialize from map-discovery

Load the artifact from [feature-discovery](ideation/feature-discovery/REFERENCE.md#map-discovery-artifact).

| Artifact section | Map / GitHub action |
|------------------|---------------------|
| **Ticket candidates** - sharp Question | Create child issue + **To Do** row; title per [ticket title conventions](REFERENCE.md#ticket-title-conventions) |
| **Fog** | Append to map **Not yet specified** |
| **Out of scope suggestions** | Confirm with user; then **Out of scope** |
| **Zone matrix** | Stays on map-discovery comment only; do not paste into map body |
| **Notes** | Merge into map **Notes** if durable |

After materialize: reply on the map-discovery comment thread with **Status:** `materialized`; add **Completed** gist on map (*Map discovery materialized - N tickets*).

**Create order:** Tickets → wire blockers → link sub-issues → update map body.

**Label each ticket:** `wf:todo` + `wf:research` | `:prototype` | `:grilling` | `:task` + `wf:hitl` | `:afk`.

---

## Reconcile resolution template

Post as a **comment on the session ticket** (grilling, research, prototype, task). Non-binding until human approval - same spirit as research **Proposed tracker updates**, but Reconcile may also propose binding GM rows pending approval.

**Section order (fixed):**

```markdown
## Resolution - {ticket title}

### Session summary

<What was settled, deferred, or left open - concrete terms, not zone labels alone.>

### Decision log (proposed append)

**{MAP-SLUG}-GM-NNN** - … `[global]` when infrastructure applies map-wide; omit tag when bundle-scoped.
(from [{ticket title}](#ticket-num))

### Decision coverage (proposed)

| GM ID | Status | Linked issue |
|-------|--------|--------------|
| {MAP-SLUG}-GM-NNN | open / global | - |

### Map updates (proposed)

- **Completed gist:** …
- **Not yet specified:** …
- **Out of scope:** …
- **Notes:** …

### New ticket candidates

| Title | Type | Mode | Question | Blocked by |
|-------|------|------|----------|------------|
| **Grill:** … / **Research:** … / etc. | research / prototype / grilling / task | HITL / AFK | One sharp Question | - or issue ref |

Title prefix must match Type - see [ticket title conventions](../REFERENCE.md#ticket-title-conventions).

Omit table when none. Derive **Done when** bullets for `research` rows at materialize time (see [feature-discovery - research ticket shape](ideation/feature-discovery/REFERENCE.md#research-ticket-shape-materialize)).

### Bundle cluster suggestions

<!-- Non-binding - define-bundle owns draft/approved bundle issues. Reconcile suggests only. -->

| Suggested name | Covered GM IDs | Rationale | Excluded |
|----------------|----------------|-----------|----------|
| … | {MAP-SLUG}-GM-012-015 | One vertical slice / subsystem | globals, already bundled, still foggy |

Omit table when no cluster is ready. Rows stay **`open`** until **`bundle approved`**.

### Ticket invalidations

- **Close:** [#N Title](link) - superseded by {MAP-SLUG}-GM-NNN / merged into this session
- **Retitle / retype:** …
- **Move to Out of scope:** …

Omit when none.

### Route hint

<One recommended next step + skill - e.g. frontier ticket, define-bundle on cluster above, create-tasks on approved bundle.>

---

Ready for review - reply **Approved - reconcile and close** (or **Approved - reconcile, keep open**) when accepted. Edit any section in this comment before approving.
```

**Draft-only:** Do not append decision log, edit map body, create issues, or close tickets until an approval phrase.

---

## Reconcile inference

Run after loading map, decision log, **Decision coverage**, **To Do**, and the sibling session output.

### All session types

| Signal in session | Propose in resolution |
|-------------------|----------------------|
| Binding decision with durable prose | Decision log row + **Decision coverage** (`open` or `global`) |
| Explicit deferral (“decide later”, “needs spike”) | **Not yet specified** + often a **New ticket candidate** |
| Supersedes an open To Do **Question** | **Ticket invalidations** |
| Mis-scoped or rejected direction | **Out of scope** or invalidation |
| Durable preference for map **Notes** | **Map updates → Notes** |
| Next obvious frontier unchanged | **Route hint** → existing unblocked To Do |

### Grilling (`grill-me`, `strategic-ideation`)

Primary source for full-session inference.

| Signal | Propose |
|--------|---------|
| Branch marked `complete` with binding answers | GM row(s); split distinct decisions into separate IDs |
| Branch deferred with named follow-up | Fog item + typed ticket candidate (`research` for facts; `grilling` for decisions; `prototype` for layout/interface exploration) |
| **Surfaces & experience** - layout agreed but build order unclear | `prototype` or `grilling` ticket; optional bundle cluster if GM cluster is build-ready |
| Multiple `open` GMs describing one deliverable | **Bundle cluster suggestion** (see [define-bundle cluster heuristics](actions/define-bundle/REFERENCE.md#route-heuristics-for-wayfinder)) |
| Infrastructure / cross-cutting constraint | `[global]` on log row; coverage **`global`** |
| Uncertain scope tag | Default **`[global]` when unsure**; human edits in review |

Do **not** create `wf:bundle` issues in Reconcile - narrate clusters for [define-bundle](actions/define-bundle/SKILL.md).

### Research

| Signal | Propose |
|--------|---------|
| Findings imply binding choices | GM rows only after human treats as decided (often via follow-up grilling) |
| **Proposed tracker updates** in findings comment | Merge into **New ticket candidates** / fog / Notes |
| **Done when** satisfied | **Completed gist**; close on full approval |
| Gaps needing more investigation | New `research` ticket or **keep open** |

### Prototype / task

| Signal | Propose |
|--------|---------|
| Asset delivered; decisions captured | GM rows if binding; **Completed gist** |
| Outcome opens new questions | Typed ticket candidates |
| Checklist incomplete | **keep open** or task invalidation |

### Materialize-on-approval (ticket candidates)

When resolution **New ticket candidates** are approved:

1. Create child issues - labels: `wf:todo` + type + mode (same as [Materialize](REFERENCE.md#materialize-from-map-discovery)).
2. Append **To Do** rows; wire **blocked-by** in a second pass.
3. For `research`, include **Done when** in issue body per [research ticket template](actions/research/REFERENCE.md#research-ticket-template).

Skip rows the human struck from the resolution comment before approving.

### Constrain-fog

Run when loading a **`Constrain:`** ticket whose body contains **`## Fog resolution`** with **Status:** `ready for reconcile`.

| Signal in artifact | Propose in resolution |
|--------------------|----------------------|
| **New ticket candidates** rows | **New ticket candidates** table (materialize on approval - same labels as Materialize) |
| **Cleanup** - `out of scope` | **Map updates → Out of scope** |
| **Cleanup** - `deleted` or **Per-item** - remove | Drop line from **Not yet specified** |
| **Remaining fog** / rewrite / split | **Map updates → Not yet specified** |
| **Ticket invalidations** | **Ticket invalidations** section |
| **Route hint** in artifact | **Route hint** (merge with standard inference) |
| **Session summary** | **Session summary** |
| Binding decision prose | **Grill / Ideate** ticket candidates only - **not** GM rows unless user explicitly requested during constrain-fog |

Do **not** use **Materialize** for constrain-fog output. Do **not** append **`## Map discovery`** to the map issue.

---

## Completed workflow and approval

1. Sibling skill (or Reconcile draft) → **resolution comment** on ticket ([template above](#reconcile-resolution-template)).
2. Human reviews → edits comment if needed → sends an **approval phrase** (below).
3. Agent **Reconcile** executes approved GitHub/map updates (including ticket materialize when candidates were not removed).
4. If resolution invalidates other tickets → update or close those; move mis-scoped items to **Out of scope**.

### Approval phrases

| User says | Agent may |
|-----------|-----------|
| **Approved - reconcile and close** | Close ticket issue; move row To Do → **Completed** with gist; append decision log **body**; add/update **Decision coverage** row(s) (last map section); update fog/Notes/Out of scope; **create To Do issues** from **New ticket candidates**; apply **Ticket invalidations**; remove **`wf:needs-review`** |
| **Approved - reconcile, keep open** | Post comment + decision log body + Decision coverage + map notes + optional ticket creates/invalidations; remove **`wf:needs-review`**; **do not** close or move source ticket to Completed |
| (no approval) | Post draft resolution comment only; **do not** close, edit map, append log, or create issues |

**Not Reconcile:** **`bundle approved`** → [define-bundle](actions/define-bundle/SKILL.md). **`scope approved`** / **`tasks approved`** → [create-tasks](actions/create-tasks/SKILL.md).

Synonyms accepted if unambiguous: “approve and close #N”, “reconcile and close this ticket”.

**Requires:** `gh` authenticated on the target repo for issue close/edit.

### Map and issue body edits (Reconcile)

Reconcile, Materialize, and [create-tasks](actions/create-tasks/SKILL.md) often replace a **full map or decision-log issue body**. Collapsed single-line bodies and mojibake are a common agent failure mode on Windows.

**Required workflow**

1. **Fetch (read-only)** - `gh issue view <num> --json body -q .body` for inventory only.
2. **Draft full body** - Write the complete replacement markdown to a UTF-8 `.md` file with section newlines per [map skeleton](#map-skeleton). Apply only the approved delta (To Do row move, Completed gist, Decision coverage, fog/Notes).
3. **Validate** - Run validator before upload. Stop if it fails:

   ```powershell
   .\wayfinder\utilities\scripts\validate-map-body.ps1 path\to\map-body.md
   ```

   ```bash
   bash wayfinder/utilities/scripts/validate-map-body.sh path/to/map-body.md
   ```

4. **Upload** - `gh issue edit <num> --body-file path/to/map-body.md` only.
5. **Verify** - Re-fetch; confirm line count, section headers, and no mojibake.

**Never**

| Anti-pattern | Why it breaks |
|--------------|---------------|
| PowerShell `$body = gh issue view ...` then `-replace` then write back | Preserves collapsed lines; corrupts UTF-8 on Windows |
| `Out-File -Encoding utf8` without BOM control on edited bodies | Encoding surprises; use UTF-8 no-BOM or agent Write tool |
| Partial string patch on live body | Drops `\n\n` section breaks between headers |
| Unicode em dash, en dash, middle dot in map bodies | Mojibake under round-trip; use ASCII `-` only |

**Validation gates** (enforced by `validate-map-body`):

| Check | Threshold |
|-------|-----------|
| Line count | >= 40 (collapsed broken maps are often ~10 lines) |
| Sections | `## To Do`, `## Completed`, `## Decision coverage` each on its own line |
| Mojibake | No `Ã`, `Γ`, `╬`, box-drawing corruption sequences |
| Punctuation | No em dash, en dash, middle dot - ASCII hyphen only |

Same workflow applies to **decision-log** issue bodies when Reconcile appends GM rows (draft full body file; validate structure if map-shaped; upload via `--body-file`).

### Implementation task Reconcile

When closing a **`wf:approved`** implementation task (from [create-tasks](actions/create-tasks/SKILL.md)):

| User says | Agent may |
|-----------|-----------|
| **Approved - reconcile and close** | Close task; remove **`wf:approved`** and **`wf:needs-review`**; move **Implementing** row → **Completed** gist; set Decision coverage **`implemented`** for bundle-scoped GMs shipped by this task |

Synonyms accepted if unambiguous: "reconcile and close task #N".

See [create-tasks REFERENCE - implementation Reconcile](actions/create-tasks/REFERENCE.md#implementation-reconcile).

---

## Frontier queries

**Frontier** = rows in **To Do** whose linked issues are: **open**, **unblocked** (all blockers closed), **unclaimed** (no assignee) or assigned to current worker per session rules.

Use GitHub’s blocked-by graph for ordering. Open tickets not listed in **To Do** should not exist - the table is the human-facing frontier index.

---

## Routing table

Suggest-only - user starts the recommended skill. Map ticket **Type** → default skill:

| Ticket type | Default skill | Notes |
|-------------|---------------|-------|
| `grilling` (implementation) | `grill-me` | Ticket **Question** is the grill seed |
| `grilling` (scope/strategy) | `strategic-ideation` | When **Question** is bundling, roadmap, or scope shape |
| `research` | `research` | HITL v1; AFK deferred per map Notes |
| `prototype` (To Do) | `design-modules` or inline stub | Planning frontier; per ticket **Question** |
| `prototype` (Implementing) | [implement-task](orchestrators/implement-task/SKILL.md) → Method **`prototype`** | [actions/prototype](actions/prototype/SKILL.md); bundle tasks only |
| `task` (Implementing) | [implement-task](orchestrators/implement-task/SKILL.md) | Default Method **`write-code`** for normal build; **`prototype`** for throwaway demos |
| Approved bundle (post-approval) | [design-modules](actions/design-modules/SKILL.md) | Optional HITL modules shaping (one or more) before [create-tasks](actions/create-tasks/SKILL.md) |
| `task` (To Do) | [one-off](orchestrators/one-off/SKILL.md) | Map-scoped repo deliverables; trivial checklist-only errands stay *Agent checklist or human* |
| GM cluster ready to build | `define-bundle` | While planning To Do or fog may stay open; see [define-bundle REFERENCE](actions/define-bundle/REFERENCE.md#route-heuristics-for-wayfinder) |
| Approved bundle | `create-tasks` | Splits into **Implementing** tasks; bundle **Branch:** already set by [define-bundle](actions/define-bundle/REFERENCE.md#bundle-branch-wf-eco-gm-027) |
| Small scope, no map | `write-a-prd` → `prd-to-issues` | **Not** a map Route handoff |
| New feature, no map | wayfinder **Chart** | Then `feature-discovery` |

After sibling session: remind user to invoke wayfinder **Reconcile** (explicit invoke - see map fog if auto-reconcile is ever desired).

### Route heuristics - constrain-fog

Suggest [constrain-fog](ideation/constrain-fog/SKILL.md) when **all** of:

1. Map **To Do** table is **empty** (no open frontier rows)
2. **Not yet specified** is **non-empty**

**Never** auto-suggest constrain-fog when open **To Do** items exist - user may **explicitly invoke** constrain-fog anytime.

When **To Do** has items, Route the planning frontier per [frontier queries](#frontier-queries) instead.

**Contrast:**

| Skill | When |
|-------|------|
| **feature-discovery** | Post-Chart whole-map breadth-first triage → **`## Map discovery`** → Materialize |
| **constrain-fog** | Existing map; groom **Not yet specified** → **`## Fog resolution`** on **`Constrain:`** ticket → Reconcile |
| **grill-me / strategic-ideation / research** | Existing typed tickets on **To Do** |

---

## Subfeature maps

**When:** A zone or subsystem is large enough for its own discovery + ticket graph but must stay consistent with the parent.

**Steps:**

1. **Chart** child map `{SubFeatureName}:Map` with its own slug and decision log.
2. Add link under parent **Subfeatures** with one-line boundary ("Owns search UX; parent owns shell IA").
3. Add parent **To Do** ticket if needed: *Integration review - align `{Child}-GM-*` with `{Parent}-GM-*`* (grilling, blocked by child frontier empty or milestone).
4. Child **Notes** must link parent map and list parent `GM-xx` rows that constrain it.

Cross-map conflicts → parent grilling ticket, not silent edits to child logs.

---

## Tracker operations

**Default:** GitHub issues on the **target repo** (or `KroniK907/agent-config` for meta/skills work). This is the **canonical tracker** when issues are enabled.

| Artifact | Label |
|----------|--------|
| Map | `wf:map` |
| Decision log | `wf:decision-log` |
| Build bundle | `wf:bundle` - body **Branch:** `afk/bundle-{issue-num}-{slug}` when approved |
| To Do ticket | `wf:todo` + type + mode |
| Implementation task (draft) | `wf:task` or `:prototype` + `:hitl` or `:afk` |
| Approved implementation task | above + **`wf:approved`**; body **Status:** `ready` \| `awaiting-reconcile` |
| AFK run lock | **`wf:afk-running`** on current AFK task |
| Awaiting approval | **`wf:needs-review`** - add when agent posts draft awaiting human gate phrase; remove when phrase received |

Map-discovery artifact = **comment on map issue** (no label).

### `wf:needs-review`

Bright-red queue signal: an agent finished a draft step and a **human approval phrase** is required before tracker writes or close.

| Add label | When | Remove label |
|-----------|------|--------------|
| [define-bundle](actions/define-bundle/SKILL.md) | Draft bundle issue posted | **`bundle approved`** |
| [create-tasks](actions/create-tasks/SKILL.md) | Draft task issue(s) posted | **`tasks approved`** (or **`scope approved`** if no further task promotion pending) |
| [wayfinder](SKILL.md) **Reconcile** | Resolution draft comment posted on session ticket | **`Approved - reconcile and close`** or **`Approved - reconcile, keep open`** |
| [implement-task](orchestrators/implement-task/SKILL.md) | Success end-of-run (**Status:** `awaiting-reconcile`) | wayfinder **Reconcile** on **`Approved - reconcile and close`** |

```powershell
gh issue edit <num> --add-label "wf:needs-review"
gh issue edit <num> --remove-label "wf:needs-review"
```

**Sub-issues:** Link map → decision log and tickets via GitHub sub-issues. **Blocked-by:** Use native issue dependencies for frontier ordering.

**Chart create order:** Decision log → map (with log link) → link sub-issues → hand off to feature-discovery.

**Local fallback:** `wayfinder/utilities/plans/{FeatureName}.Map.md` and `{FeatureName}.Map-Discovery.md` only when GitHub is unavailable, or for export. See [plans/README.md](utilities/plans/README.md). Do not commit local files that duplicate active GitHub issues.

---

## Skills repo layout

In **`KroniK907/agent-config`**, the **wayfinder pack** is rooted at **`skills/wayfinder/`**:

| Path | Role |
|------|------|
| `skills/wayfinder/SKILL.md` | Hub orchestrator - Chart, Materialize, Reconcile, Route |
| `skills/wayfinder/orchestrators/<name>/` | Workflow orchestrators - `one-off`, `implement-task` |
| `skills/wayfinder/ideation/<name>/` | Planning interviews - `feature-discovery`, `grill-me`, `strategic-ideation`, `constrain-fog` |
| `skills/wayfinder/actions/<name>/` | Focused playbooks - build Methods (`write-code`, `prototype`) and direct skills (`research`, `define-bundle`, `create-tasks`, `design-modules`, `code-review`); see [actions/PATTERNS.md](actions/PATTERNS.md) |
| `skills/wayfinder/utilities/` | Bootstrap, scripts, plans, release notes - not agent skills |
| `skills/<one-off>/` | Map-free utilities - `commit`, `writing-for-agents`, PRD tools |

**Method path validation:** default pool is skills at **`skills/wayfinder/**/<name>/SKILL.md`** in the pinned pack. Skills under `skills/` are valid only when **## Method** explicitly names them.

Install examples:

```text
npx skills@latest add KroniK907/agent-config/skills/wayfinder
npx skills@latest add KroniK907/agent-config/skills/wayfinder/actions/research
npx skills@latest add KroniK907/agent-config/skills/wayfinder/ideation/grill-me
```

**AFK app repos:** cross-repo bootstrap checklist - [AFK-BOOTSTRAP.md](utilities/AFK-BOOTSTRAP.md). Pin this repo at a semver tag ([RELEASE.md](utilities/RELEASE.md)); templates under [bootstrap/](utilities/bootstrap/).

---

## Ecosystem integration

Skills that **read** wayfinder maps:

| Skill | Reads | Writes |
|-------|-------|--------|
| `write-a-prd` | Map Completed + decision log | PRD issue |
| `prd-to-issues` | PRD | `agent-queue` issues |

Skills that **write** wayfinder state:

| Skill | Writes |
|-------|--------|
| `feature-discovery` | Map-discovery comment on map issue |
| [constrain-fog](ideation/constrain-fog/SKILL.md) | Auto-created **`Constrain:`** ticket; **`## Fog resolution`** artifact; Reconcile materializes ticket candidates |
| `strategic-ideation` | Scope handoff (chat); Reconcile records on map |
| `grill-me` | Decision log `{MAP-SLUG}-GM-xx`; resolution comment on grilling ticket; Reconcile proposes full-session tracker delta (tickets, bundle clusters, route) |
| `define-bundle` | Draft/approved bundle issue; proposed **Branch:** in draft; git branch create + push on **`bundle approved`**; Decision coverage `scoped`; log `- bundled via [#N]` suffixes |
| `create-tasks` | Implementation task issues; **Implementing** table; coverage `assigned` on scope approval; deferred/serial **`wf:approved`** on **`tasks approved`**; `implemented` on Reconcile close |
| [one-off](orchestrators/one-off/SKILL.md) | HITL To Do implementation without bundle pipeline; draft/materialize ticket; `one-off/*` branch; implement-task tail with gate waivers in one-off REFERENCE |
| [implement-task](orchestrators/implement-task/SKILL.md) | Bundle-branch run; Method dispatch; **code-review** after Method; resolution comment; **Status:** `awaiting-reconcile`; dependent unblock; AFK serial handoff |
| [code-review](actions/code-review/SKILL.md) | Two-axis Standards + Spec review; auto-fix obvious mistakes when invoked by implement-task; ad-hoc branch/PR/WIP review on request |
| [actions/prototype](actions/prototype/SKILL.md) | Bundle **`wf:prototype`** Method - throwaway LOGIC (HTML demo) or UI (`?variant=` + switcher) on bundle branch |
| `wayfinder` | Map To Do / Completed / fog / Subfeatures; ticket create/close on approval |
| [research](actions/research/SKILL.md) | Findings comment on research ticket; non-binding Proposed tracker updates |
| Cloud AFK automation | Issue comment trigger **`Approved - AFK implement`** (v1); label **`wf:approved`** for reviewer + gates; runs [implement-task](orchestrators/implement-task/SKILL.md) on bundle branch; **push + resolution comment** (no agent PRs); human Reconcile closes task - setup via [AFK-BOOTSTRAP.md](utilities/AFK-BOOTSTRAP.md) |

**Route hint:** When the user asks to review a branch, PR, WIP changes, or diff since a ref outside an implement-task run, suggest [`code-review`](actions/code-review/SKILL.md) in ad-hoc mode. Complements built-in `review-bugbot` / `review-security`. During **implement-task**, code-review runs automatically after Method - no separate Route handoff.

**Handoff chain:** Chart → feature-discovery → Materialize → sibling skills → Reconcile → **`define-bundle`** ( **`bundle approved`** → create **Branch:** `afk/bundle-{N}-{slug}` ) → **`create-tasks`** ( **`tasks approved`** → **`wf:approved`** + AFK pickup comment when unblocked ) → **`implement-task`** (checkout bundle branch → Method → **code-review** → push) → Reconcile. Map-free: grill-me → `write-a-prd` → `prd-to-issues`.

---

## Chart handoff message (template)

Post or narrate after skeleton creation:

```markdown
**Wayfinder Chart complete** - [{FeatureName}:Map](link) - decision log [#N](link)

**Next:** Run [feature-discovery](ideation/feature-discovery/SKILL.md) with:
- Map: #N
- Seed: …
- Target outcome: …

When the map-discovery comment has **Status:** `ready for materialize`, invoke **wayfinder Materialize** with the map link.
```
