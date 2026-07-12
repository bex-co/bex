# w2 · m9 — Private-repo deploys + zero-config GitHub push-to-deploy

**Worker:** worker2 **Goal:** a private GitHub repo deploys end-to-end via the m8 connection, and a plain `git push` redeploys with no manual webhook setup. **Status:** code-complete (t001–t004, t006–t008 built + unit/envtest-tested + lint-clean; m8 built alongside as the prerequisite); t005 live acceptance pending a real private repo + cluster run (see t005).

## Tasks (in order)

| id   | title                                                                                        | est | depends_on |
| ---- | -------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Types + operator: `App.spec.cloneSecret` → BuildKit `GIT_AUTH_TOKEN` for the git context      | 45m | —          | — **DONE** |
| t002 | bex-api: mint installation token → clone Secret on create/deploy/webhook-redeploy             | 45m | t001       | — **DONE** |
| t003 | Webhook: accept GitHub-App-signed pushes (`BEX_GITHUB_WEBHOOK_SECRET` as second key)          | 30m | t002       | — **DONE** |
| t004 | Dashboard: Build & Deploy — Auto-Deploy toggle + "deploys via GitHub" source indicator        | 40m | t003       | — **DONE** (code + backend `setAutoDeploy`/read field; UI needs `yarn codegen` + build to verify) |
| t005 | Live acceptance: private repo → live URL; push → auto-redeploy; `autoDeploy:false` suppresses | 40m | t004       | — **OPEN** (needs a real GitHub App + private repo + cluster; runbook in t005) |
| t006 | Render parity — cross-surface consistency, ledger row "Git connections" → ✅                  | 30m | t005       | — **DONE** |
| t007 | Simplify — `/simplify` over the milestone's diff                                              | 30m | t006       | — **DONE** |
| t008 | Test coverage — token refresh, expired-token failure mode, two-key webhook, autoDeploy gate   | 40m | t006       | — **DONE** |
| t009 | Closeout — DoD verified, move to `done/`                                                      | 15m | t008       | — **OPEN** (gated on t005 live run) |

## Definition of done

The t005 live loop passes against a real **private** GitHub repo on a real cluster: create from the repo → in-cluster build clones with an installation token → live URL; a `git push` to the tracked branch auto-redeploys (new revision in deploy history) with **zero manual webhook configuration on the repo**; `autoDeploy: false` suppresses the redeploy; an unsigned or wrongly-signed GitHub delivery is rejected 401.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w2` 2026-07-11; second half of [docs/render-parity.md](../../../docs/render-parity.md) row "Git connections (GitHub / GitLab app)".
- **Goal linkage:** pillar 4 (deploy-from-chat / push-to-deploy) — completes the "a later git push redeploys" promise for private repos, hands-free.
- **Expected outcome:** private repos are first-class deploy sources; push-to-deploy needs no per-repo webhook or shared-secret handout — the GitHub App delivers signed pushes for every installed repo.
- **Why now:** direct sequel to w2/m8 (tokens/webhooks are useless without a connection); split from m8 to keep each milestone in the few-hours band per the end-to-end-milestones rule. Render parity task included: feature dev touching REST/GraphQL/MCP semantics (`autoDeploy`, webhook) + dashboard UI.

## Design decisions (from the brainstorm)

- **Operator stays mechanism-only and GitHub-free**: bex-api mints the 1h installation token and writes it to a k8s Secret; `spec.cloneSecret` points the build Job at it; BuildKit consumes it as its standard `GIT_AUTH_TOKEN` build secret.
- **Reuse `/v1/webhooks/git`**: GitHub App pushes carry the same `X-Hub-Signature-256`; the endpoint verifies against `BEX_GITHUB_WEBHOOK_SECRET` as a second accepted key. All matching (repo canonicalization, branch, rootDir, `autoDeploy`) already exists from w2/m2.
- **Accepted limitation** (ADR'd in m8/t001): an operator-side build retry >1h after its trigger finds an expired token and fails until the next deploy.
