# CLI sweep 500 — suspend/resume round trip

**Verdict: no bug filed.** One free web service was created, suspended and resumed twice over, and deleted by ID with a HTTP `404` absence proof; the baseline diff reflects only the concurrent hunt's churn.

## Journey

| step | observed result |
| --- | --- |
| `POST /v1/services/{id}/suspend` | HTTP `202`; service reads `suspended: suspended`, `suspenders: ["user"]`. |
| Suspend again | HTTP `202` — idempotent acceptance, matching Render's 202 contract for both calls. |
| `POST /v1/services/{id}/resume` | HTTP `202`; service reads `suspended: not_suspended`; `bex services instances` shows a running instance again. |
| Resume again | HTTP `202` — same idempotent acceptance; no error, no state corruption. |

## Safety and unrun checks

- No plan mutation; only the exact owned ID was deleted; secrets unrecorded.
- Suspended-period ingress behavior was not probed (the service was resumed within seconds; earlier sweeps covered suspended traffic dialects).
- Deploy lag means deployed revisions can precede local `main`.
