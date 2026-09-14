---
name: improve-codebase-architecture
description: deep modules, shallow modules, improve architecture, refactoring opportunities, consolidate tightly-coupled modules, module deepening, architectural friction
agent-config-sync: true
---

# Improve Codebase Architecture

Explore a codebase the way an agent would, note where navigation gets hard, and propose module-deepening refactors as GitHub issue RFCs.

A **deep module** (John Ousterhout, "A Philosophy of Software Design") has a small interface hiding a large implementation. Deep modules are easier to test at the boundary and easier for agents to reason about.

## Process

### 1. Explore the codebase

Use the Agent tool with subagent_type=Explore to navigate the codebase. Note where you hit friction:

- Understanding one concept requires bouncing between many small files
- The interface is nearly as complex as the implementation (shallow module)
- Pure functions were extracted for testability, but bugs hide in how they are wired together
- Tightly-coupled modules create integration risk at their boundaries
- Parts of the codebase are untested or hard to test

Where you slow down is the signal.

**Done when:** You have a list of friction points with file paths.

### 2. Present candidates

Present a numbered list of deepening opportunities. For each candidate, show:

- **Cluster**: Which modules/concepts are involved
- **Why they're coupled**: Shared types, call patterns, co-ownership of a concept
- **Dependency category**: See [REFERENCE.md](REFERENCE.md) for the four categories
- **Test impact**: What existing tests would be replaced by boundary tests

Do not propose interfaces yet. Ask the user: "Which of these would you like to explore?"

**Done when:** The user picks one candidate (or declines).

### 3. User picks a candidate

Wait for the user's choice before continuing.

### 4. Frame the problem space

Before spawning sub-agents, write a user-facing explanation of the problem space for the chosen candidate:

- The constraints any new interface would need to satisfy
- The dependencies it would need to rely on
- A rough illustrative code sketch to make the constraints concrete - this is not a proposal, just a way to ground the constraints

Show this to the user, then proceed to Step 5. The user reads while the sub-agents work in parallel.

**Done when:** The problem-space write-up is posted.

### 5. Design multiple interfaces

Spawn 3+ sub-agents in parallel using the Agent tool. Each must produce a **distinct** interface for the deepened module.

Prompt each sub-agent with a separate technical brief (file paths, coupling details, dependency category, what's being hidden). Give each agent a different design constraint:

- Agent 1: "Minimize the interface - aim for 1-3 entry points max"
- Agent 2: "Maximize flexibility - support many use cases and extension"
- Agent 3: "Optimize for the most common caller - make the default case trivial"
- Agent 4 (if applicable): "Design around the ports & adapters pattern for cross-boundary dependencies"

Each sub-agent outputs:

1. Interface signature (types, methods, params)
2. Usage example showing how callers use it
3. What complexity it hides internally
4. Dependency strategy (how deps are handled - see [REFERENCE.md](REFERENCE.md))
5. Trade-offs

Present designs sequentially, then compare them in prose.

After comparing, give your own recommendation: which design you think is strongest and why. If elements from different designs would combine well, propose a hybrid. Be opinionated - the user wants a strong read, not just a menu.

**Done when:** All designs are presented with a recommendation.

### 6. User picks an interface (or accepts recommendation)

Wait for the user's choice.

### 7. Create GitHub issue

Create a refactor RFC as a GitHub issue using `gh issue create`. Use the template in [REFERENCE.md](REFERENCE.md). Create it and share the URL.

**Done when:** The issue URL is posted.
