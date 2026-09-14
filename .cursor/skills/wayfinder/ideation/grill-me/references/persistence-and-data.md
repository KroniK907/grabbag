# Persistence & data

## Quick triage (read every session)

Skip this zone when **no** durable state changes - no DB, no object store, no new files written, no cache semantics changed, no retention/PII impact. Pure ephemeral compute or config-only changes might qualify (verify audit needs).

**Common skip signals**

- **Read-only** behavior change with **identical** stored data and queries (still check derived fields/caches).
- **Ephemeral** state only (with confirmed no audit/compliance trail requirement).

If **any** **schema**, **index**, **migration**, **new entity**, **PII**, **encryption**, or **permissions-at-rest** changes, this zone is **in scope**.

## Deep prompts (load when not N/A)

- **Entities & fields:** what is stored; nullability; uniqueness; lifecycle (create/update/archive/delete).
- **Migrations:** online vs offline; backfills; rollback; dual-write/dual-read phases if needed.
- **Access control at rest:** row-level rules, admin overrides, export restrictions.
- **Query patterns:** filters, sorts, pagination, hot paths, N+1 risks.
- **Retention & compliance:** GDPR/CCPA, legal hold, audit logs, encryption at rest/in transit, key rotation touchpoints.

### Web app overlay (use when relevant)

- **API payloads** vs **stored** shape: denormalization, caching, eventual consistency visible to UI.

### Non-web examples

- **Analytics/warehouse** tables vs OLTP; CDC; event sourcing snapshots.
