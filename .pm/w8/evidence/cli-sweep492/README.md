# CLI sweep 492 — `services instances` edge dialects

**Verdict: no bug filed.** One free Key Value was created solely to probe the cross-kind dialect, then deleted by ID with a not-found absence proof; the baseline was restored exactly (one new row belongs to the concurrent hunt).

## Probes (all expected outcomes)

| probe | observed result | assessment |
| --- | --- | --- |
| `bex services instances -o json` (no ID, non-TTY) | Exit `1`: "`service id must be specified when output is not interactive`" | Precise upstream guard for the interactive browser form. |
| `bex services instances <unknown srv id>` | Exit `1`: "`failed to list instances: 404 Not Found`" | Correct not-found dialect — notably NOT the list fall-through closed in sweep 490, since `instances` is a real subcommand with a required positional. |
| `bex services instances <red-… KV id>` | Exit `1`: same `404` | Cross-kind IDs do not leak datastore instances through the service surface. |

## Safety and unrun checks

- No plan mutation; only the exact owned KV was deleted; secrets unrecorded.
- Deploy lag means deployed revisions can precede local `main`.
