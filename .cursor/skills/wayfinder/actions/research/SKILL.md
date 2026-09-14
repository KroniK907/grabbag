---
name: research
description: research, wf:research, research ticket ready, map research ticket, fact-gathering, investigate research ticket, wayfinder Route research
agent-config-sync: true
---

# Research

Load a **`wf:research`** ticket, investigate per [behavior rules](REFERENCE.md#behavior-rules), post a **structured findings comment** on the ticket, and hand off **non-binding Proposed tracker updates**. Does **not** append decision-log rows, post Reconcile approval phrases, or edit the map without human review.

**v1 is human-initiated HITL only** - cloud AFK pickup deferred.

## Not this skill

| Skill | When instead |
|-------|----------------|
| [wayfinder](../../SKILL.md) | Chart, Materialize, Reconcile, Route only |
| [grill-me](../../ideation/grill-me/SKILL.md) | Depth-first Q&A â†’ binding `{MAP-SLUG}-GM-*` rows |
| [strategic-ideation](../../ideation/strategic-ideation/SKILL.md) | Scope/strategy expand â†’ tension â†’ prune |
| [feature-discovery](../../ideation/feature-discovery/SKILL.md) | Breadth-first zone triage before tickets exist |

## Prerequisites

- Open **`wf:research`** ticket (`wf:todo` + `wf:hitl` in v1) with **Question**, **Done when**, and **Map** sections
- `gh` authenticated on the target repo (for posting findings comment)
- Parent map + decision log links available from ticket **Map** section

## Workflow

### 1. Load context

```text
gh issue view <research-num> --json body,title,url,labels
gh issue view <map-num> --json body,title,url
```

From the ticket: **Question**, **Done when** bullets, **Map** link, optional **Source hints** and **Perspectives**.

From the map: slug, decision log link, **Notes**, relevant **Not yet specified** fog.

**Gate:** Ticket must be labelled `wf:research`. If the **Question** is scope/strategy shape rather than fact-gathering, hand off to [strategic-ideation](../../ideation/strategic-ideation/SKILL.md) or [grill-me](../../ideation/grill-me/SKILL.md) per [wayfinder routing](../../REFERENCE.md#routing-table).

### 2. Investigate

Follow [behavior rules](REFERENCE.md#behavior-rules):

- **Primary sources first** - docs, specs, code, official APIs
- **Secondary sources** only when labeled; lower weight in Findings
- Seek **â‰¥1 alternate viewpoint** for Viewpoints/alternatives
- If Coverage fails on a **valid premise**, run **one scope-expansion pass** then stop

Use web search, codebase exploration, and ticket **Source hints** as appropriate. Do not treat "couldn't find sources" as invalid premise.

### 3. Draft findings comment

Build comment per [output template](REFERENCE.md#findings-comment-template):

Summary â†’ Findings â†’ Gaps & follow-ups â†’ Viewpoints/alternatives â†’ Coverage â†’ (optional **Invalid premise**) â†’ Proposed tracker updates.

**Coverage:** Map each **Done when** bullet to `satisfied` / `partial` / `not satisfied` with brief evidence.

**Invalid premise:** Include only for logical/category ticket errors (wrong question type, impossible ask, mis-scoped ticket) - not missing data.

### 4. Post comment

Post as a **new top-level comment** on the research ticket:

```powershell
gh issue comment <research-num> --body-file path\to\findings.md
```

Each session posts a **new** comment; prior runs stay in the thread.

**Default:** Do not post Reconcile approval phrases or edit map/decision log from this skill.

### 5. Hand off

Tell the user:

- Findings are **non-binding** - review **Proposed tracker updates** before any map or log edits
- **Binding decisions:** run [grill-me](../../ideation/grill-me/SKILL.md) on findings, or invoke wayfinder **Reconcile** after explicit human approval
- **Follow-up research:** start a new session on the same ticket (new comment); or close via wayfinder **Reconcile** when **Done when** is satisfied and user approves

End with: *Review the findings - invoke wayfinder **Reconcile** when ready to sync approved outcomes to the map.*

## Interaction rules

1. **Non-binding only** - post findings and proposed tracker updates; leave decision-log rows and Reconcile approval to the human
2. **Comment, not file** - findings live on the ticket thread, not repo Markdown
3. **One scope-expansion pass** - if Coverage still fails after expansion, note gaps and stop
4. **HITL v1** - wait for human direction before follow-up research or Reconcile
5. **Invalid premise is rare** - distinguish from insufficient evidence

## Quick start

User: "Research ticket #N on map #M."

Load ticket #N + map #M â†’ investigate per behavior rules â†’ post structured findings comment â†’ hand off for human review.

See [REFERENCE.md](REFERENCE.md) for ticket template, output sections, behavior rules, and design defaults.
