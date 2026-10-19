# CLI sweep 532 — nonexistent-image deploy rides the full rollout budget

**Verdict: one bug filed** as [w8/131](../../131.md). Same isolated session, `bex-canary`, HEAD build `218dffcd1`, 2026-10-19T10:00–10:17Z. One owned fixture created, watched to terminal failure, deleted; no loop-owned resources remain.

A free prebuilt-image service pointing at `docker.io/library/qa532-no-such-image-zzz:latest`:

| time | observed |
| --- | --- |
| +0s | Create accepted; Events immediately carry `image_pull_failed` and a precise `stallReason` (Docker Hub: "pull access denied, repository does not exist or may require authorization: server message: insufficient_scope: authorization failed"). |
| +8s…+890s | Deploy reads only `update_in_progress` on the CLI; `services instances` lists the stuck pod's instance. |
| +906s | Settles `update_failed`, phase `Failed`, `failureReason` names the pull error verbatim; first release stopped (instances `[]` after settle). |

The early settle that should have fired — `predeploy.PermanentPullFailure` after its 90s grace, shared by rollout pods — substring-matches registry answers, and Docker Hub's wording contains none of its seven substrings, despite the function's comment stating missing-image/denied-auth answers are exactly its remit. Full record and fix: [w8/131](../../131.md) (uncommitted).

## Safety and unrun checks

- Cleanup: exact-owned `srv-dbaukm7b7irc73chfvvg` deleted by ID after its terminal settle.
- The registry-wording vector was captured from the live Events feed, not inferred; other registries' wordings (GHCR/ECR/GCR "manifest unknown" class) already match existing substrings and were not re-probed live.
- Deploy lag means deployed revisions can precede local `main`; the classifier gap is in HEAD's substring list, so lag does not explain it.
