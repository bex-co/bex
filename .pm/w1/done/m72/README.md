# w1 · m72 — Converge the ten `dev-N` harnesses onto one parameterized script

**Worker:** worker1 **Goal:** one dev-environment implementation instead of ten diverging copies, so a milestone verified on one worker's stack is verified on the same stack every other worker has. **Status:** done (2026-08-18) — t001–t008 **DONE**. The DoD's live two-workstream walk was **waived by the user** rather than rebuilding the shared CAPD cluster (its control-plane node lost `/etc/kubernetes/pki`, so the apiserver crashloops; restarting the stopped containers revived etcd/scheduler/controller-manager but not it). The milestone closes on the CI-green unit tests + shellcheck + the traceable union in the evidence; no live `up`/`status`/`down` walk is claimed — see `done/t007.md`.

**Measured:** tracked `dev-N` files **133 → 31**, tracked `dev-N` LOC **7,129 → 1,658**, and the shared implementation (script + tests + templates) is 1,204 LOC — a **net −4,267 tracked LOC**. `up.sh` variants **8 → 1**, `status.sh` **4 → 1**. `du -sh .pm` **4.7 GB → 21 MB**.

## Tasks (in order)

| id   | title                                                       | est | depends_on |
| ---- | ----------------------------------------------------------- | --- | ---------- |
| t001 | Reconcile the eight `up.sh` variants into one feature set     | 1h  | — — **DONE** |
| t002 | Write `scripts/dev-env.sh` (up / down / status, N-parameterized) | 2h | t001 — **DONE** |
| t003 | Migrate all ten workstreams onto it                           | 1h  | t002 — **DONE** |
| t004 | Bound the log growth                                          | 30m | t002 — **DONE** |
| t005 | Simplify                                                      | 30m | t003, t004 — **DONE** |
| t006 | Test coverage                                                 | 45m | t003, t004 — **DONE** |
| t007 | Closeout                                                      | 15m | t005, t006 — **DONE** (2026-08-18): live walk **waived by the user**; closed on the CI-green derivation/isolation tests + shellcheck |

## Definition of done

