# w2 · m77 — ADR059 hibernation: production enablement + resume-SLO evidence

**Worker:** worker2 **Goal:** turn on the hibernation tier w2/m68 shipped env-gated OFF: provision the SSE-enabled snapshot bucket + scoped credential, arm `BEX_AGENT_SNAPSHOT_S3_*` in prod, and prove a real session hibernates on idle and rehydrates on resume with recorded latency against the ADR059 SLOs. **Status:** DONE (2026-08-19)

## Definition of done

A real prod agent session hibernates when its idle grace elapses (phase `hibernated`, snapshot object under the workspace prefix in the SSE-enabled bucket, pod reclaimed) and rehydrates on Steer/Resume into a fresh sandbox with restored `/workspace` state; resume latency is captured from the m68 instrumentation (`service.go`'s `agent-session rehydrate: … resumed in …` log line) and recorded against ADR059's p50<~5s / p95<~15s SLOs; evidence is filed under `evidence/`; the rollback (unset env ⇒ reclaim reverts to Terminate) is documented.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w2` 2026-08-18 (round 2, item 3); w2/m68's DONE note ([.pm/w2/done/m68/README.md](../done/m68/README.md)) explicitly recorded "the live hibernate→rehydrate walk with recorded resume latency is the enablement step when the S3 contract is provisioned" — never filed anywhere.
- **Goal linkage:** [docs/ADR059-agent-sandbox-hibernation.md](../../../docs/ADR059-agent-sandbox-hibernation.md) names hibernation "the mandatory cost foundation" for agent sessions (pillar 5).
- **Expected outcome:** idle finished sessions stop burning compute and stop losing workspace state at terminate; the Hibernated tier is live.
- **Why now:** every idle prod session today burns the 30m grace then is terminated with state loss; the code is done and only the operational contract is missing.
- **Render parity omitted:** operational enablement of an already-shipped surface (m68 carried the parity task); no new REST/GraphQL/MCP/UI change.

## Tasks (in order)

| id   | title                                                                                                                                                | est | depends_on  |
| ---- | ---------------------------------------------------------------------------------------------------------------------------------------------------- | --- | ----------- |
| t001 | Provision the snapshot object store: SSE-enabled bucket (NEVER bex-tfstate), scoped durable credential, `.env`/`.env.example` names + gh-secrets flow | 45m | —          | — **DONE** |
| t002 | Arm prod: set the `BEX_AGENT_SNAPSHOT_S3_*` contract in the prod deploy secrets/manifests, deploy, confirm armed; review retention + pin-quota knobs  | 30m | t001        | — **DONE** |
| t003 | Live hibernate walk: run a session to completion, let idle grace elapse (or lower `BEX_AGENT_SANDBOX_IDLE_TTL` on a canary), verify phase/snapshot/pod-gone | 30m | t002        | — **DONE** |
| t004 | Live rehydrate walk: Steer/Resume, verify restored workspace state, capture resume latency vs the ADR059 SLOs                                         | 30m | t003        | — **DONE** |
| t005 | Evidence + docs: file the walk under `.pm/w2/m77/evidence/`, update ADR059's enablement trail and the m68 note                                         | 30m | t004        | — **DONE** |
| t006 | Simplify: run /simplify over whatever code/scripts this milestone touched (standing closing task)                                                     | 20m | t005        | — **DONE** |
| t007 | Test coverage: guard the env contract (all-or-nothing S3 settings validation) + regression-test any gap the live walks surface (standing closing task) | 30m | t005        | — **DONE** |
| t008 | Closeout: verify DoD, mark done, move milestone to done/ (standing closing task)                                                                      | 15m | t007        | — **DONE** |

## Notes

- Repo fact: bex-api's prod env/secrets are wired in `lego/operator/config/api/deployment.yaml` (secretKeyRef pattern, e.g. `bex-stripe` with `optional: true`), **not** in `deploy/gitops/` — that tree carries only bex-api RBAC/admission manifests. Secret material stays out of git (the `scripts/*-secret.sh` posture).
- Repo fact: `BEX_AGENT_SNAPSHOT_S3_*` is wired on the api Deployment as six optional `secretKeyRef`s to `bex-agent-snapshot`. Partial config fails startup (`ErrPartialS3SnapshotConfig`); all-unset stays Terminate-only.
- `.env.example` carries all eight variable names; t001 filled `.env` values. Dedicated bucket is `bex-agent-snapshots`.
- Out of scope: pinning UX and retention tuning shipped in m68; ADR050 per-store credential rotation stays its own deferral; no changes to the Completer semantics.
- DoD (2026-08-19): session `ags-da33092c0fus738gr25g` hibernated (object under `agent-snapshots/<ws>/`, pod gone) and Steer-rehydrated with `/workspace/m77-hibernate-walk.txt` intact; warm-node resume 3.12–3.25s; evidence `evidence/2026-08-19-hibernate-rehydrate-walk.md`; rollback remains delete Secret `bex-agent-snapshot`.
