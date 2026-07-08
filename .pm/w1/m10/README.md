# w1 · m10 — OpenBao prod wiring: env-vars live in prod

**Worker:** worker1 **Goal:** Close the gap where prod bex-api points at OpenBao (`BEX_OPENBAO_URL` is live in the api Deployment) but prod OpenBao was never initialized — so the shipped env-vars API and dashboard Environment tab 503 in prod. Wire init/unseal into CI exactly as docs/secrets.md's "Prod deploy path" prescribes. **Status:** todo

## Tasks (in order)

| id   | title                                                                              | est | depends_on   |
| ---- | ----------------------------------------------------------------------------------- | --- | ------------ |
| t001 | One-time prod `bao-init.sh`; keys/token into `.env` (+ mirror names in templates)   | 25m | —            |
| t002 | CI: `gh-secrets.sh` gains `BAO_*` keys; deploy.yml runs bao-init + bao-k8s-auth     | 30m | t001         |
| t003 | Prod sizing overlay: `server.ha.replicas`, drop local `storageClass` patch          | 20m | —            |
| t004 | Live acceptance: PUT env-vars on prod end-to-end; update secrets.md prod path       | 25m | t002         |
| t005 | Simplify — `/simplify` over the code this milestone changed                         | 20m | t004         |
| t006 | Test coverage — meaningful tests for the behavior this milestone shipped            | 30m | t004         |

## Definition of done

`PUT /v1/services/{id}/env-vars` against prod returns 200 (not 503); the value is in OpenBao under `tenants/default/services/<svc>/env` and survives a bex-api pod restart; a re-run of deploy.yml unseals idempotently (no re-init); docs/secrets.md no longer lists CI wiring as deferred.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w1` (2026-07-08); docs/secrets.md §"Prod deploy path" (steps written, unexecuted); 2026-07-08 docs-vs-code audit (`BEX_OPENBAO_URL` live in `lego/operator/config/api/deployment.yaml` while prod OpenBao is uninitialized).
- **Goal linkage:** completes w4/m6's tenant-credential path in prod; Render env-vars parity (pillar 1).
- **Expected outcome:** a dashboard-visible shipped feature (Environment tab, w4/m6.5) starts working in prod.
- **Why now:** the feature is already broken in prod today — the api points at a store that answers 503 on every credential verb. (Thematically at home in w4; placed in w1 as prod-roadmap work.)
