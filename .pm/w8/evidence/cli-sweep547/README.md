# CLI sweep 547 — image reference forms: untagged and digest-pinned

**Verdict: no bug filed.** Same isolated session, `bex-canary`, HEAD-built CLI, production still on the `8e57d4eaa121` pin, 2026-10-19T11:06–11:08Z. Two owned fixtures created and deleted by ID; final inventory shows zero leftovers.

| form | observed result |
| --- | --- |
| `--image docker.io/ealen/echo-server` (no tag) | Accepted and stored verbatim (no synthesized `:latest` in `imagePath`); deploy `live` within ~25 s and serves `200` — the pull defaults to `latest` at the kubelet, exactly as a Render user pasting a bare image name expects. |
| `--image …@sha256:000…0` (nonexistent digest) | Accepted at create; deploy settles `update_failed` in **98 s** with `code = NotFound … failed to resolve image` named in `failureReason`. Even on the pre-w8/131 pin this wording hits the existing permanence classifier (`code = notfound`) — a useful control confirming sweep 532's diagnosis that *only* Docker Hub's missing-**repository** answer ("pull access denied… insufficient_scope") evades the early settle. |

- Unrun: a real digest pin of an existing image (byte-identical serving), registry-with-port and `localhost` reference shapes (shape validity is `store.ValidateImage`'s tested surface).
- Deploy lag means deployed revisions can precede local `main`; w8/129, /130, /131 fixes remain ahead of the pin.