- One implementation — `scripts/dev-env.sh <N> {up|down|status}` — replaces the ten copied harnesses. Each `.pm/wN/dev-N/` keeps only what is genuinely per-workstream: `ports.env`, `README.md`, `.gitignore`, and any deliberate local override.
- Every capability present in any of the eight `up.sh` variants is either in the shared script or explicitly recorded as dropped, with a reason. Nothing is lost silently.
- All ten workstreams come up, report healthy, and tear down through the shared script. At least two are verified end to end on the live local cluster, including one of the divergent variants (`w5`, whose `up.sh` is 274 lines against `w2`'s 175).
- Log growth is bounded, so no harness can reach the 3.9 GB currently sitting in `.pm/w5/dev-5/logs`.
- Each `wN/README.md`'s "Local dev environment" section points at the shared script.
- No worker's running environment is destroyed by the migration.

## Source + Goal linkage

- **Source:** principal-engineer architecture review, 2026-08-18 (session hand-off, no inbox note). Measured at HEAD: 133 tracked files across ten `.pm/wN/dev-N/` directories, ~708 LOC copied ten times (7,078 tracked LOC total).
- **Goal linkage:** developer infrastructure, not product. It advances every roadmap milestone indirectly by making per-workstream verification evidence comparable.
- **Expected outcome:** ~7,078 tracked LOC collapses to one script plus ten small config files, and a fix to the dev stack reaches all ten workers instead of one.
- **Why now:** the copies have already diverged, and the divergence is now affecting what can be tested where. `up.sh` has **eight distinct variants across ten copies** (only `w2`/`w7`/`w10` still agree); `status.sh` has five. Concretely: `w1/dev-1/up.sh` wires `BEX_SMTP_ADDR` + `BEX_REQUIRE_VERIFIED_INVITE_EMAIL` and forwards a Mailpit SMTP port, and `w2/dev-2` does not — so invite-email behaviour simply cannot be exercised on some workers' stacks, and a milestone that verifies it depends on which workstream happened to run it. That is a correctness-of-evidence problem, and it compounds with every new copy.
- **Correction to the hand-off brief:** the brief asked to gitignore `dev-*/bin` and `dev-*/logs`. That is already done — each `dev-N/.gitignore` lists `.kubeconfig`, `.pids/`, `logs/`, `bin/`, and `git ls-files` confirms zero tracked files under either. The 3.9 GB is untracked local disk, so the real work there is bounding log growth (t004), not gitignoring.
- **Render parity closing task omitted:** this touches no REST/GraphQL/MCP/UI surface and no product code — it is local developer tooling only.

## Constraints

- `/pm` is normally the only skill that writes to `.pm/`. This milestone deliberately restructures files under `.pm/wN/dev-N/`; that is dev-harness tooling, not board state, and no milestone/task/inbox file is touched. Recorded here so the exception is visible rather than looking like drift.
- Another worker may be mid-session on any `dev-N`. Migration must not delete a running environment out from under them — see t003.

## What shipped

**One implementation.** `scripts/dev-env.sh <N> {up|down|status|clean|env}`, with the manifests templated under `scripts/dev-env/` (rbac, three CNPG Clusters, Mailpit, Kratos/Hydra/Loki/log-shipper values) and rendered at apply time. Each `.pm/wN/dev-N/` keeps only `ports.env`, `README.md`, `.gitignore` — plus `w9`'s `bootstrap-key.sh`, kept deliberately as a per-workstream extra (it mints a CLI-compat API key and is already parameterized by `BEX_DEV_ENV_DIR`; it is not part of up/down/status).

**The union, not the intersection** ([`evidence/variants.md`](evidence/variants.md)). Every capability any variant had is in the shared script: `w5`'s preflight and self-installing CNPG, its Loki + log-shipper (default on, `DEV_ENV_OBSERVABILITY=0` opts out), `w3`'s CAPD kubeconfig handling and platform-node label, `w4`/`w9`'s loopback TLS fix, `w3`/`w9`'s hydra-public forward and OAuth bootstrap, `w1`'s Mailpit-SMTP + invite-mail wiring, `w8`'s throwaway GitHub-App identity, and `w5`'s pass/fail verification inventory in `status`.

**Three defects the reconciliation surfaced** — the argument for doing this at all:

1. **Five of the ten harnesses could not start `bex-api`.** It refuses to boot with `BEX_CP_DB_URI` set and `BEX_OPENFGA_URL` unset unless `BEX_ALLOW_INSECURE_AUTHZ=1` (w1/m65 F16). Only `w5`/`w8` set it; `w2`/`w4`/`w6`/`w7`/`w10` set neither it nor anything equivalent, so their `up.sh` would exhaust its five start attempts and exit 1.
2. **The Hydra issuer pointed at a dead port in eight of ten** — `57000+N*10` is the Kratos-admin slot; only the two workstreams that actually forwarded `hydra-public` had it right.
3. **Three settings claimed the same port family** (`58000+N*10`): `w5`'s Loki, `w3`/`w9`'s hydra-public, and `w1`'s hard-coded Mailpit-SMTP. The converged map separates them, and only `w5`'s Loki port moves.

**The 3.9 GB, and why it existed.** Six port-forward logs of ~677 MB each, all written *after* the CAPD cluster died: `while true; do kubectl port-forward; sleep 1; done` with no backoff and no give-up is an error-per-second log bomb once the cluster is gone. The shared `forward()` keeps self-healing, backs off to 10s after five immediate failures, and gives up after 60 with a message naming the fix — so the root cause is closed, not just the symptom. `up` also truncates, and `clean` reclaims `logs/`+`bin/` while refusing to run against a live environment. `.pm` went **4.7 GB → 21 MB**.

**Tests** (`scripts/dev-env.test.sh`, wired into `scripts.yml` and shellcheck-clean): every per-N derivation distinct across N=1..10, no two settings sharing a port for one N (the collision above), `BEX_CP_IDENTITY` per-N, the checked-in `ports.env` records matching the derivation (this is what would have caught `w5`'s stale Loki port), invalid N rejected loudly (including `2; rm -rf /`), `down --dry-run` naming only its own namespaces with cross-N leakage asserted absent, `clean` refusing while up, and the override hook refusing to rename the namespaces isolation depends on.

**No worker's environment was destroyed.** No `dev-N` had a live process (every pid file was stale), the migration touched no `.kubeconfig`/`.pids`, and `logs/`+`bin/` were reclaimed only after confirming nothing was running.
