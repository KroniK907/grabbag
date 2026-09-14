---
name: write-code
description: write-code, wf:task, Method write-code, implement-task dispatch write-code, bundle task implementation, implement approved task, build task deliverables
agent-config-sync: true
---

# Write code

Build the task **What to build** on the bundle branch using **vertical-slice TDD** at pre-agreed seams, plus regular typecheck and test runs. Dispatched by [implement-task](../../orchestrators/implement-task/SKILL.md) after startup gates.

Detail: [REFERENCE.md](REFERENCE.md). TDD reference: [tests.md](tests.md) - [mocking.md](mocking.md).

Adapted from [mattpocock/skills - engineering/implement](https://github.com/mattpocock/skills/tree/main/skills/engineering/implement) and [engineering/tdd](https://github.com/mattpocock/skills/tree/main/skills/engineering/tdd).

## When to use

- **`wf:task`** implementation task with **## Method:** `write-code` (default for normal bundle build work)
- Task **What to build** ships repo code, tests, or docs in the target project

## Not this skill

| Skill | When instead |
|-------|----------------|
| [implement-task](../../orchestrators/implement-task/SKILL.md) | Gates, bundle branch, code-review, commit, push, resolution comment |
| [code-review](../code-review/SKILL.md) | Standards + Spec review after Method (always implement-task) |
| [prototype](../prototype/SKILL.md) | Throwaway demos when **## Method:** `prototype` |
| [design-modules](../design-modules/SKILL.md) | Modules interface shaping before tasks exist |

## Rules

1. **Seams first** - agree test seams with the human (or task spec) before writing tests; no tests at unconfirmed seams.
2. **Vertical slices** - one failing test â†’ minimal pass â†’ repeat; no horizontal "all tests then all code."
3. **Verify often** - typecheck regularly; run the focused test file after each slice; run the full suite once before returning to implement-task.
4. **Follow Decisions** - task **Decisions** and bundle **Constraints** are binding; scope to **Done when**.
5. **Stop at build** - return artifacts to implement-task. It handles code-review, commit, push, and resolution.
