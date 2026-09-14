# Boundaries & integration

## Quick triage (read every session)

Skip this zone when the change stays **inside one process** with **no** new or altered **external** contract - no HTTP/RPC, no queues/events/webhooks, no new third-party calls, no new file formats exchanged.

**Common skip signals**

- Refactor with **identical** wire formats, message schemas, and call graphs.
- Local-only computation with **no** IO boundary changes (verify: logging/metrics can still imply boundaries).

If **any** new **endpoint**, **event type**, **topic**, **webhook**, **partner API** usage, or **auth boundary** moves, this zone is **in scope**.

## Deep prompts (load when not N/A)

- **Trust boundaries:** what is authenticated, authorized, rate-limited, and how are tokens/credentials handled?
- **Contracts:** request/response shapes, errors, versioning, deprecation strategy, compatibility guarantees.
- **Transport assumptions:** timeouts, retries, backoff, idempotency keys, duplicate delivery.
- **Routing & discovery:** how callers find this capability (paths, service names, DNS, gateways).
- **Side integrations:** email/SMS/push providers, payment, maps, identity providers - failure modes and fallbacks.

### Web app overlay (use when relevant)

- **HTTP routes/handlers**, middleware order, CORS, cookies/headers, CSRF where applicable.
- **BFF** vs direct-to-service: where orchestration lives.

### Non-web examples

- **Message queues/events:** ordering keys, partitioning, consumers, replay, poison-letter handling.
- **Batch/file exchange:** SFTP drops, CSV/Parquet schemas, checksums, partial uploads.
