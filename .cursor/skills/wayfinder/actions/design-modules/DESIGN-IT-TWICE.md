# Design it twice

When a **module's** interface shape is open, explore alternatives with parallel sub-agents. Based on "Design It Twice" (Ousterhout) - your first idea is unlikely to be the best.

Run **per module** after module count and seams are settled (or confirmed with the human). Uses vocabulary from [REFERENCE.md](REFERENCE.md) - **module**, **interface**, **seam**, **adapter**, **caller payoff**.

Adapted from [mattpocock/skills - engineering/codebase-design/DESIGN-IT-TWICE](https://github.com/mattpocock/skills/blob/main/skills/engineering/codebase-design/DESIGN-IT-TWICE.md).

## Process

### 1. Frame the problem space (this module)

Before spawning sub-agents, write a user-facing explanation for **this module only**:

- Constraints any new interface must satisfy
- Which bundle decisions or ticket scope this module owns
- Dependencies and category ([DEEPENING.md](DEEPENING.md))
- How this module relates to sibling modules (when multi-module)
- Rough illustrative sketch - not a proposal

Show this to the user, then proceed to step 2. The user reads while sub-agents work in parallel.

### 2. Spawn sub-agents

Spawn 3+ sub-agents in parallel via Task tool. Each must produce a **radically different** interface for **this module**.

Prompt each sub-agent with a separate technical brief (relevant **Decisions**, file paths when known, dependency category, what sits behind the seam). Give each agent a different design constraint:

- Agent 1: "Minimize the interface - aim for 1-3 entry points max. Get the most behaviour from each entry point."
- Agent 2: "Maximise flexibility - support many use cases and extension."
- Agent 3: "Optimise for the most common caller - make the default case trivial."
- Agent 4 (if applicable): "Design around ports and adapters for cross-seam dependencies."

Each sub-agent outputs:

1. Interface (types, methods, params - plus invariants, ordering, error modes)
2. Usage example
3. What the implementation hides behind the seam
4. Dependency strategy and adapters ([DEEPENING.md](DEEPENING.md))
5. Trade-offs - where caller payoff is high, where it is thin

### 3. Present and compare

Present designs sequentially. Compare in prose by **depth**, **locality** (where change concentrates), and **seam placement**. See [REFERENCE.md](REFERENCE.md) glossary.

Give a recommendation: strongest design and why. Propose a hybrid when elements from different designs combine well. Be opinionated - the user wants a strong read, not just a menu.

### 4. Capture in artifact

Fold the recommendation into this module's section of the [single-module artifact](REFERENCE.md#single-module-artifact-template) on the bundle or ticket comment.

Repeat steps 1-4 for each module that needs exploration.

## Anti-patterns

- Sub-agents must produce distinct designs - enforce radical difference
- Comparison is the value - do not skip it
- Settle module count and seams before design-it-twice
- Interface shape only unless the human explicitly requests spike code
- Pick the shape that hides the most complexity, not the one that looks easiest to build
