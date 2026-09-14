---
name: prd-to-issues
description: tracer bullets, PRD to issues, convert PRD to GitHub issues, implementation tickets from PRD, break PRD into work items, vertical slice issues
agent-config-sync: true
---

# PRD to Issues

Break a PRD into independently-grabbable GitHub issues using **vertical slices** (tracer bullets) so work can proceed **in parallel** with minimal merge thrash - **not** broad horizontal layers.

**Assumption:** You usually have **only the PRD text + codebase context**, not the original grill-me chat. All commitments must come from the **parent PRD** (especially the **`GM-xx` decision log**).

## Process

### 1. Locate the PRD

Ask the user for the PRD GitHub issue number (or URL).

If the PRD is not already in your context window, fetch it with `gh issue view <number>` (with comments).

### 2. Explore the codebase (optional)

If you have not already explored the codebase, do so to understand the current state of the code.

### 3. Pre-split human checkpoint (conditional)

**Before** drafting slices, have an **open-ended** conversation with the user when **any** of the following is true:

- The PRD includes a **numbered implementation ladder**, refactor sequence, or other strong ordering signal.
- You are **not confident** you can choose between **strict ladder order** vs **parallelism within steps** without breaking `GM-xx` commitments.

Skip this step for small, obviously parallel PRDs.

Goal: agree on how aggressive parallelism should be **before** you propose a breakdown. If still unclear after a short exchange, state your best assumption and ask for confirmation.

### 4. Draft vertical slices

Break the PRD into **tracer bullet** issues. Each issue is a thin vertical slice that cuts through **all** integration layers end-to-end, **not** a horizontal slice of one layer.

Slices may be `HITL` or `AFK`. `HITL` slices require human interaction (e.g. design review). `AFK` slices can be implemented and merged without human interaction. Prefer `AFK` over `HITL` where possible.

<vertical-slice-rules>
- Each slice delivers a narrow but COMPLETE path through every layer (schema, API, UI, tests)
- A completed slice is demoable or verifiable on its own
- Prefer many thin slices over few thick ones
</vertical-slice-rules>

**`Blocked by` / prerequisites**

- Use **`Blocked by` only** in issue bodies. **No separate â€œserialize withâ€ field.**
- Add **`Blocked by` links only for true prerequisites**: later work would be wrong, non-building, or violate a `GM-xx` / ladder commitment if earlier work is missing.
- **Do not** add blockers â€œto be safeâ€ if **splitting the slice** (different integration seams) could preserve parallelism instead.

If the PRDâ€™s **`GM-xx` log or implementation ladder** implies an order, the **blocker graph should respect that order** unless the user explicitly opted for more parallelism in step 3.

### 5. Quiz the user

Present the proposed breakdown as a numbered list. For each slice, show:

- **Title**: short descriptive name
- **Type**: HITL / AFK
- **Blocked by**: which other slices (if any) must complete first
- **User stories covered**: which user stories from the PRD this addresses
- **`GM-xx` covered**: which decision log rows this slice must satisfy

Ask the user:

- Does the granularity feel right? (too coarse / too fine)
- Are the dependency relationships correct?
- Should any slices be merged or split further?
- Are the correct slices marked as HITL and AFK?

Iterate until the user approves the breakdown.

### 6. Create the GitHub issues

For each approved slice, create a GitHub issue using `gh issue create`. Use the issue body template below.

Label each issue with the `agent-queue` label.

Create issues in dependency order (blockers first) so you can reference real issue numbers in the "Blocked by" field.

**Quoting from the PRD**

- For every **`GM-xx`** that applies to the slice, copy the **`GM-xx` paragraph verbatim** from the parent PRD into the issue (no paraphrase). If the PRD uses appendix pointers, keep those pointer phrases intact.
- For every **user story** the slice addresses, copy the **full story line(s) verbatim** from the parent PRD (the numbered `As aâ€¦` text), not only the numbers.

<issue-template>
## Parent PRD

#<prd-issue-number>

## What to build

A concise description of this vertical slice. End-to-end behavior and boundaries. Reference appendices in the parent PRD by name where helpful.

## Decisions (`GM-xx`, verbatim from parent PRD)

Paste the full text from the parent PRD for each relevant row:

**GM-00x** - â€¦verbatim paragraph from PRDâ€¦

**GM-00y** - â€¦

(Add rows as needed.)

## User stories (verbatim from parent PRD)

Paste the full numbered story lines this slice implements:

1. â€¦
2. â€¦

## Acceptance criteria

- [ ] Criterion 1
- [ ] Criterion 2
- [ ] Criterion 3

## Blocked by

- Blocked by #<issue-number> (if any)

Or "None - can start immediately" if no blockers.

</issue-template>

Do NOT close or modify the parent PRD issue.
