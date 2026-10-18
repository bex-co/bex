# CLI sweep 491 — `services instances` on a free web service

**Verdict: no bug filed.** One free web service was created, its instances listed from just after create through steady state, and the service was deleted by ID with a HTTP `404` absence proof; the baseline was restored modulo the concurrent hunt's churn.

## Journey

| step | observed result |
| --- | --- |
| `bex services instances --help` | Real subcommand upstream (unlike the `get` fall-through closed in sweep 490): `bex services instances [serviceID]`. |
| `bex services instances <id> -o json` right after create | Exit `0`, one row (`id`, `createdAt`) for the single free-tier instance. |
| Same, ~2 minutes later | The identical instance row — stable identity, no duplicate/phantom instances across the rollout. |

The instance identity is opaque and stable, the shape matches the Render contract, and no `numInstances` scaling was attempted (free plans run one instance; scaling is a paid surface outside this loop's approved fixture class).

## Safety and unrun checks

- No plan mutation; only the exact owned ID was deleted; secrets unrecorded.
- Deploy lag means deployed revisions can precede local `main`.
