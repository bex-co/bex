# CLI sweep 493 — deploy cancel state-machine dialects

**Verdict: no bug filed.** One free web service was created, its deploy lifecycle exercised, and the service deleted by ID with a HTTP `404` absence proof; the baseline was restored exactly (one new row belongs to the concurrent hunt).

## Journey

| step | observed result |
| --- | --- |
| `bex deploys list <id> -o json` ~25s after create | One deploy, `status: live`, `trigger: other`, image ref exact, started/finished timestamps ordered. |
| `bex deploys cancel` on the live deploy | Exit `1`, HTTP `409` "`deploy … is already live`" — terminal states refuse cancellation precisely. |
| `bex deploys create` then cancel | Create exit `0` (new deploy ID); first cancel exit `0`. |
| Second cancel of the same deploy | Exit `1`, HTTP `409` "`deploy … is already canceled`" — idempotence is refused with a precise conflict rather than silently succeeding, matching the platform's strict dialect elsewhere. |

## Safety and unrun checks

- No plan mutation; only the exact owned ID was deleted; secrets unrecorded.
- Deploy lag means deployed revisions can precede local `main`.
