# Research reference

## Research ticket template

Use when [feature-discovery](../../ideation/feature-discovery/SKILL.md) **Materialize** or wayfinder **Materialize** creates a `wf:research` ticket. Title: **`Research: {investigation topic}`** - see [ticket title conventions](../../REFERENCE.md#ticket-title-conventions).

```markdown
## Question

<Single investigation this ticket resolves - one session of fact-gathering.>

## Done when

- [ ] <Acceptance bullet 1 - verifiable from findings>
- [ ] <Acceptance bullet 2>

## Map

Parent: [{FeatureName}:Map](#parent-issue-number)

## Source hints

<Optional - URLs, repos, docs, code paths to start from.>

## Perspectives

<Optional - viewpoints or stakeholders to seek; alternate framings to cover.>
```

**Required:** `## Question`, `## Done when`, `## Map`.
**Optional:** `## Source hints`, `## Perspectives`.

Labels: `wf:todo` + `wf:research` + `wf:hitl` (v1 default) or `wf:afk` (deferred).

---

## Findings comment template

Post as a **new top-level comment** on the research ticket each session.

```markdown
## Research findings

**Ticket:** [#N](url) - **Map:** [{FeatureName}:Map](map-url)
**Session:** <one-line scope note>

### Summary

<2-4 sentences - direct answer to Question; confidence level.>

### Findings

<Primary evidence first - facts, links, code refs, quotes with sources. Label secondary sources explicitly.>

### Gaps & follow-ups

<What remains unknown; suggested follow-up questions or tickets.>

### Viewpoints/alternatives

<≥1 alternate framing, stakeholder view, or competing approach - even when consensus exists.>

### Coverage

| Done when | Status | Notes |
|-----------|--------|-------|
| <bullet from ticket> | satisfied / partial / not satisfied | <brief evidence> |

### Invalid premise

<!-- Include this section ONLY when the ticket Question is logically wrong, mis-categorized, or impossible - NOT when sources are missing. -->

<Why the ticket premise fails; suggested reticket or handoff skill.>

### Proposed tracker updates

<!-- Non-binding - human must approve before Reconcile or map edits. -->

- **Map fog:** …
- **New ticket candidate:** …
- **Notes for map:** …
- **Decision log (for grilling, not direct append):** …
```

**Section order (fixed):** Summary → Findings → Gaps & follow-ups → Viewpoints/alternatives → Coverage → Proposed tracker updates.

**Invalid premise** sits after Coverage, before Proposed tracker updates, and **only when triggered**.

---

## Behavior rules

### Source hierarchy

| Tier | Examples | Weight |
|------|----------|--------|
| Primary | Official docs, source code, specs, standards, first-party APIs | Highest - lead Findings |
| Secondary | Blog posts, Stack Overflow, third-party tutorials | Lower - label explicitly; use to fill gaps only |

### Viewpoints/alternatives

Always seek **≥1 alternate viewpoint** - competing approach, dissenting opinion, stakeholder with different goals, or "do nothing" baseline. If none found, state that and why.

### Scope expansion

When **Coverage** shows `not satisfied` on a **valid premise** (question is sensible but evidence is thin):

1. Run **one** scope-expansion pass - broaden search, adjacent docs, related code paths
2. Re-assess Coverage
3. Stop - do not loop expansion; record remaining gaps in **Gaps & follow-ups**

### Coverage mapping

Each **Done when** bullet maps to exactly one row:

| Status | Meaning |
|--------|---------|
| `satisfied` | Evidence fully meets the bullet |
| `partial` | Some evidence; material gaps remain |
| `not satisfied` | No meaningful evidence after investigation (+ optional expansion pass) |

### Invalid premise

Use **only** for logical/category ticket errors:

- Question asks for binding decision (→ grill-me / strategic-ideation)
- Question is impossible or contradictory
- Ticket type wrong (should be prototype, grilling, or task)

**Not invalid premise:** couldn't find sources, partial answers, or disagreeing experts.

---

## Approval and handoff

Research sessions are **non-binding fact-gathering**:

| Research may | Research must not |
|--------------|-------------------|
| Post findings comment | Append decision-log rows |
| Propose tracker updates (non-binding) | Post Reconcile approval phrases |
| Suggest new tickets or fog items | Edit map body without human review |
| Recommend grill-me on findings | Treat findings as binding decisions |

**Binding path:**

1. Human reviews findings and **Proposed tracker updates**
2. For decisions → [grill-me](../../ideation/grill-me/SKILL.md) or [strategic-ideation](../../ideation/strategic-ideation/SKILL.md) on findings
3. For tracker sync → invoke wayfinder **Reconcile** with human **Approved - reconcile and close** (or **keep open**). Reconcile merges research **Proposed tracker updates** into its [Reconcile resolution template](../../REFERENCE.md#reconcile-resolution-template).

**Follow-up research:** New session on same ticket posts a **new comment**; prior runs preserved in thread.

---

## Route trigger (for wayfinder)

| Trigger | Suggest `research` |
|---------|-------------------|
| Open unblocked `wf:research` ticket on map frontier | Yes - default |
| User explicitly invokes research / names a research ticket | Yes |
| User asks general question without a ticket | No - answer inline or suggest Materialize first |

Explicit user invoke always valid even when ticket is not frontier.

---

## Design defaults

| Topic | Default |
|-------|---------|
| REFERENCE split | Templates and behavior rules in this file; workflow in SKILL.md |
| Route trigger | Frontier `wf:research` ticket **or** explicit user invoke |
| Issue body template file | `wayfinder/utilities/scripts/issue-bodies/research.md` for Materialize scripts |
| Global re-tag pass | **Deferred** - separate Reconcile pass; not part of research v1 |

Research does **not** close tickets or edit map/decision log - that is wayfinder **Reconcile** after human approval.
