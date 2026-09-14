# Write-code action reference

Method playbook for bundle **`wf:task`** build work. Implements [PATTERNS.md](../PATTERNS.md) five mandatory sections.

Combines Matt Pocock [implement](https://github.com/mattpocock/skills/tree/main/skills/engineering/implement) build loop with [tdd](https://github.com/mattpocock/skills/tree/main/skills/engineering/tdd) vertical-slice discipline. **Code review, commit, and push are excluded** - owned by [implement-task](../../orchestrators/implement-task/SKILL.md).

---

## 1. Output artifact

On the **bundle branch** (implement-task commits and pushes):

| Artifact | Shape |
|----------|--------|
| Implementation | Source changes matching task **What to build** and **Done when** |
| Tests | New or updated tests at pre-agreed **seams** (see [tests.md](tests.md)) |
| Verification log | Notes for resolution: seams agreed, slices completed, typecheck + full suite result |

No PR, no resolution comment, no task **Status** edits from this skill.

---

## 2. Prerequisites

Hard gates (implement-task runs first):

- **`wf:approved`** task; bundle branch checked out
- Task **What to build**, **Done when**, **Decisions**, and **## Method:** `write-code`
- AFK: **## Method** required and valid

Before coding:

- Read task **Decisions** / bundle constraints
- Skim target area in codebase for conventions
- When seams are not named in the task, confirm seams with the human before the first test (HITL) or infer from **What to build** + **Decisions** and state assumption in resolution inputs (AFK)

---

## 3. Workflow

1. **Load task** - **What to build**, **Done when**, **Decisions**, constraints.

2. **Agree seams** - list public boundaries under test; confirm with human when ambiguous. See [tests.md](tests.md) and task spec.

3. **Tracer bullet** - one failing test at the first seam → minimal implementation → pass.

4. **Vertical slices** - for each remaining **Done when** behavior:
   - RED: one test describing observable behavior through the seam
   - GREEN: minimal code to pass
   - Run focused test file; typecheck when the project supports it

5. **Anti-patterns** - avoid horizontal slicing, implementation-coupled tests, tautological assertions ([tests.md](tests.md), [mocking.md](mocking.md)).

6. **Final verification** - run full test suite once; fix failures within task scope.

7. **Return to implement-task** - summary of changes, test evidence, **Done when** mapping inputs. Do not commit, push, code-review, or edit task **Status**.

---

## 4. Done when mapping

Map task bullets to build artifacts - resolution comment copies verbatim.

| Typical task bullet | Verification |
|--------------------|----------------|
| Feature behavior implemented | Code exists; behavior matches **What to build** |
| Tests at agreed seams | Test files exercise public interface only |
| Typecheck / lint clean | Project typecheck passes (when applicable) |
| Full test suite green | Documented run at end of Method |
| Bundle **Decisions** honored | Constraints reflected in implementation scope |
| No scope creep | Changes limited to task slice |

Adapt to the task issue's actual **Done when** bullets.

---

## 5. Division of labor

| implement-task | write-code (this skill) |
|----------------|-------------------------|
| Startup gates, bundle branch, AFK serial | Agree seams; vertical-slice TDD build |
| [code-review](../code-review/SKILL.md) after Method | - |
| `git commit` / `push` | Edit files on branch only |
| Resolution comment + **`awaiting-reconcile`** | Supply change summary, test evidence, Done when mapping |
| Unblock scan + AFK handoff | - |
| Never close task; never open PR | Never commit, push, review, or post resolution |

**HITL vs AFK:** Same build shape; seam confirmation may be chat-driven (HITL) or inferred from task text (AFK).

**Default Method:** [create-tasks](../create-tasks/REFERENCE.md#method-field) should propose **`write-code`** for normal **`wf:task`** slices unless **prototype** or a human override applies.
