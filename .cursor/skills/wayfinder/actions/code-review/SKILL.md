---
name: code-review
description: code review, review since ref, review since a ref, review branch, review PR, review WIP, ad-hoc code review, implement-task code review, after implement-task Method, wayfinder code review
agent-config-sync: true
---

# Code review

Two-axis review of the diff between `HEAD` and a **fixed point**:

- **Standards** - documented coding standards + Fowler smell baseline where the repo is silent
- **Spec** - faithful implementation of the originating issue / spec

Both axes run as **parallel sub-agents** (Task tool, `generalPurpose`). Aggregate under separate `## Standards` and `## Spec` headings - do **not** merge or rerank across axes.

Detail: [REFERENCE.md](REFERENCE.md)

## Not this skill

| Skill | When instead |
|-------|----------------|
| `review-bugbot` | Fixed-prompt bug diff review - no spec lookup |
| `review-security` | Fixed-prompt security diff review |
| [implement-task](../../orchestrators/implement-task/SKILL.md) | Full orchestration - this skill is invoked **by** implement-task after Method |

## Invocation modes

| Mode | Trigger | Fixed point | Spec source |
|------|---------|-------------|-------------|
| **implement-task** | Orchestrator after Method completes | Pre-Method `HEAD` SHA (orchestrator records) | Task issue body |
| **ad-hoc** | User asks to review branch / PR / WIP / since ref | User-supplied ref | [Spec lookup order](REFERENCE.md#spec-lookup-order) |

**implement-task mode:** auto-fix [obvious mistakes](REFERENCE.md#auto-fix-policy); return [orchestrator artifact](REFERENCE.md#implement-task-return-artifact) for the resolution comment. See [implement-task integration](REFERENCE.md#implement-task-integration).

## Ad-hoc process

### 1. Pin the fixed point

Commit SHA, branch, tag, `main`, `HEAD~5`, etc. If unspecified, ask.

```powershell
git diff <fixed-point>...HEAD
git log <fixed-point>..HEAD --oneline
```

Gate: `git rev-parse <fixed-point>` succeeds and diff is non-empty.

### 2. Identify spec + standards

Spec: [lookup order](REFERENCE.md#spec-lookup-order). Standards: repo docs + [Fowler baseline](REFERENCE.md#fowler-smell-baseline).

### 3. Spawn parallel sub-agents

One message, two Task calls (`generalPurpose`). Prompts: [REFERENCE Â§ Sub-agent prompts](REFERENCE.md#sub-agent-prompts).

### 4. Aggregate

[Output format](REFERENCE.md#output-format) - present to user; no auto-fix unless user asks.
