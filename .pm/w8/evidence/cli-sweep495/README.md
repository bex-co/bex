# CLI sweep 495 — one-off jobs contract and restart

**Verdict: no bug filed.** One free web service was created, probed, restarted, and deleted by ID with a HTTP `404` absence proof; the baseline diff showed one row removed by the concurrent hunt and no additions of mine left behind.

## Journey

| step | observed result | assessment |
| --- | --- | --- |
| `bex jobs list <id> -o json` | `[]` at exit `0` | Existing jobs remain listable, as the create refusal promises. |
| `bex jobs create <id> --start-command "echo qa495"` | Exit `1`, HTTP `410` (`ONE_OFF_JOBS_UNSUPPORTED`): one-off jobs are not supported on bex, with named alternatives (SSH, pre-deploy command, cron job) | Deliberate platform contract with an honest, precise refusal — not a defect. |
| `bex restart <id> --confirm` | Exit `0`, "`<id> restarted successfully`"; `services instances` then showed a fresh instance identity | Real restart (new instance), not a no-op echo. |

## Safety and unrun checks

- No plan mutation; `--plan-id` on jobs was never passed; only the exact owned ID was deleted; secrets unrecorded.
- Deploy lag means deployed revisions can precede local `main`.
