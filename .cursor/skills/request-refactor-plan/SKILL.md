---
name: request-refactor-plan
description: refactor plan, refactoring RFC, plan a refactor, incremental refactor steps, tiny-commit refactor plan, break refactor into safe steps
agent-config-sync: true
---

# Request refactor plan

Walk the user through a refactor and file a GitHub issue with a tiny-commit plan. Skip a step only when the user already answered it in full.

## Process

### 1. Capture the problem

Ask for a detailed description of the problem and any solution ideas they already have.

**Done when:** You can restate the problem in one paragraph without guessing.

### 2. Verify against the repo

Explore the repo to check their claims and understand the current code.

**Done when:** You have traced the affected code paths and noted existing tests.

### 3. Surface alternatives

Ask whether they considered other options. Present at least two alternatives with trade-offs.

**Done when:** The user picks a direction or confirms the original approach.

### 4. Interview on implementation

Ask targeted questions about interfaces, callers, rollout, and failure modes. One topic at a time until the shape is clear.

**Done when:** Module boundaries, data flow, and non-goals are written down.

### 5. Lock scope

Write what will change and what will not. Get explicit confirmation before drafting the plan.

**Done when:** The user agrees to the in/out list.

### 6. Check test coverage

Look for tests on the affected area. If coverage is thin, ask what tests they want before or during the refactor.

**Done when:** You know which modules get new or updated tests.

### 7. Draft tiny commits

Break the work into the smallest commits that each leave the codebase working. Follow Fowler's rule: each step should be independently verifiable.

**Done when:** You have an ordered commit list with a one-line purpose per commit.

### 8. Create the GitHub issue

Create the issue with `gh issue create`. Use the template below.

**Done when:** The issue URL is posted to the user.

<refactor-plan-template>

## Problem Statement

The problem from the developer's perspective.

## Solution

The chosen approach from the developer's perspective.

## Commits

An ordered implementation plan in plain English. Each commit is as small as possible and leaves the codebase in a working state.

## Decision Document

Implementation decisions made during the interview:

- Modules to build or modify
- Interface changes
- Technical clarifications from the developer
- Architectural decisions
- Schema changes
- API contracts
- Specific interactions

Do not include file paths or code snippets. They go stale quickly.

## Testing Decisions

- What makes a good test here (behavior at public boundaries, not internals)
- Which modules will be tested
- Similar tests already in the codebase

## Out of Scope

What this refactor explicitly does not touch.

## Further Notes (optional)

Anything else worth recording.

</refactor-plan-template>
