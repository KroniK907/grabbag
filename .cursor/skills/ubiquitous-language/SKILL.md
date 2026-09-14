---
name: ubiquitous-language
description: ubiquitous language, DDD glossary, domain model terms, build a glossary, harden terminology, define domain terms, UBIQUITOUS_LANGUAGE.md
agent-config-sync: true
---

# Ubiquitous Language

Extract domain terms from the current conversation into a consistent glossary and save it locally.

## Process

1. **Scan the conversation** for domain-relevant nouns, verbs, and concepts
2. **Identify problems**:
 - Same word used for different concepts (ambiguity)
 - Different words used for the same concept (synonyms)
 - Vague or overloaded terms
3. **Propose a canonical glossary** with opinionated term choices
4. **Write to `UBIQUITOUS_LANGUAGE.md`** in the working directory using the format below
5. **Output a summary** inline in the conversation

**Done when:** `UBIQUITOUS_LANGUAGE.md` exists (or is updated) and the summary lists term count, new entries, and flagged ambiguities.

## Output Format

Write a `UBIQUITOUS_LANGUAGE.md` file with this structure:

```md
# Ubiquitous Language

## Order lifecycle

| Term | Definition | Aliases to avoid |
|------|-----------|-----------------|
| **Order** | A customer's request to purchase one or more items | Purchase, transaction |
| **Invoice** | A request for payment sent to a customer after delivery | Bill, payment request |

## People

| Term | Definition | Aliases to avoid |
|------|-----------|-----------------|
| **Customer** | A person or organization that places orders | Client, buyer, account |
| **User** | An authentication identity in the system | Login, account |

## Relationships

- An **Invoice** belongs to exactly one **Customer**
- An **Order** produces one or more **Invoices**

## Example dialogue

> **Dev:** "When a **Customer** places an **Order**, do we create the **Invoice** immediately?"
> **Domain expert:** "No - an **Invoice** is only generated once a **Fulfillment** is confirmed. A single **Order** can produce multiple **Invoices** if items ship in separate **Shipments**."
> **Dev:** "So if a **Shipment** is cancelled before dispatch, no **Invoice** exists for it?"
> **Domain expert:** "Exactly. The **Invoice** lifecycle is tied to the **Fulfillment**, not the **Order**."

## Flagged ambiguities

- "account" was used to mean both **Customer** and **User** - these are distinct concepts: a **Customer** places orders, while a **User** is an authentication identity that may or may not represent a **Customer**.
```

## Rules

- **Be opinionated.** When multiple words exist for the same concept, pick the best one and list the others as aliases to avoid.
- **Flag conflicts explicitly.** If a term is used ambiguously in the conversation, call it out in the "Flagged ambiguities" section with a clear recommendation.
- **Keep definitions tight.** One sentence max. Define what it IS, not what it does.
- **Show relationships.** Use bold term names and express cardinality where obvious.
- **Only include domain terms.** Skip generic programming concepts (array, function, endpoint) unless they have domain-specific meaning.
- **Group terms into multiple tables** when natural clusters emerge (e.g. by subdomain, lifecycle, or actor). Each group gets its own heading and table. If all terms belong to a single cohesive domain, one table is fine - don't force groupings.
- **Write an example dialogue.** A short conversation (3-5 exchanges) between a dev and a domain expert that demonstrates how the terms interact naturally. The dialogue should clarify boundaries between related concepts and show terms being used precisely.

## Re-running

When invoked again in the same conversation:

1. Read the existing `UBIQUITOUS_LANGUAGE.md`
2. Incorporate any new terms from subsequent discussion
3. Update definitions if understanding has evolved
4. Mark changed entries with "(updated)" and new entries with "(new)"
5. Re-flag any new ambiguities
6. Rewrite the example dialogue to incorporate new terms

## Post-output

After writing the file, tell the user:

- The file path
- How many terms are defined
- Any flagged ambiguities

State that you will use these terms consistently in this conversation. If the user spots drift or a missing term, they can invoke this skill again.
