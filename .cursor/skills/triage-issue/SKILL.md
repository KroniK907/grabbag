---
name: triage-issue
description: triage, bug report, file issue with fix plan, investigate a problem, root cause and TDD fix plan, what's broken
agent-config-sync: true
---

# Triage Issue

Investigate a reported problem, find its root cause, and create a GitHub issue with a TDD fix plan. Minimize questions to the user.

## Process

### 1. Capture the problem

Get a brief description of the issue from the user. If they haven't provided one, ask ONE question: "What's the problem you're seeing?"

Start investigating immediately. Do not ask follow-up questions yet.

**Done when:** You have a one-line problem statement to investigate.

### 2. Explore and diagnose

Use the Agent tool with subagent_type=Explore to investigate the codebase. Find:

- **Where** the bug manifests (entry points, UI, API responses)
- **What** code path is involved (trace the flow)
- **Why** it fails (the root cause, not just the symptom)
- **What** related code exists (similar patterns, tests, adjacent modules)

Look at:
- Related source files and their dependencies
- Existing tests (what's tested, what's missing)
- Recent changes to affected files (`git log` on relevant files)
- Error handling in the code path
- Similar patterns elsewhere in the codebase that work correctly

**Done when:** You can name the root cause and the code path that triggers it.

### 3. Identify the fix approach

Based on your investigation, determine:

- The minimal change needed to fix the root cause
- Which modules/interfaces are affected
- What behaviors need to be verified via tests
- Whether this is a regression, missing feature, or design flaw

**Done when:** You have a minimal fix strategy in one paragraph.

### 4. Design TDD fix plan

Create a concrete, ordered list of RED-GREEN cycles. Each cycle is one vertical slice:

- **RED**: Describe a specific test that captures the broken/missing behavior
- **GREEN**: Describe the minimal code change to make that test pass

Rules:
- Tests verify behavior through public interfaces, not implementation details
- One test at a time, vertical slices (NOT all tests first, then all code)
- Each test should survive internal refactors
- Include a final refactor step if needed
- Describe behaviors and contracts, not internal structure. Tests assert on observable outcomes (API responses, UI state, user-visible effects), not internal state. A good suggestion reads like a spec; a bad one reads like a diff.

**Done when:** You have a numbered RED-GREEN list with acceptance criteria tied to observable behavior.

### 5. Create the GitHub issue

Create a GitHub issue using `gh issue create` with the template below. Create it and share the URL.

**Done when:** The issue URL and a one-line root-cause summary are posted.

<issue-template>

## Problem

A clear description of the bug or issue, including:
- What happens (actual behavior)
- What should happen (expected behavior)
- How to reproduce (if applicable)

## Root Cause Analysis

Describe what you found during investigation:
- The code path involved
- Why the current code fails
- Any contributing factors

Do not include specific file paths, line numbers, or implementation details that couple to current code layout. Describe modules, behaviors, and contracts instead. The issue should remain useful even after major refactors.

## TDD Fix Plan

A numbered list of RED-GREEN cycles:

1. **RED**: Write a test that [describes expected behavior]
   **GREEN**: [Minimal change to make it pass]

2. **RED**: Write a test that [describes next behavior]
   **GREEN**: [Minimal change to make it pass]

...

**REFACTOR**: [Any cleanup needed after all tests pass]

## Acceptance Criteria

- [ ] Criterion 1
- [ ] Criterion 2
- [ ] All new tests pass
- [ ] Existing tests still pass

</issue-template>
