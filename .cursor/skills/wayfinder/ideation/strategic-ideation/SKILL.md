---
name: strategic-ideation
description: strategic ideation, scope unstable, strategy unstable, wf:grilling feature shape, Ideate: ticket, expand tension prune, before write-a-prd, Not yet specified scope, strategic expansion, strategic pruning, grill-me handoff, idea-level tensions, not zone triage
agent-config-sync: true
---

# Strategic ideation (expand â†’ tension â†’ prune)

Take a **simple seed** (or chat history) and expand scope deliberately past the target size through **ideation**, stress it with **idea-level tensions** (like grill-me but not implementation), repeat bands until shape stabilizes, then **prune** to a single bounded description suitable for **[grill-me](../grill-me/SKILL.md)** or PRD prep.

## Not this skill

| Skill | When instead |
|-------|----------------|
| [feature-discovery](../feature-discovery/SKILL.md) | Chart phase: breadth-first zone triage â†’ map-discovery comment |
| [grill-me](../grill-me/SKILL.md) | Depth-first Q&A on one implementation branch; resolves a ticket **Question** |
| [wayfinder](../../SKILL.md) | Map/issues tracker, materialize, reconcile, next-step routing |

## When to use (typical triggers)

- A `wf:grilling` ticket whose **Question** is scope, bundling, or strategy
- Map **Not yet specified** notes scope is unstable before PRD
- Optional pass before `write-a-prd` when idea-level tensions were never resolved in tickets
- User explicitly asks for strategic expand/tension/prune (not zone discovery)

## Roles (fixed)

| Mode | Job |
|------|-----|
| **Ideation** | Expansion: new contexts, use cases, surfaces, future-proofing, adjacent areas that might grow together. Add branches and possibilities - bias toward **too big** before pruning. |
| **Tension** | Idea/strategy stress-test: conflicts with existing features or roadmap, separation vs bundling, **internal consistency** of goals. **One question per assistant reply.** Not API/load-testing - that belongs in grill-me. |
| **Prune** | Consolidate into a **handoff artifact** (template in [REFERENCE.md](REFERENCE.md)): positive **In scope** + **Boundaries**; **no** default â€œout of scope / deferredâ€ dumps for grill-me (see REFERENCE handoff rules). |

## Rhythm: alternating bands

Default loop:

1. **Ideation band:** **3 rounds** (each round = **one full exchange**: user message â†’ assistant reply).
2. **Tension band:** **2 rounds** (each tension reply asks **exactly one** question).

**One assistant reply = one round only.** Address exactly one round per message. When a round ends, stop and wait for the user's next message before advancing the round counter or switching phase/band - including at ideationâ†’tension and tensionâ†’ideation boundaries.

Repeat until entering **prune**. Adjust band lengths only when the user asks or when the seed is tiny - see [REFERENCE.md](REFERENCE.md).

Forked takes (optional): During ideation, occasionally offer **2-3 deliberately different, expanded** interpretations of the seed so expansion stays vivid - merge user picks into the running picture.

## Entering prune mode

Use prune when **either**:

1. **User asks** (e.g. â€œletâ€™s prune,â€ â€œfinalize scope,â€ â€œhand off to grillâ€).
2. **Assistant softly suggests** - when ideation loops repeat without new ground, expansions keep collapsing, or tensions already surfaced dominate - **never** force; invite (â€œWant to prune, or one more ideation band?â€).

If the user prefers more cycles, resume alternating bands.

## Every reply: recap first (all phases)

**Recap is mandatory** on every assistant turn (including the first - see below). It is the **first** substantive block in the message.

1. **Recap:** Short summary of **the last exchange**: what was decided, proposed, rejected, or how you interpreted the userâ€™s last message. Gives the user a chance to correct drift. On the **first** reply with no prior exchange: state the seed or history you are using and that the session is starting in **IDEATION** at **Round 1/3**.
2. **Session state:** One line per [REFERENCE.md](REFERENCE.md) (`Phase`, `Band`, `Round`, `Next`).
3. **Body:** Ideation exploration, tension **Question** (single), or prune synthesis - per current phase.

Rename or merge Recap + Session state only if the client UI makes a single combined header clearer - the **information** must always appear.

## Output shape by phase

### Ideation band

After Recap + Session state:

- Drive **conversational** expansion (not a single-question grill). You may propose several related directions in one reply; stay grounded in the userâ€™s seed and accumulated list of in/out scope as it emerges.
- Track **working** scope notes as they emerge (labels provisional until prune). During **prune**, follow [REFERENCE.md](REFERENCE.md) handoff rules for grill-me: affirmative boundaries; avoid negative backlog lists in the grill pack.

### Tension band

After Recap + Session state:

Mirror grill-me discipline for the **question** only:

**Question:**  
Exactly **one** tension question (conflict, consistency, boundary with existing work). No multi-part or stacked questions.

Optional short **Recommendation:** line - your lean on how to answer *that* question only - same spirit as grill-me.

### Prune

After Recap + Session state:

- Synthesize threads into one coherent picture.
- Produce the handoff per [REFERENCE.md](REFERENCE.md): **In scope**, **Boundaries** (positive edges), optional tiny **Explicitly not building** only when misleading defaults exist; **Tensions**; **Ready for grill-me**. Put backlog/deferred material only under **Notes for PRD**, not in the grill-me seed.
- Offer edits until the user accepts the block.
- If tied to a wayfinder map: tell the user to invoke **wayfinder Reconcile** to record scope decisions on the ticket/decision log when ready.

## Mechanical tracking

Maintain internally:

- **Phase:** `IDEATION` | `TENSION` | `PRUNE`.
- **Band:** increments each time you finish an ideation segment + tension segment cycle (or document your counting if you reset per user request).
- **Round-within-band:** e.g. ideation **2/3**, then tension **1/2**.

Expose this in **Session state** every turn so â€œwhatâ€™s nextâ€ is obvious.

## Interaction rules (non-negotiable)

1. **Round** = **one full exchange** (user message â†’ assistant reply).
2. **One round per reply:** Each assistant message addresses exactly one round in the current phase. You may note in **Session state** what comes next, but wait for the user's message before performing it.
3. **Recap** first every time; never skip because the reply is long or late in session.
4. **Tension:** exactly **one** question per tension reply.
5. **Prune suggestion:** soft invitation only; user controls exit from expansion loops.
6. **Grill-me handoff content:** Give grill-me mostly **what to build** and open decisions on that slice. Put backlog material under **Notes for PRD**, not in the grill seed - see [REFERENCE.md](REFERENCE.md).
7. **Label the mode** each turn when ideation and grilling happen in one chat.

## Quick start

User provides a one-sentence seed (or points to prior chat / wayfinder ticket). Respond with Recap (start rules) + Session state + first ideation expansion. After **3** ideation rounds, switch to **2** tension rounds, then repeat until prune.

See [REFERENCE.md](REFERENCE.md) for the handoff template and session-state examples.
