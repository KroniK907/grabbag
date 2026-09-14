# Change, risk & evidence

## Quick triage (read every session)

Skip this zone for **personal/local** change with **no** production path, **no** collaborators, **no** rollback concern - e.g. a draft note. Anything merged for others to run almost always needs at least a lightweight pass.

**Common skip signals**

- Truly **local-only** experiment not shared (explicitly discarded caveat).

If **any** rollout, customer impact, **security** sensitivity, **SLO** risk, or need to **prove** correctness exists, this zone is **in scope**.

## Deep prompts (load when not N/A)

- **Rollout:** feature flags, gradual release, kill switches, config toggles, canary strategy.
- **Backwards compatibility:** old clients, old jobs, old data, deprecation windows, breaking changes comms.
- **Operational readiness:** runbooks, on-call impact, playbooks for failure modes.
- **Observability:** logs, metrics, traces, alerts - what to watch; expected error budgets.
- **Security & abuse:** threat model deltas, secrets handling, data exfiltration paths, rate limits.
- **Evidence:** tests (unit/integration/contract), staging checks, acceptance criteria, sign-off owners.

### Web app overlay (use when relevant)

- **Cache/CDN** invalidation, SEO/indexing impact, client bundle size or perf regressions.

### Non-web examples

- **Pipeline/CI** gates; **database** migration risk windows; **consumer** contract tests for events.
