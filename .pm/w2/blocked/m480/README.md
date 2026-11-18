# w2 · m480 — Native builds get the Managed OIDC token at build time, as on Render; Dockerfile builds do not

**Worker:** worker2 **Goal:** A Pro-or-higher native-runtime service with a Managed OIDC trigger variable can read its provider token through the same `*_FILE` variables (and GCP configuration) in its build command, while Dockerfile and `builder: buildpack` builds get none and no image layer keeps the token. **Status:** blocked (t001–t003, t006, t007 done; t004 needs a production pin carrying this work and `68b45459b`, which clears by itself, then t005, t008)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Record the build-time boundary — **DONE** | 20m | w2/m477/t003 |
| t002 | Mint a build-scoped token and pass it as a BuildKit secret to native builds — **DONE** | 75m | t001 |
| t003 | BuildKit-gated tests: readable in native builds, absent from Dockerfile builds and every image layer — **DONE** | 45m | t002 |
| t004 | Live: a native build exchanges its token (gate: a production pin carrying this work) | 30m | t003, w2/m477/t005 |
| t005 | Render parity | 20m | t004 |
| t006 | Simplify — **DONE** | 15m | t005 |
| t007 | Test coverage — **DONE** | 30m | t005 |
| t008 | Closeout | 10m | t006, t007 |

## Definition of done

- A native-runtime service in a Pro workspace with a Managed OIDC trigger variable can read the token through the same `*_FILE` variables (and GCP configuration) in its build command, and the exchange with the in-cluster STS-compatible relying party succeeds on dev-2 or production.
- A Dockerfile build and a `builder: buildpack` build of the same service see neither the variables nor the files, as on Render.
- No layer of the built image contains the token (layer inspection in the BuildKit-gated test).

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w2` (2026-11-17); the user filed all five proposed milestones with `/pm them all to w2` the same day. That reopens the 2026-08-23 deferral recorded in [`.pm/FUTURE-MAYBE.md`](../../../FUTURE-MAYBE.md) (from `w2/025`) under the Render-parity-first rule (AGENTS.md, user decisions 2026-10-17/2026-10-20; precedent: `w2/m252` absorbed a FUTURE-MAYBE item under the same rule). Ledger: [ADR018](../../../../docs/ADR018-render-parity.md) row "Managed OIDC for AI providers" (✖ on REST/GraphQL/MCP/UI, the last all-✖ row that is not an anti-goal) and [ADR030](../../../../docs/ADR030-pricing.md)'s pricing-parity table ("Open gap | OIDC authentication"). render.com/docs/oidc § Limitations (build time).
- **Goal linkage:** Render parity first (ADR008): build-time behavior is part of the product.
- **Expected outcome:** Apps that fetch private build inputs from S3, GCS or a model API during a native build keep building after moving to bex.
- **Why now:** It reuses w2/m477's minter and the existing BuildKit secret path, so it is cheapest right after the runtime mechanism lands.
- **Render parity:** Included: builds triggered from REST, GraphQL, MCP, the dashboard and Blueprints behave alike; the native/Dockerfile split matches Render's page.
- **Depends on:** w2/m477 (minter).

## Park 2026-11-18

- **Shipped:** [ADR116 D12](../../../../docs/ADR116-managed-oidc.md#d12--build-time-w2m480). A native build decides its providers from the environment its build command sees (`status.buildManagedOIDC`). bex-api mints them into `<app>-oidc-build`, stamped with the build's generation, while the App builds. The operator copies them next to the Job (`bld-<app>-oidc`), and the authored build RUN alone reads them at `/var/lib/render/oidc/<file>` through BuildKit secret mounts, with the same file variables as at runtime. Dockerfile, `auto` and buildpack builds get nothing. Both Secrets go when the build ends.
- **Done:** t001–t003, t006, t007. The real-BuildKit gate ([t003](done/t003.md)) passed on the pinned BuildKit: the build command verified every token, a Dockerfile build of the same options saw neither variable nor file, and no blob of the image carried a token. It failed, as it should, with the token as a build arg or copied into the image.
- **Gate (clears by itself):** t004's live exchange cannot run on dev-2, because no native build runs on the arm64 local cluster: the pinned common run image has no linux/arm64 manifest. It needs a production pin carrying this milestone's commit and w2/m476's `68b45459b`. Check: `curl -sS https://oidc.bex.co/tea-daif693dqjvc73e7as3g/.well-known/openid-configuration` answers 200 (it answered 404 at 14:30Z), and production's `bex-operator` and `bex-api` images are at or after this commit. No user action is needed: the relying party can be `scripts/oidc-aws-verify.sh`'s MinIO STS run as a public QA web service, because a build pod reaches only public addresses. [t004](t004.md) has the steps.
- **Then:** t004, t005 (the surfaces all write the release environment the decision reads; walk them on production), t008.
