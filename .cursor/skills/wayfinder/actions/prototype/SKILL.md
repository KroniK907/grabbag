---
name: prototype
description: prototype, wf:prototype, Method prototype, implement-task dispatch prototype, throwaway prototype, logic demo, UI variant exploration, prototype task
agent-config-sync: true
---

# Prototype

A prototype is **throwaway code that answers a question**. The question decides the shape.

Dispatched by [implement-task](../../orchestrators/implement-task/SKILL.md) on bundle branch after startup gates. Contract: [REFERENCE.md](REFERENCE.md). Branch playbooks: [LOGIC.md](LOGIC.md) - [UI.md](UI.md).

## When to use

- **`wf:prototype`** implementation task with **## Method:** `prototype`
- Task **What to build** asks to sanity-check logic/state or explore what a UI should look like

## Not this skill

| Skill | When instead |
|-------|----------------|
| [design-modules](../design-modules/SKILL.md) | Map **To Do** prototype tickets (planning frontier); bundle module shaping before create-tasks |
| [implement-task](../../orchestrators/implement-task/SKILL.md) | Gates, git push, resolution comment, **`awaiting-reconcile`** |
| [grill-me](../../ideation/grill-me/SKILL.md) | Binding decisions via Q&A |

## Pick a branch

Identify which question is being answered - from the task **What to build**, surrounding code, or by asking if the user is around:

- **"Does this logic / state model feel right?"** â†’ [LOGIC.md](LOGIC.md). Build a single shareable HTML file - free-play buttons plus tabbed guided walkthroughs - that pushes the state machine through cases hard to reason about on paper, and that a non-developer can drive.
- **"What should this look like?"** â†’ [UI.md](UI.md). Generate several radically different UI variations on a single route, switchable via a URL search param and a floating bottom bar.

The two branches produce very different artifacts - getting this wrong wastes the whole prototype. If the question is genuinely ambiguous and the user isn't reachable, default to whichever branch better matches the surrounding code (a backend module â†’ logic; a page or component â†’ UI) and state the assumption at the top of the prototype.

## Rules that apply to both

1. **Throwaway from day one, and clearly marked as such.** Locate prototype code close to where it will actually be used (next to the module or page it's prototyping for) so context is obvious - but name it so a casual reader can see it's a prototype, not production. For throwaway UI routes, obey whatever routing convention the project already uses; don't invent a new top-level structure.
2. **Trivial to run.** A UI prototype starts from one command in the project's task runner - `pnpm dev`, `python -m â€¦`, `bun run â€¦`, etc. A logic demo is a single HTML file the user double-clicks. Either way, no thinking required to start it.
3. **No persistence by default.** State lives in memory. Persistence is the thing the prototype is _checking_, not something it should depend on. If the question explicitly involves a database, hit a scratch DB or a local file with a clear "PROTOTYPE - wipe me" name.
4. **Skip the polish.** No tests, no error handling beyond what keeps the prototype runnable, no abstractions. Speed over completeness.
5. **Show state after every action.** After every action (logic) or on every variant switch (UI), print or render the full relevant state so the user can see what changed.
6. **Capture the answer when done.** Record the verdict and the question it settled for [implement-task](../../orchestrators/implement-task/SKILL.md) resolution **Summary** / **Done when**. Prototype files stay on the **bundle branch** as primary source - implement-task commits and pushes; agents never open PRs. Fold validated decisions into real code only when the task **Done when** requires it; otherwise leave the prototype as the artifact.
