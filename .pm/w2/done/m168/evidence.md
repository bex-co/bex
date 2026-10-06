# w2/m168 evidence

## Pre-rollout checks (2026-10-06, read-only against `hetzner-prod`)

- **Card gate.** `bex-stripe` Secret `BEX_REQUIRE_PAYMENT_METHOD=all`. The 045 note claimed the gate was empty; that was a misread, because `kubectl get deploy` can't show a `secretKeyRef` value.
- **Consent gate wiring.** The dashboard Deployment reads `BEX_OPS_ROLE_TOKEN` from Secret `bex-ops`, which exists and has that key. bex-api already serves `/internal/ops-role` with the same bearer, so `/internal/identity-claims` mounts on deploy.
- **SSH gateway wiring.** `bex-ssh-gateway` has `BEX_SANDBOX_EXEC_SECRET`, and its other :8091 URLs point at `bex-api.bex-system.svc`. The default `BEX_IDENTITY_VERIFICATION_API_URL` matches.
- **SSH key owners.** There are 2 keys:
  - one with a verified human owner;
  - one owned by `bex-bootstrap` (`m39-exact`, a w2/m39 acceptance key). Kratos admin answers 404 for that subject, which classifies it as a machine and exempts it.
- **Kratos admin 404 shape.** Probed for both a non-UUID subject (`bex-bootstrap`) and an unknown UUID; both return 404, confirming the human/machine split.
- **Unverified members of first-party or active workspaces.**
  - Most are `example.com` QA fixtures.
  - Four real addresses (redacted `on***@gmail.com`, `rg***@gmail.com`, `jj***@gmail.com`, `mf***@bex.co`) are unverified.
  - The login backstop (`require_verified_address`) already refuses them any _new_ session. The gate newly affects only their still-live sessions and CLI refresh tokens.
  - Per the user decision (no grandfathering), they recover by verifying through the resend flow. Their apps keep running, because the gate is API-only.
- **Rollout ordering risk.** If the dashboard rolls out before bex-api has the identity-claims verb, consent fails closed: OAuth logins are refused for minutes until bex-api is up.

## Live production check (t006, 2026-10-06 ~10:30 UTC)

Deployed through pin `f3b7b4cd7` (images from `df294ac76`, which contains `22180488a`). New digests rolled out on bex-api, bex-ssh-gateway and the dashboard.

- **Probe A (API flow).** Kratos `registration/api` returned an unverified identity plus a session token.
  - `GET https://api.bex.co/v1/services`, `POST /graphql` and `POST /mcp` (tools/list) each returned **403** with the identical body: `{"code":"EMAIL_VERIFICATION_REQUIRED", …}`.
  - `tenants`/`tenant_members` show **0** rows for the subject, so no workspace was minted.
- **Probe B (browser).** Sign-up in Playwright landed on `/auth/verification`, signed in, with the new subtitle and a sign-out link.
  - Navigating to `/` or to `/services` bounced back to `/auth/verification`, carrying `next` for the deep link.
  - An OAuth `authorize` for the `skip_consent` client `bex-forum` sent the browser to the login page with a `login_challenge`. Kratos asked the existing unverified session to **reauthenticate**, and after the password the `require_verified_address` backstop routed it to `/auth/verification`.
  - Hydra shows **no consent sessions** for either probe subject, and no code was issued.
  - So in prod the login backstop fires before consent. The w2/045 note's inferred "sign-up session silently completes OAuth" did **not** reproduce. The consent gate remains the defense-in-depth layer, for example for a Hydra-remembered login whose email later became unverified; its unit tests cover every client class.
- **Cleanup.** Both probe identities were deleted through Kratos admin (204, then 404 on read).

## Local acceptance (t009, dev-2, 2026-10-06)

- **Cluster.** The CAPD app cluster had disappeared, and `scripts/mock-cluster.sh` hung on a locked macOS Keychain: Helm v4's oras auto-detects `osxkeychain` from `~/.docker/config.json`. Workaround: a stub credential helper (`credsStore: "none"` plus a `docker-credential-none` that answers "not found"), passed through `PATH`, `DOCKER_CONFIG` and `HELM_REGISTRY_CONFIG`. The public chart then pulled, the cluster provisioned with every check `ok`, and `dev-env.sh 2 up` succeeded, printing the new `BEX_IDENTITY_CLAIMS_URL`/`BEX_OPS_ROLE_TOKEN` dashboard wiring.
- **API journey.** A Kratos API-flow sign-up produced an unverified session. REST returned 403; GraphQL and MCP both returned `EMAIL_VERIFICATION_REQUIRED`. After the Mailpit code went through the verification flow, the **same session token** got REST 200, with no re-login.
- **Browser journey.** Sign-up landed on `/auth/verification` (signed in). An OAuth authorize for a dev third-party client asked for reauthentication, and the login backstop sent it back to `/auth/verification`. Entering the Mailpit code continued to `/setup/payment` (D7).
- **Consent gate, live.** After verifying, the same authorize reached the consent card, so the GET passed the real identity-claims verb. The identity was then un-verified through Kratos admin, and **Approve** redirected to `cb?error=access_denied&error_description=Verify+your+email+address+before+signing+in+to+other+apps`. Hydra holds no consent session for the subject.
- **Not exercised live** (test-covered only):
  - Native SSH: dev-2 runs no ssh-gateway of its own. `nativessh` tests drive real SSH handshakes against the gateway for both refusal-at-auth and refusal-on-next-channel, with mutation-checked assertions.
  - The pinned Render CLI device login: device approval goes through the same consent acceptor, and an unverified subject can't reach it past the login backstop. An already-held human OAuth token is refused by the middleware's Kratos-admin resolution, covered by unit tests.
- **Cleanup.** The dev client and both dev identities were deleted (204). dev-2 is left up.
