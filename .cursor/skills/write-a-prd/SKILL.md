---
name: write-a-prd
description: write a PRD, product requirements document, after grill-me or decision artifact, GM-xx decision log, plan a new feature (large scope), coordinate large refactor, binding engineering decisions already captured
agent-config-sync: true
---

When **not** to use this skill: if the change is small enough that there are **no binding engineering decisions** (behavior-only tweaks, obvious one-file fixes), skip the PRD and implement directly.

**This skill does not run grill-me.** Depth-first Q&A belongs in a separate session (e.g. the `grill-me` skill) so this workflow does not lose instructions mid-chat. Run that first - or produce an equivalent artifact - then invoke `write-a-prd`.

### Inputs required (gate)

Before doing meaningful work, confirm **at least one** of:

1. **Long discussion in context:** the current conversation already contains a substantial back-and-forth about the feature (decisions, constraints, structure, tradeoffs) - typically from a prior grill-me or design session; **or**
2. **Decision artifact file:** a Markdown or plain-text path the user points you to (plan, notes, RFC fragment, exported chat) that records enough implementation detail and **binding decisions** to build the `GM-xx` log without inventing them.

If **neither** is true, **stop.** Tell the user you need either a pasted/summarized long discussion in the thread or a file path to a detailed artifact, and that running `grill-me` (or similar) first is the right way to produce that raw material.

You may skip later steps if you don't consider them necessary.

1. **Resolve sources:** Identify which messages in context and/or which file(s) are authoritative. Read artifact files from disk when given paths.

2. Explore the repo to verify claims from those sources and understand the current state of the codebase.

3. Sketch out the major modules you will need to build or modify from the sources plus the repo. Actively look for opportunities to extract deep modules that can be tested in isolation.

A deep module (as opposed to a shallow module) is one which encapsulates a lot of functionality in a simple, testable interface which rarely changes.

If the sources are ambiguous on module boundaries, ask **targeted** clarifying questions only where gaps would corrupt the PRD; do **not** restart a full grill-me here.

4. Write the PRD using the template below, consolidating **binding decisions** from the sources into `GM-xx` (see â€œConsolidating decisionsâ€). **Primary consumer:** another agent (often followed by `prd-to-issues`). Optimize for **stable decision IDs**, **explicit pointers**, and **section order** so downstream work does not drift.

5. Submit the PRD as a GitHub issue.

## Consolidating decisions into `GM-xx`

- The **`GM-xx` log is not a transcript** of the source material and need not map 1:1 to turns or bullet points in the artifact. It is a **numbered decision ledger**: one row per **binding** outcome distilled from the discussion or file.
- If a single exchange locks in **several independent decisions**, use **separate `GM-xx` rows** (better traceability into issues).
- Each `GM-xx` entry is **one paragraph** (freeform) that states the decision and any non-negotiable constraints. Include a **one-line context prefix** only if the decision would be cryptic without it.
- **No numeric word cap.** Aim for the **shortest paragraph that preserves every constraint** someone would regret losing if it were omitted.
- **Spawning rows from one batch:** you may start daughter rows with **Same decision batch as `GM-012`** (or repeat a one-line scope phrase) to avoid noise, then give that rowâ€™s decision paragraph.
- **Bulky artifacts** (big tables, directory trees, mermaid): put them in **appendices** with **stable headings** (e.g. `## Appendix A - Target layout`). Each `GM-xx` that depends on that artifact must **name the appendix in prose** (e.g. â€œsee Appendix Aâ€). **Markdown anchor links** are optional - useful for long PRDs humans will navigate; plain pointers are enough for agents.
- **Repo paths and filenames** are allowed wherever they are part of the **agreed outcome** (refactors, ownership, routing). Use **full repo links only sparingly** - when the name alone would be ambiguous (many `handlers.go`, similarly named packages). Prefer searchable paths in prose otherwise.
- If the work genuinely has **no binding engineering decisions**, write a single line under the decision log: **`No binding engineering decisions`** (rare for PRDs created through this skill).

<prd-template>

Sections **must** appear in this order (another agent will read the PRD end-to-end; decisions before the long narrative reduces drift).

## Problem Statement

The problem that the user is facing, from the user's perspective. Keep it tight.

## Solution

The solution to the problem, from the user's perspective. Keep it tight.

## Decision log (`GM-xx`)

Numbered, stable identifiers: **`GM-001`**, **`GM-002`**, â€¦

For each id, **one paragraph** per the rules above. This section is the **authoritative** record of architecture and implementation commitments for downstream issues.

## User Stories

A numbered list of user stories. Each user story should be in the format of:

1. As an <actor>, I want a <feature>, so that <benefit>

<user-story-example>
1. As a mobile bank customer, I want to see balance on my accounts, so that I can make better informed decisions about my spending
</user-story-example>

Cover the feature thoroughly. **Default:** when a story depends on a structural or sequencing commitment, add **`Ref: GM-0xx`** (and additional refs if needed). Omit refs only for ordinary product behavior that does not rest on a logged decision.

## Supporting specifications (optional)

Use for **implementation ladders** (numbered steps), compatibility notes, rollout plans, or other material that is too large for a `GM-xx` paragraph. **Appendices** (tables, diagrams, file trees) live here with stable headings; **`GM-xx` rows point to them by name.**

## Testing decisions

A list of testing decisions that were made. Include:

- A description of what makes a good test (only test external behavior, not implementation details)
- Which modules will be tested
- Prior art for the tests (i.e. similar types of tests in the codebase)

## Out of scope

A description of the things that are out of scope for this PRD.

## Further notes

Any further notes about the feature.

</prd-template>
