# CLI sweep 944 — suspend/resume webhooks fire from `Running`, not from `Deploying` (live, 2026-10-20)

Five disposable service+webhook pairs created and deleted (204s throughout). No new bug filed — the apparent gap dissolved under a phase-aware retest.

- Subscribing to `service_suspended`/`service_resumed` and suspending a **still-`Deploying`** service (held a full 60 s) produced no deliveries, while the same endpoint received `deploy_ended` (sweep 943) and the REST feed showed `suspender_added`/`suspender_removed`. First hypothesis: a fan-out gap.
- Decisive retest: wait for phase `Running`, then suspend → `service_suspended` delivered within ~24 s; resume → `service_resumed` delivered. The webhook types derive from observed phase transitions (`store/event_facts.go` `observedStateFacts`: a `Hibernated` transition with `spec.suspended` set), so a service suspended before its first `Running` observation has no transition edge to project — its audit-derived feed events exist, but no typed fact is minted. Recorded as understood behavior (a never-ran service has no suspension edge), not filed.
- The resumed poll also showed the suspended delivery twice in history — consistent with at-least-once retries being recorded per attempt (the receiver answers 405); not treated as a duplicate-delivery defect.
