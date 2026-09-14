# Deepening

How to deepen a cluster of shallow modules safely, given dependencies. Assumes the vocabulary in [REFERENCE.md](REFERENCE.md) and [SKILL.md](SKILL.md).

Adapted from [mattpocock/skills - engineering/codebase-design/DEEPENING](https://github.com/mattpocock/skills/blob/main/skills/engineering/codebase-design/DEEPENING.md).

## Dependency categories

When assessing a candidate module for deepening, classify its dependencies. The category determines how the deepened module is tested across its seam.

### 1. In-process

Pure computation, in-memory state, no I/O. Safe to deepen - merge modules and test through the new interface directly. No adapter needed.

### 2. Local-substitutable

Dependencies with local test stand-ins (PGLite for Postgres, in-memory filesystem). Safe to deepen if the stand-in exists. Test with the stand-in in-suite. Seam is internal; no port at the module's external interface.

### 3. Remote but owned (Ports & Adapters)

Your own services across a network boundary. Define a **port** (interface) at the seam. Deep module owns logic; transport is an injected **adapter**. Tests use an in-memory adapter; production uses HTTP/gRPC/queue adapter.

### 4. True external (Mock)

Third-party services you do not control. Module takes the dependency as an injected port; tests provide a mock adapter.

## Seam discipline

- **One adapter means a hypothetical seam. Two adapters means a real one.** Do not introduce a port unless at least two adapters are justified (typically production + test).
- **Internal seams vs external seams.** A deep module may have internal seams private to its implementation. Do not expose internal seams through the interface just because tests use them.
- **Between modules.** When a bundle splits into multiple modules, the seam **between** them is as important as each module's external interface - document caller/callee direction and what crosses the boundary.

## Testing strategy: replace, don't layer

- Old unit tests on shallow modules become waste once tests at the deepened interface exist - delete them.
- Write new tests at each deepened module's interface. The **interface is the test surface**.
- Tests assert observable outcomes through the interface, not internal state.
- If a test must change when implementation changes without behaviour change, it is testing past the interface.
