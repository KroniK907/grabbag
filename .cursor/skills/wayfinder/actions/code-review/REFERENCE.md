# Code review reference

Reference for [SKILL.md](SKILL.md).

---

## implement-task integration

**Owner:** [implement-task](../../orchestrators/implement-task/SKILL.md) invokes this skill after Method build work, before commit/push.

### Orchestrator steps

1. **Before Method dispatch** - record baseline: `git rev-parse HEAD` → `<pre-method-sha>`
2. **After Method completes** - load this skill in **implement-task mode**
3. **Run review** - fixed point `<pre-method-sha>`, spec = task issue (already loaded)
4. **Auto-fix** obvious findings per [auto-fix policy](#auto-fix-policy)
5. **Return artifact** - structured block for resolution comment **Code review** section
6. **Commit** - include Method work + auto-fixes (one or two commits; reference task `#N`)

Skip code-review when the diff `<pre-method-sha>...HEAD` is empty (Method produced no file changes).

### Spec in implement-task mode

Use the task issue directly - no lookup chain:

- **What to build**, **Done when**, **Decisions**, **Outcomes / stories**
- Parent bundle **Decisions** (orchestrator passes or already loaded)

Pass full task + bundle decision text to the Spec sub-agent.

---

## Auto-fix policy

Apply fixes **before** returning the orchestrator artifact. Re-run review mentally on the post-fix diff only for items you fixed - do not spawn sub-agents again.

### Fix (obvious - act without asking)

| Category | Examples |
|----------|----------|
| **Syntax / build** | Missing import, typo breaking parse, wrong file extension, unclosed bracket |
| **Task-required paths** | File named in **Done when** or **What to build** missing or at wrong path when the correct path is unambiguous |
| **Broken references** | Internal markdown/skill links pointing to paths that exist elsewhere in the repo |
| **Documented standard - single fix** | Violation of an explicit repo rule with one clear remediation (e.g. required frontmatter field missing) |
| **Trivial typos** | Obvious spelling in user-facing strings, comments, or skill descriptions when meaning is unambiguous |

Commit auto-fixes on the bundle branch. Note each fix in **Auto-fixes applied**.

### Defer (report in resolution - do not fix)

| Category | Examples |
|----------|----------|
| **Fowler smells** | All baseline smells - judgement calls |
| **Spec gaps** | Missing **Done when** bullets, partial features, scope creep |
| **Ambiguous standards** | Style preferences without explicit repo rule |
| **Design / architecture** | Refactors, API shape, module splits |
| **Behavioral bugs** | Logic errors needing tests or human verification |

When uncertain, **defer** and report under Standards or Spec.

---

## implement-task return artifact

Return this structure to the implement-task orchestrator (also paste into resolution **Code review** section):

```markdown
### Code review

**Fixed point:** `<pre-method-sha>...HEAD` - **Spec:** [#N task title](url)

#### Auto-fixes applied

- `<path>` - <what changed and why>
- …
- _(none)_ - if no auto-fixes

#### Standards (remaining)

<sub-agent report - verbatim or lightly cleaned; exclude items auto-fixed>

#### Spec (remaining)

<sub-agent report - exclude items auto-fixed>

#### Summary

| Axis | Auto-fixed | Remaining |
|------|------------|-----------|
| Standards | K | N |
| Spec | - | M |
```

---

## Spec lookup order

For **ad-hoc** mode only (implement-task uses task issue directly):

1. **Issue refs in commit messages** - `#123`, `Closes #45`. Fetch via `gh issue view <num> --json body,title,url`
2. **User-provided path**
3. **Repo spec files** - `docs/`, `specs/`, `.scratch/`
4. **Ask the user** - skip Spec sub-agent if none

For wayfinder tasks, scan **What to build**, **Done when**, **Decisions**, bundle links.

**Not used:** Matt Pocock `docs/agents/issue-tracker.md`

---

## Standards sources

| Source | Examples |
|--------|----------|
| Project docs | `CONTRIBUTING.md`, `CODING_STANDARDS.md`, `AGENTS.md` |
| Cursor rules | `.cursor/rules/*.mdc` |
| Skill conventions | [writing-for-agents](../../../writing-for-agents/SKILL.md), [wayfinder REFERENCE](../../REFERENCE.md) |
| Language defaults | Only when repo is silent |

**Repo overrides** baseline smells. **Judgement calls** for smells - skip tooling-enforced rules.

---

## Fowler smell baseline

From Fowler (*Refactoring*, ch. 3). Paste in full into Standards sub-agent prompt:

- **Mysterious Name** - name doesn't reveal purpose. → rename
- **Duplicated Code** - same logic shape in multiple hunks. → extract shared shape
- **Feature Envy** - method uses another object's data more than its own. → move method
- **Data Clumps** - same fields travel together. → bundle into one type
- **Primitive Obsession** - primitive stands in for domain concept. → small type
- **Repeated Switches** - same switch/if cascade on same type. → polymorphism or shared map
- **Shotgun Surgery** - one change scattered across many files. → gather into one module
- **Divergent Change** - one module edited for unrelated reasons. → split
- **Speculative Generality** - abstraction for spec-unmentioned needs. → delete, inline
- **Message Chains** - long `a.b().c().d()`. → hide behind one method
- **Middle Man** - mostly delegates. → cut, call target direct
- **Refused Bequest** - subclass ignores inheritance. → composition

---

## Sub-agent prompts

Send both in **one message** as parallel Task calls (`subagent_type: generalPurpose`).

### Standards sub-agent

```text
Review this diff for coding standards compliance.

Repository: <absolute path>
Diff command: git diff <fixed-point>...HEAD
Commits: <git log --oneline>

Standards sources:
- <each path found>

Fowler smell baseline (repo silent; repo docs override):
<paste full baseline>

Brief: Report - per file/hunk - (a) documented standard violations: cite file + rule; (b) baseline smells: name + quote hunk. Hard violations vs judgement calls. Skip tooling-enforced rules. Under 400 words.
```

### Spec sub-agent

```text
Review this diff against the spec.

Repository: <absolute path>
Diff command: git diff <fixed-point>...HEAD
Commits: <git log --oneline>

Spec source: <#N title + URL>
Spec contents:
<paste task What to build, Done when, Decisions, bundle Decisions if any>

Brief: Report: (a) missing/partial requirements; (b) scope creep; (c) wrong implementations. Quote spec line per finding. Under 400 words.
```

---

## Output format

Ad-hoc mode - present to user:

```markdown
## Code review - `<fixed-point>`...HEAD

**Commits:** N - **Spec:** <#N / path / none>

## Standards
...

## Spec
...

**Summary:** Standards: N findings (worst: …) - Spec: M findings (worst: …)
```

Implement-task mode - use [implement-task return artifact](#implement-task-return-artifact).

---

## Why two axes

- Standards pass, Spec fail → wrong thing built correctly
- Spec pass, Standards fail → right thing built against conventions

Keep axes separate so one does not mask the other.

---

## Division of labor

| Owner | Responsibility |
|-------|----------------|
| **implement-task** | Record pre-Method SHA; invoke code-review; commit/push; resolution comment; status `awaiting-reconcile` |
| **code-review** | Parallel review; auto-fix obvious items; return artifact |
| **Method skill** | Build deliverables only - no review |
