# Prototype action reference

Method playbook for bundle **`wf:prototype`** tasks. Implements [PATTERNS.md](../PATTERNS.md) five mandatory sections.

Playbook shape adapted from [mattpocock/skills - engineering/prototype](https://github.com/mattpocock/skills/tree/main/skills/engineering/prototype): **LOGIC** vs **UI** branch, throwaway code, capture verdict on bundle branch.

---

## 1. Output artifact

Branch-dependent deliverables on the **bundle branch** (implement-task pushes):

| Branch | Artifact | Typical shape |
|--------|----------|---------------|
| **Logic** ([LOGIC.md](LOGIC.md)) | Shareable HTML demo | Single self-contained `.html` next to target module; pure liftable logic in `<script>`; free-play + tabbed walkthroughs |
| **UI** ([UI.md](UI.md)) | Multi-variant route | 3-5 structurally different variants on one route via `?variant=`; floating bottom switcher; sub-shape A (existing page) preferred |

**Always capture for resolution:**

- Stated **question** the prototype answers (one paragraph)
- **Verdict** - what was learned; preferred variant or validated model (or open gaps)
- **Paths** - files, route URL, run command (`double-click file` or `pnpm dev …`)
- **Liftable bits** - pure logic module or winning variant name for follow-on tasks

No production merge, PR, or tests unless task **Done when** explicitly requires folding a validated piece into real code.

---

## 2. Prerequisites

Hard gates (implement-task runs first):

- **`wf:approved`** task; bundle branch checked out
- Task **What to build** states or implies a design question (logic/state vs UI look)
- **Done when** lists verifiable artifacts (file paths, route, variant count, verdict)
- AFK: **## Method:** `prototype` valid

Before picking a branch:

- Read task **Decisions** / bundle constraints
- Skim target module or page in codebase when path exists
- If question is ambiguous and user unreachable: pick branch per [SKILL.md](SKILL.md#pick-a-branch) and state assumption in prototype intro

---

## 3. Workflow

1. **Load task** - **What to build**, **Done when**, **Decisions**, constraints.

2. **Pick branch** - [SKILL.md § Pick a branch](SKILL.md#pick-a-branch):
 - Logic / state / API shape → [LOGIC.md](LOGIC.md)
 - Layout / page look → [UI.md](UI.md)

3. **Follow branch playbook** - complete all steps in LOGIC or UI (state question → build → hand over → capture).

4. **Obey shared rules** - [SKILL.md § Rules that apply to both](SKILL.md#rules-that-apply-to-both).

5. **Return to implement-task** - verdict, question, artifact paths, run instructions, **Done when** evidence table inputs. Do not push, post resolution, or edit task **Status**.

---

## 4. Done when mapping

Map task bullets to branch artifacts - resolution comment copies verbatim.

| Typical task bullet | Branch | Verification |
|--------------------|--------|----------------|
| Logic demo answers state question | Logic | HTML file exists; intro states question; walkthroughs + free-play work |
| Pure module liftable from demo | Logic | Logic isolated in `<script>` with no DOM coupling |
| ≥3 UI variants on one route | UI | Variant A/B/C (or more) structurally distinct; `?variant=` switches |
| Floating switcher shareable URL | UI | Bottom bar cycles variants; URL reload-stable |
| Verdict documented | Both | Verdict paragraph ready for resolution **Summary** |
| Runnable without setup (logic) | Logic | Double-click HTML opens demo |
| Runnable via project dev command (UI) | UI | One documented dev command starts app with prototype route |
| Bundle **Decisions** honored | Both | Constraints reflected in prototype scope or verdict notes |

Adapt to the task issue's actual **Done when** bullets.

---

## 5. Division of labor

| implement-task | prototype (this skill) |
|----------------|------------------------|
| Startup gates, bundle branch, AFK serial | Pick LOGIC vs UI branch; build throwaway artifact |
| `git commit` / `push` | Write prototype files on branch |
| Resolution comment + **`awaiting-reconcile`** | Supply verdict, paths, run instructions, Done when evidence |
| Unblock scan + AFK handoff | - |
| Never close task; never open PR | Never close task; never push or post resolution |

**HITL vs AFK:** Same prototype shape; queue semantics are implement-task only.

**vs design-modules:** Map **To Do** interface exploration and optional post-bundle module shaping use [design-modules](../design-modules/SKILL.md). This action is the bundle **Method** when **## Method:** `prototype`.

**vs mattpocock prototype:** Same branch model and playbooks; wayfinder adds bundle-branch capture via implement-task resolution instead of ad-hoc throwaway-branch + issue pointer.
