# CLI sweep 527 — `services instances` across baseline types (read-only)

**Verdict: no bug filed.** Same isolated session, `bex-canary`, HEAD build `218dffcd1`, 2026-10-19T09:31Z. Reads against pre-existing baseline resources only; nothing woken or mutated (the public URL was deliberately not curled, since that would wake a hibernated baseline service).

| probe | observed result |
| --- | --- |
| `services instances` on two baseline cron jobs | Exit `0`, `[]` — the parity ledger's rule (cron has no standing instance to list). |
| `services instances` on the baseline web service | Exit `0`, `[]`; a metadata read shows `phase: Hibernated, replicas: 1` — auto-slept free tier at zero running pods, exactly the ledger's "observed-Hibernated returns `[]`" case, not a missing instance. |

## Safety and unrun checks

- Read-only; the hibernated service was left asleep.
- Positive instance listing on an awake owned service was sweep 506 (and 491/492 in the prior loop); suspended-state `[]` was sweep 515.
- Deploy lag means deployed revisions can precede local `main`.
