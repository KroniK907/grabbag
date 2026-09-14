---
name: feature-discovery
description: feature discovery, wayfinder Chart handoff, zone triage, map-discovery artifact, ready for materialize, breadth-first discovery, known vs unknown, five coverage zones, wayfinder Materialize, feature start, before grilling
agent-config-sync: true
---

# Feature discovery (breadth-first zone triage)

Walk all **five coverage zones** once at **triage depth** - collect what is **known**, **unknown**, or **needs research**. Do **not** depth-first grill; do **not** resolve unknowns. On completion, post a **[map-discovery artifact](REFERENCE.md#map-discovery-artifact)** as a **comment on the map issue** for wayfinder **Materialize**.

Typically runs **after** wayfinder **Chart** creates the map skeleton and decision log.

## Not this skill

| Skill | When instead |
|-------|----------------|
| [constrain-fog](../constrain-fog/SKILL.md) | Groom existing map **Not yet specified** fog - reuses zone matrix at triage depth per item |
| [strategic-ideation](../strategic-ideation/SKILL.md) | Scope/strategy expand â†’ tension â†’ prune; idea-level tradeoffs |
| [grill-me](../grill-me/SKILL.md) | Depth-first Q&A on one branch; resolves a single ticket **Question** |
| [wayfinder](../../SKILL.md) | Create map, materialize To Do tickets, reconcile, routing |

## Prerequisites

Wayfinder **Chart** must have created `{FeatureName}:Map` (with target outcome in the body).

**Local fallback:** If GitHub is unavailable, write the artifact to `wayfinder/utilities/plans/{FeatureName}.Map-Discovery.md` - same layout per [REFERENCE.md](REFERENCE.md#map-discovery-artifact).

## Coverage zones

Same five as [grill-me](../grill-me/SKILL.md#coverage-zones-canonical-branches). Read each zone file through **Quick triage** only:

| Zone | Reference |
|------|-----------|
| Surfaces & experience | [surfaces-and-experience.md](../grill-me/references/surfaces-and-experience.md) |
| Behavior & correctness | [behavior-and-correctness.md](../grill-me/references/behavior-and-correctness.md) |
| Boundaries & integration | [boundaries-and-integration.md](../grill-me/references/boundaries-and-integration.md) |
| Persistence & data | [persistence-and-data.md](../grill-me/references/persistence-and-data.md) |
| Change, risk & evidence | [change-risk-and-evidence.md](../grill-me/references/change-risk-and-evidence.md) |

## Session shape

**Default: one zone per assistant reply** (five zones â†’ five discovery rounds minimum).

1. **Recap** - Last exchange, or on first reply: seed + **map issue** link + starting zone.
2. **Session state** - Per [REFERENCE.md](REFERENCE.md#session-state-line).
3. **Body** - For the **current zone only**: summarize **known**, **unknown**, **needs research** (with the question to answer). Invite corrections; optionally propose blue-sky edges.
4. **Accumulate** - Track ticket candidates, fog, and notes internally (or in chat) as zones progress.
5. **Stop** - Wait for user reply before the next zone.

**Multi-session discovery:** After each zone (or band), you may post a **partial** map-discovery comment on the map issue with **Status:** `in progress` so work survives chat changes. Replace content by posting an updated full comment (see [REFERENCE.md](REFERENCE.md#persist-as-map-comment)).

After all five zones are visited, post the **final** map-discovery comment with **Status:** `ready for materialize`.

**Valid cell entries:** concrete decisions, `unknown`, or `needs research` (with sharp question).

## Every reply: recap first

**Recap is mandatory** on every turn. On the **first** reply: state the feature seed, map issue link, and that discovery starts at **Surfaces & experience - Round 1/5**.

## Completion

When all zones are captured:

1. Emit the artifact in chat per [REFERENCE.md](REFERENCE.md#map-discovery-artifact).
2. Post the same block as a comment on the map issue (`gh issue comment <map-num> --body-file â€¦`).
3. Set **Status:** `ready for materialize` in the comment.
4. Tell the user to invoke **wayfinder Materialize** with the map link (same or new chat).

Do **not** create **To Do** tickets - that is wayfinder **Materialize**.

## Interaction rules

1. **Breadth, not depth** - Do not drill into one unknown across zones; note it and move on.
2. **One zone per reply** - Default; user may ask to batch two adjacent zones in one reply.
3. **No stacked zones** - Finish the current zoneâ€™s turn before advancing the zone counter.
4. **Note scope edges** - Surface possible scope edges as fog candidates; wayfinder sorts ticket vs fog vs out-of-scope at materialize.
5. **Comment on map** - The canonical artifact lives as a map-issue comment, not a separate issue.

## Quick start

User (or wayfinder handoff): map issue #N, seed, target outcome. Reply with Recap + Session state + **Surfaces & experience** triage. Continue zone-by-zone; post final map-discovery comment; invoke Materialize.

See [REFERENCE.md](REFERENCE.md) for artifact layout and session-state examples.
