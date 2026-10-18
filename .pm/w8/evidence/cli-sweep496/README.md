# CLI sweep 496 — workspaces and workflows read surfaces

**Verdict: no bug filed.** Zero fixtures were created; the sweep was read-only.

## Probes (all expected outcomes)

| probe | observed result |
| --- | --- |
| `bex workspaces -o json` | Exit `0`; the QA user's three memberships (`bex`, `tian-personal`, `bex-canary`) with stable `tea-…` IDs. |
| `bex workflows list -o json` | Exit `0`, literal `null` (workspace has no workflow services) — the null-vs-`[]` wire class closed repeatedly as Render-consistent; noted, not filed. |
| `bex jobs`/`restart` follow-ups | Covered in sweep 495; not repeated. |

## Safety and unrun checks

- Read-only sweep; nothing created, nothing to clean up; secrets unrecorded.
- Workflow create/dev/init mutate local or platform state beyond a read probe and were intentionally not exercised this sweep.
- Deploy lag means deployed revisions can precede local `main`.
