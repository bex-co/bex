# CLI sweep 537 — `services update --image` saves, deploy applies

**Verdict: no bug filed.** Same isolated session, `bex-canary`, HEAD-built CLI against production, 2026-10-19T10:31–10:38Z. One owned fixture (Google `hello-app`, whose body prints its version) created, flipped, and deleted; former-URL `404`.

| step | observed result |
| --- | --- |
| Create with `hello-app:1.0` | `live`; body serves `Version: 1.0.0`. |
| `services update --image …:2.0` | Exit `0`; saved `imagePath` reads back `2.0` everywhere, events record `source_changed`, `undeployedChanges: true` — and the running release keeps serving `1.0.0` for 4+ minutes with no deploy row opened. This is bex's documented saved-vs-serving release model (ADR004 §undeployedChanges), and matches Render's own API framing ("Most changes to your service will require a new deploy to take effect"). |
| `deploys create --wait` | Exit `0`; body flips to `Version: 2.0.0`, `undeployedChanges` clears. |

Notes, not findings: the pinned CLI's update JSON doesn't surface the bex-extension `undeployedChanges` flag (upstream struct has no such field), so a CLI-only user learns the change is pending only from the dashboard/REST read — inherent to the unmodified-client contract. Compare sweep 515, where a *config* edit on a suspended service opened a `config_change` row that canceled: source changes (`source_changed`) and config changes take deliberately different paths in the release model, and both read truthfully at every step.

Cleanup: exact-owned `srv-dbav31fmnaes738pfoo0` deleted by ID; former-URL 404 verified.

- Deploy lag means deployed revisions can precede local `main`.
