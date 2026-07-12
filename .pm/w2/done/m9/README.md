# w2 · m9 — Private-repo deploys + zero-config GitHub push-to-deploy

**Worker:** worker2 **Goal:** a private GitHub repo deploys end-to-end via the m8 connection, and a plain `git push` redeploys with no manual webhook setup. **Status:** **DONE 2026-07-12** — backend shipped 2026-07-11; t004's dashboard toggle re-landed 2026-07-12 (same codegen fix as m8/t006); t005 live acceptance PASSED 2026-07-12 against a real private repo on the mock cluster (evidence below).

## Tasks (in order)

| id   | title                                                                                        | est | depends_on |
| ---- | -------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Types + operator: `App.spec.cloneSecret` → BuildKit `GIT_AUTH_TOKEN` for the git context      | 45m | —          | — **DONE** |
| t002 | bex-api: mint installation token → clone Secret on create/deploy/webhook-redeploy             | 45m | t001       | — **DONE** |
| t003 | Webhook: accept GitHub-App-signed pushes (`BEX_GITHUB_WEBHOOK_SECRET` as second key)          | 30m | t002       | — **DONE** |
| t004 | Dashboard: Build & Deploy — Auto-Deploy toggle + "deploys via GitHub" source indicator        | 40m | t003       | — **DONE** (backend shipped 2026-07-11; toggle re-landed 2026-07-12, verified live — flips `spec.autoDeploy`, shows the via-GitHub source line; screenshot `.playwright-mcp/m9-autodeploy-toggle-on.png`) |
| t005 | Live acceptance: private repo → live URL; push → auto-redeploy; `autoDeploy:false` suppresses | 40m | t004       | — **DONE** (PASSED 2026-07-12, all four checks; evidence below) |
| t006 | Render parity — cross-surface consistency, ledger row "Git connections" → ✅                  | 30m | t005       | — **DONE** |
| t007 | Simplify — `/simplify` over the milestone's diff                                              | 30m | t006       | — **DONE** |
| t008 | Test coverage — token refresh, expired-token failure mode, two-key webhook, autoDeploy gate   | 40m | t006       | — **DONE** |
| t009 | Closeout — DoD verified, move to `done/`                                                      | 15m | t008       | — **DONE** (2026-07-12) |

## Definition of done

The t005 live loop passes against a real **private** GitHub repo on a real cluster: create from the repo → in-cluster build clones with an installation token → live URL; a `git push` to the tracked branch auto-redeploys (new revision in deploy history) with **zero manual webhook configuration on the repo**; `autoDeploy: false` suppresses the redeploy; an unsigned or wrongly-signed GitHub delivery is rejected 401.

## Live acceptance evidence (2026-07-12, local CAPD mock cluster)

Setup: GitHub App **bex-co** (id 2091812, installation 90623475); private test repo `bex-co/bex-hello-go-live` (a private copy of `examples/hello-go`, Dockerfile at root); app webhook temporarily retargeted to a cloudflared quick tunnel → local bex-api (restored to `https://api.bex.co/v1/webhooks/git` after the run); `BEX_GITHUB_WEBHOOK_SECRET` from the app; `bex-operator:dev` from main with `BEX_REGISTRY=192.168.147.4:30500`, `BEX_BUILD_NAMESPACE=bex-builds`.

1. **Private repo → live URL**: `POST /v1/services` (`repo=…/bex-hello-go-live`, `autoDeploy:"yes"`) minted `hello-live-clone` Secret + `spec.cloneSecret`; build Job `bld-hello-live-gen-1` Complete in 27s — cloned the **private** repo with the installation token, no auth error — pod Running, served `v1 private clone` (HTTP 200) at rev-1.
2. **Hands-free push**: commit `5900eaa` pushed to `main` → GitHub app delivery to the webhook (status 200 in the app's delivery log, **zero webhook config on the repo**) → `bld-hello-live-gen-2` → rev-2 → response changed to `v1 private clone — pushed v2`.
3. **`autoDeploy:false` suppresses**: GraphQL `setAutoDeploy(id:"hello-live", enabled:false)` → commit `7ad20c9` pushed → GitHub delivered it (status 200) but **no gen-4 build, revision unchanged, content still the v2 response**. (The `setAutoDeploy` spec patch itself triggered the expected gen-3 rebuild — generation bump, not a webhook redeploy.)
4. **Signature**: tampered `X-Hub-Signature-256` → 401; missing signature → 401; correctly signed `ping` event → `200 {"ignored":"ping"}`.

Dashboard (local `yarn dev` vs the live bex-api): Build & Deploy shows the Auto-Deploy switch with the "redeploys automatically via the GitHub app" source line (connection live + github.com repo); flipping it wrote `spec.autoDeploy` (verified via GraphQL `server.autoDeploy`). Screenshot `.playwright-mcp/m9-autodeploy-toggle-on.png`.

**Noted (follow-up filed, `w2/005.md`)**: deploy-history rows (w2/m5) exist only for store-managed apps (internal `POST /v1/apps` path), so `GET /v1/services/hello-live/deploys` is `[]` for this public-surface create — the new revision was evidenced via App CR status (rev-1→rev-2) + build Jobs. The store-managed path in turn does **not** mint `spec.cloneSecret` (its private-repo build failed with `could not read Username`), so the two paths need composing.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w2` 2026-07-11; second half of [docs/ADR018-render-parity.md](../../../docs/ADR018-render-parity.md) row "Git connections (GitHub / GitLab app)".
- **Goal linkage:** pillar 4 (deploy-from-chat / push-to-deploy) — completes the "a later git push redeploys" promise for private repos, hands-free.
- **Expected outcome:** private repos are first-class deploy sources; push-to-deploy needs no per-repo webhook or shared-secret handout — the GitHub App delivers signed pushes for every installed repo.
- **Why now:** direct sequel to w2/m8 (tokens/webhooks are useless without a connection); split from m8 to keep each milestone in the few-hours band per the end-to-end-milestones rule. Render parity task included: feature dev touching REST/GraphQL/MCP semantics (`autoDeploy`, webhook) + dashboard UI.

## Design decisions (from the brainstorm)

- **Operator stays mechanism-only and GitHub-free**: bex-api mints the 1h installation token and writes it to a k8s Secret; `spec.cloneSecret` points the build Job at it; BuildKit consumes it as its standard `GIT_AUTH_TOKEN` build secret.
- **Reuse `/v1/webhooks/git`**: GitHub App pushes carry the same `X-Hub-Signature-256`; the endpoint verifies against `BEX_GITHUB_WEBHOOK_SECRET` as a second accepted key. All matching (repo canonicalization, branch, rootDir, `autoDeploy`) already exists from w2/m2.
- **Accepted limitation** (ADR'd in m8/t001): an operator-side build retry >1h after its trigger finds an expired token and fails until the next deploy.
