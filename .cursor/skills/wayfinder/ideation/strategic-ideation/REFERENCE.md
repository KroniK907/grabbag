# Reference: session state, counts, handoff

## Default cadence (configurable)

| Segment | Typical length | Notes |
|---------|------------------|--------|
| Ideation band | **3 rounds** per band | Conversational expansion |
| Tension band | **2 rounds** per band | **Exactly one question** per assistant reply |

Repeat bands until prune triggers.

**One assistant reply = one round.** Address one round per message. Wait for the user's reply before advancing to the next round or phase.

Adjust counts only when the user asks or when the seed is trivially small (shorter bands).

## Session state line (every assistant reply)

Emit one compact line after **Recap**:

`Session state: Phase [IDEATION|TENSION|PRUNE] - Band [n] - Round [current]/[band_total] - Next: [what follows this reply]`

The **Next** field describes what you will do **after** the user's **next** message. Wait for that message before executing the step listed in **Next**.

- `Session state: Phase IDEATION - Band 2 - Round 2/3 - Next: IDEATION round 3/3, then TENSION 1/2`
- `Session state: Phase TENSION - Band 3 - Round 1/2 - Next: TENSION round 2/2`
- `Session state: Phase PRUNE - Band - - Round - - Next: finalize handoff artifact`

**First reply (no prior user exchange):** Recap states the seed or history you are using and that the session starts in **IDEATION** at **Round 1/3**; Session state matches (e.g. `Phase IDEATION - Band 1 - Round 1/3 - Next: IDEATION round 2/3`).

## Handoff artifact (after prune)

Produce this **once** when prune completes (unless user asks for edits). Paste-friendly block.

### Rules for the block intended for **grill-me**

Grill-me works best when the seed describes **what to build**, not lengthy negatives or backlog.

1. **Lead with positive scope:** Feature summary, **In scope**, and a short **Boundaries** section that states **edges in affirmative terms** (what this slice *is*, how far it reaches) - without dumping everything cut during pruning.
2. Put big **out-of-scope**, **deferred**, and "maybe later" lists under **Notes for PRD**, not in the grill-me handoff. They pull grill-me off the agreed slice.
3. **Optional exception - `Explicitly not building`:** Include **only** when 1-3 items would **ordinarily look like obvious inclusions** but are **deliberately rejected**, so grill-me does not assume them. Omit the section entirely when nothing meets that bar - this is **not** the default.
4. **Roadmap / backlog / deferred ideas** belong in **Notes for PRD** (or a separate doc), **not** in the grill-me pack.

```markdown
## Feature summary
[One short paragraph: what this feature is and who it is for]

## In scope
- [Actionable bullets: what grill-me should design and implement.]

## Boundaries
[Short positive description of how far this feature extends - shape of "done" - without enumerating rejected or future work.]

## Explicitly not building (optional - include only when needed)
[Only if something would be a natural mistaken inclusion: 1-3 bullets with a one-line reason each. Otherwise omit this entire section.]

## Tensions surfaced (idea/strategy level)
- [Conflict / tradeoff → current leaning or "open"]

## Ready for grill-me
- [Open implementation / architecture decisions to resolve in grill-me - keep these about the **in-scope** work.]

## Notes for PRD / roadmap (optional - not part of grill-me seed)
[Backlog, deferred exploration, stakeholder context. Do not paste this section into grill-me - use it for humans or PRD follow-up only.]
```

## Tension vs grill-me vs feature-discovery

| | strategic-ideation (tension) | grill-me | feature-discovery |
|--|------------------------------|----------|-------------------|
| Layer | Idea: stakeholders, consistency with adjacent initiatives | Implementation: correctness, integrations, rollout | Edge inventory across five zones |
| Pace | One question per tension reply | One question per reply | One zone per reply (default) |
| Output | Scope handoff | `{MAP-SLUG}-GM-xx` | Map-discovery comment on map issue |
