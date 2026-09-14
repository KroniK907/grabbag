---
name: design-modules
description: design-modules, module interface design, module design, design an interface, after bundle approved, before create-tasks, wf:prototype interface exploration, wayfinder Route design-modules, deep module shaping
agent-config-sync: true
---

# Design modules

Turn bundled **Decisions** (or a planning ticket **Question**) into **deep module** shapes: small interfaces, clear **seams**, dependency categories, and recommended designs with rationale. A bundle may warrant **one module or several**. Notice natural seams in the decision cluster and propose multiple modules when separation improves depth, locality, or task split. Output is structured comment(s) on the bundle or ticket - not repo edits unless the human explicitly requests spike code.

Vocabulary and deepening: [REFERENCE.md](REFERENCE.md). Parallel exploration: [DESIGN-IT-TWICE.md](DESIGN-IT-TWICE.md). Dependency categories: [DEEPENING.md](DEEPENING.md).

Adapted from [mattpocock/skills - engineering/codebase-design](https://github.com/mattpocock/skills/tree/main/skills/engineering/codebase-design). Replaces **design-an-interface**.

**v1 is human-initiated HITL only** - no AFK path.

## Not this skill

| Skill | When instead |
|-------|----------------|
| [wayfinder](../../SKILL.md) | Chart, Materialize, Reconcile, Route only |
| [define-bundle](../define-bundle/SKILL.md) | Group GM rows into draft/approved bundles |
| [create-tasks](../create-tasks/SKILL.md) | Split approved bundle into implementation tasks |
| [prototype](../actions/prototype/SKILL.md) | Throwaway code on bundle branch when **## Method:** `prototype` |
| [improve-codebase-architecture](../../improve-codebase-architecture/SKILL.md) | Codebase-wide exploration â†’ GitHub RFC issues |
| [grill-me](../../ideation/grill-me/SKILL.md) | Binding decisions via Q&A â†’ GM rows |

## Entry paths

| Entry | Input | Output |
|-------|-------|--------|
| **Bundle** (primary) | Approved `wf:bundle` + **Decisions** / **Constraints** | Comment(s) on bundle issue - one artifact per module â†’ hand off to create-tasks |
| **Planning** | Map **To Do** `wf:prototype` or ad-hoc module question | Comment on ticket or map issue - one artifact per module when multiple apply |

## Module count

| Situation | Action |
|-----------|--------|
| Decisions describe one cohesive capability | **One module** - single artifact |
| Clear subsystem or responsibility seams in **Decisions** | **Multiple modules** - one artifact each; brief overview tying them together |
| Ambiguous | Present **recommended count** with rationale; offer single-module alternative; human confirms before design-it-twice |

Multiple modules is an **option**, not a requirement. Split only when seams improve depth or task boundaries.

## Completion

**Done when:** Module-design artifact comment(s) are posted on the bundle or ticket and you have suggested the next skill ([create-tasks](../create-tasks/SKILL.md) for bundles, or Reconcile for planning tickets).

## Quick start

**Bundle:** User says "design modules for bundle #N" after **`bundle approved`**.

Load bundle + map â†’ discover module seams from **Decisions** â†’ frame one or more modules â†’ [DESIGN-IT-TWICE.md](DESIGN-IT-TWICE.md) per module when shape is open â†’ post artifact comment(s) â†’ suggest [create-tasks](../create-tasks/SKILL.md).

**Planning:** User invokes on a prototype or interface ticket on map **To Do**.

Load ticket â†’ gather requirements â†’ discover modules â†’ design-it-twice as needed â†’ post comment(s) â†’ human Reconcile when ticket complete.

See [REFERENCE.md](REFERENCE.md) for workflow, artifact templates, and bundle vs planning deltas.
