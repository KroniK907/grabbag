# Design-modules reference

HITL module-design skill. Implements bundle and planning entry paths. Vocabulary adapted from [mattpocock/skills - engineering/codebase-design](https://github.com/mattpocock/skills/tree/main/skills/engineering/codebase-design).

---

## Glossary

Use these terms exactly - consistent language is the point.

**Module** - anything with an interface and an implementation. Scale-agnostic: function, class, package, or tier-spanning slice.

**Interface** - everything a caller must know to use the module: signature, invariants, ordering, error modes, configuration, performance characteristics.

**Implementation** - code inside the module. Distinct from **Adapter** (concrete thing at a seam).

**Depth** - behaviour you can exercise per unit of interface learned. **Deep** = lots of behaviour behind a small interface; **shallow** = interface nearly as complex as implementation.

**Seam** _(Michael Feathers)_ - where behaviour can change without editing in that place; where a module's interface lives. **Seam discovery** - finding boundaries between modules in a bundle's decision cluster.

**Adapter** - concrete thing satisfying an interface at a seam.

**Caller payoff** - capability callers get from depth.

**Locality** - maintainers get change concentrated in one place.

See full definitions and principles in workflow step 2.

---

## Prerequisites

- `gh` authenticated on the target repo
- **Bundle entry:** `wf:bundle`, **Status:** `approved`; parent map + decision log links
- **Planning entry:** open ticket with **Question** and **Map** link, or human-declared module question on a map
- HITL only - never add **`wf:afk`**

**Gate (bundle):** If bundle **Status** is `draft`, hand off to [define-bundle](../define-bundle/SKILL.md) for **`bundle approved`**.

**Gate (planning):** If the question is fact-gathering not interface shape, hand off to [research](../research/SKILL.md).

---

## Workflow

### 1. Load context

**Bundle entry:**

```text
gh issue view <bundle-num> --json body,title,url,labels
gh issue view <map-num> --json body,title,url
```

From bundle: **Decisions**, **Constraints**, scope summary, boundaries, open questions.

**Planning entry:**

```text
gh issue view <ticket-num> --json body,title,url,labels
gh issue view <map-num> --json body,title,url
```

From ticket: **Question**, **Done when**, **Map** link.

### 2. Discover modules and frame

From loaded context:

1. **Seam discovery** - read **Decisions** / **Question** for natural module boundaries:
   - Different caller communities or subsystems
   - Independent variation axes (what changes together vs apart)
   - Places where a single interface would be shallow or overloaded
   - Shared **Constraints** that apply to all modules vs decisions scoped to one slice

2. **Propose module count** - one or more working module names with one-line rationale each:
   - **Single module** when decisions describe one deep capability with one external seam
   - **Multiple modules** when clear seams improve depth, testability, or create-tasks split
   - When ambiguous, state both options and recommend one; pause for human confirmation on bundle entry when count affects task split

3. **Per module**, write a problem-space summary:
   - Problem the module solves; callers
   - Which **Decisions** / GM rows this module owns (when bundle entry)
   - Constraints inherited from bundle **Constraints**
   - Dependencies and category (see [DEEPENING.md](DEEPENING.md))
   - Rough illustrative sketch - not a proposal, just grounding

Apply deep-module principles per module:

- **Deletion test** - if deleting the module makes complexity vanish, it was pass-through; if complexity reappears across callers, it earns its keep
- **Interface is the test surface** - callers and tests cross the same seam
- **One adapter = hypothetical seam; two adapters = real seam**

Optional: short [grill-me](../../ideation/grill-me/SKILL.md) pass when bundle **Open questions** are interface-shaped.

### 3. Explore interfaces (when shape is open)

For **each module** whose interface is not already fixed by **Decisions**, follow [DESIGN-IT-TWICE.md](DESIGN-IT-TWICE.md): parallel sub-agents, present sequentially, compare on depth/locality/seam placement, give a recommendation.

When a module's shape is already specified tightly, skip sub-agents for that module and document the chosen shape with rationale.

Modules can be designed sequentially when later modules depend on earlier seam choices - narrate dependency order.

### 4. Post artifact comment(s)

**One module:** post a single comment using [single-module artifact template](#single-module-artifact-template).

**Multiple modules:** post one comment with:
- Brief [overview](#multi-module-overview) (how modules relate, seam between them)
- One complete artifact block per module (repeat [single-module template](#single-module-artifact-template) under `### Module: {name}` headings)

Alternatively, post **separate top-level comments** on the bundle - one per module - when modules are large or human prefers threaded review. Each comment must still include the full single-module artifact sections.

End with handoff:

- **Bundle:** suggest [create-tasks](../create-tasks/SKILL.md); note whether per-module task split hints should drive the split
- **Planning:** human review; Reconcile when ticket **Done when** satisfied

Do not edit the map, decision log, or bundle **Status**. Do not add **`wf:approved`** or implementation labels.

---

## Multi-module overview

When posting multiple modules in one comment, open with:

```markdown
## Modules design overview

**Entry:** bundle | planning
**Map:** [{FeatureName}:Map](map-url)
**Module count:** N

### Why N modules

{1-2 paragraphs - seams discovered, what each module owns, how they interact}

### Cross-module notes

- **Shared constraints:** …
- **Dependencies between modules:** …
- **Suggested build order:** …
```

Then one [single-module artifact](#single-module-artifact-template) per module below.

---

## Single-module artifact template

Each module gets its **own** complete artifact - whether alone or as part of a multi-module comment:

```markdown
### Module: {module name}

**Covers decisions:** {GM IDs or ticket scope slice - bundle entry only}

#### Problem and callers

{1-2 paragraphs}

#### Recommended interface

{Signature, invariants, error modes - the small surface}

#### Seam and dependency category

- **Seam placement:** …
- **Category:** in-process | local-substitutable | remote-but-owned | true-external (see DEEPENING.md)
- **Adapters:** production vs test (when applicable)

#### Alternatives considered

{Brief - only when design-it-twice ran for this module}

#### Recommendation

{Which shape and why - depth, locality, caller payoff}

#### Task split hints

{Optional bullets for create-tasks - vertical slices within this module, suggested Method per slice}

#### Open gaps

{Anything still foggy for this module - link grill or research if needed}
```

When only one module, use top-level `## Module design: {name}` instead of `### Module:` if clearer - same sections required.

---

## Bundle vs planning deltas

| Topic | Bundle entry | Planning entry |
|-------|--------------|----------------|
| Comment target | Bundle issue | Ticket (or map comment if no ticket) |
| Input authority | Bundle **Decisions** verbatim | Ticket **Question** |
| Module count | Seam discovery from GM cluster | Seam discovery from **Question** scope |
| Next step | create-tasks | Reconcile ticket when done |
| Repo edits | Only on explicit human request | Same |

---

## Interaction rules

1. **Comment-only default** - artifacts are the deliverable; no product code unless asked
2. **One artifact per module** - never merge multiple modules into one interface block
3. **Multiple modules optional** - default to fewest modules that stay deep; split only on clear seams
4. **HITL only** - no AFK pickup path in v1
5. **No tracker writes** - map, coverage, and log unchanged by this skill
6. **Replace design-an-interface** - all former `design-an-interface` routing uses **design-modules**
7. **Portable docs** - no live issue numbers in committed skill markdown

---

## Route trigger (for wayfinder)

Suggest **design-modules** when:

- User finished **`bundle approved`** and asks to shape modules before splitting tasks
- [define-bundle](../define-bundle/SKILL.md) hand off - optional step before create-tasks
- Map **To Do** `wf:prototype` ticket explores interface variants (planning entry)
- User mentions "design modules", "deep module", "design it twice", or module interface shaping

Do **not** suggest for:

- Codebase-wide refactor RFCs → **improve-codebase-architecture**
- Throwaway bundle-branch demos → **prototype** action via implement-task
- Binding GM rows → **grill-me** + Reconcile
