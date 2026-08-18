# m72 production recovery evidence — 2026-08-18

This record is intentionally redacted. It contains session/resource identifiers, phases, digests, commit IDs, and request outcomes, but no API key, database URI/password, kubeconfig, credential needle, or transcript body.

## Incident and immediate recovery

- Reported session: `ags-da1prbt040bc73aj5230`, workspace `tea-d98210cbbpdc73dcrkvg`, repository `bex-co/beancount-cms-v2`, branch `bex-agent/pricing-mobile-yearly-plan`.
- The conversation stream failed because the deployed `bex_ssh_gateway` role lacked `SELECT` on migration 0081's `agent_session_turns` table (`SQLSTATE 42501`). The repository was approximately 1.57 GiB, so the driver's old aggregate 1 GiB persisted-credential scan limit then failed. A second unguarded scrub repeated the failure and exited Node. The child Pod was `Failed`/exit 1 while its BatchSandbox remained stale `Pending`; two Completer replicas therefore retried exec against the absent container.
- The incident repair applied only the current least-privilege grant delta. `has_table_privilege('bex_ssh_gateway', 'agent_session_turns', 'SELECT')` became true, the scoped `AgentSessionTurns` read succeeded, and an unrelated sensitive-table read remained denied with `42501`. The gateway password and Kubernetes Secret were not rotated or rewritten, and no rollout was induced solely for the grant.
- An authenticated replay of the original session returned HTTP 200 with the v1 stream marker, four durable parts, and `[DONE]`; no new turn-table `42501` appeared.
- The original row now reports `failed` with bounded reason `sandbox terminated before completion`, retains its durable user turn, has no `sandboxId`, and was never relabeled successful. Exact BatchSandbox `3b34b8f2-55a9-42b0-a117-f94f5477d6e5` and Pod residue are absent.

## Shipped repair

Relevant commits, in implementation order:

- `5e798ab6` — converge durable replay, deploy-time grants/preflight, bounded large-tree scrubbing, one failure/scrub path, OpenSandbox terminal projection, and idempotent Completer fallback.
- `5f40fa25` — make verifier shellcheck invocation explicit.
- `d1789fb8` — normalize versioned OpenSandbox controller digests in deployment validation.
- `9e2f401d` — wait for a genuinely attachable sandbox in the live verifier.
- `2ecc8662` — propagate dead ACP transports as typed terminal failures.
- `1e4be41a` — ship working platform ACP profiles.
- `554c3d58` — bind Codex ACP to the exact model-proxy path/placeholder and reject typed provider failure instead of false-green completion.
- `7e733802` — keep GET/POST live attach streams alive with serialized 15-second SSE comment heartbeats; comments are transport-only and never enter the durable transcript. Also require the verifier's agent to match the workspace BYO provider and fix evidence/error-shape assertions.
- `723d2c3a` — deployment bot's immutable digest pin for `7e733802`.

The driver now scans files and Git objects as bounded streams with per-file, entry/inode, depth, time, and abort limits; it does not reject a legitimate tree merely because aggregate unrelated bytes exceed 1 GiB. Git object enumeration uses unordered object-store order and precomputed overlap. Known credential needles remain fail-closed across workspace, HOME/tmp, Git metadata, and reachable/unreachable objects. Delivery cannot run without a clean scrub verdict, and in-memory credentials are forgotten in `finally`.

Gateway grants now converge after migrations on every deploy without changing the role password/Secret. Startup preflight names a missing table/operation without exposing connection data. Failed Pods/terminated containers become typed terminal signals at the OpenSandbox and gateway boundaries; the Completer compare-and-set transition is idempotent across replicas and prunes stale in-memory failure state.

## Automated and deployment gates

- Agent driver tests cover a sparse tree larger than 1 GiB, every required persistence class, pathological bounds, abort/timeout, duplicate-cleanup prevention, delivery gating, ACP exit, and status-server survival.
- Scoped-role integration covers the complete native SSH, nonce, sandbox exec, and agent-attach store surface, including `AgentSessionTurns`, plus negative sensitive-table assertions and missing-grant preflight.
- The pinned real OpenSandbox controller envtest projects a non-zero child-Pod exit to terminal status. Gateway/sandbox/Completer regressions cover stale BatchSandbox + failed Pod, durable terminal replay without dialing the driver, concurrent observers, and bounded transient retries.
- Agent-attach unit and race tests cover idle live GET and POST streams receiving heartbeat comments before normal parts and `[DONE]`.
- Full local gates passed: backend `go test ./...`, focused `go test -race`, all-four-module `make lint` (zero issues), verifier `bash -n` + `shellcheck`, GitOps/deploy validation, and `git diff --check`.
- Production workflow [32119997957](https://github.com/bex-co/bex/actions/runs/32119997957) completed successfully for head `7e733802384a6eb24e696efa6cebefd9b656cddd`: secret scan; operator/envtest; backend integration + driver scrub; pinned OpenSandbox controller envtest; dashboard typecheck/lint/tests/build; five image builds; signatures/SBOM; all CRITICAL CVE gates; GitOps write-back; and Argo rollouts.

Deployed immutable digests and readiness after the workflow:

| component | digest | readiness |
| --- | --- | --- |
| platform (`bex-api`, gateway, manager) | `sha256:490c48afe20363bfd4cf40fb48fa722eff32f5fba666349fb605af0e04009aba` | API 2/2, gateway 2/2, manager 1/1 |
| agent sandbox | `sha256:503213b770432f6845b4cfe25aa1cb4b0e0c96e0dce57400df7c6cafb5b0c38d` | exact `BEX_AGENT_SESSION_IMAGE` value |
| OpenSandbox controller | `sha256:258465f7651b3b9b7f980704b24c1d5bc15941756a9015b6d169e4a9f33bbaec` | 1/1 |
| OpenSandbox server | `sha256:9d41dd48ff9d7322211601d1bfa5b3e548621c5daaf845ee5c5f0ddf6a4b6246` | 1/1 |

## Final production workflow

The final live verifier used `BEX_VERIFY_AGENT=claude`, matching the workspace's BYO provider credential, against `bex-co/beancount-cms-v2` with the production egress profile.

- Main session `ags-da22fiidiabc73edps60`, branch `bex-agent/verify-20260818093714-79351`, initial sandbox `7fee7be0-9ae0-47b5-9e34-b5494577faa8`, redispatch sandbox `159cdd80-a70e-494e-bfce-b1567299f2b6`.
- The first turn completed the 1.5 GiB repository workflow, tests, scrub, and delivery, opening draft PR [bex-co/beancount-cms-v2#20](https://github.com/bex-co/beancount-cms-v2/pull/20) at head `bd9fbd4928e7b56b68656b258942e9ea0908e2e5` with non-empty safe evidence.
- Steering created a second durable turn and advanced the same PR to `fdebe9961f539c423ff14165fa93ff38b50a52d4`; independent GitHub inspection confirmed `OPEN`, `isDraft=true`, exact branch, and exact head.
- A non-`bex-agent/*` branch and an unknown adapter were both refused at create. The latter was treated as the Render-shaped error body it is, never mistaken for a session ID.
- Terminal attach minted a session-bound ticket; ticketless GET returned 401; authenticated replay carried the v1 marker, non-empty durable transcript, and `[DONE]`.
- Live session `ags-da22kq2diabc73edpta0`, branch `bex-agent/verify-live-20260818093714-79351`, sandbox `37a41f01-02d2-433f-8902-fbcb800655da`, survived the prior edge-idle failure window, streamed teed parts, and ended with `[DONE]`. Reattach replayed those durable parts and ended with `[DONE]` again.
- The bounded production log window contained no turn-replay `SQLSTATE 42501` and no sustained `container not found` retry storm.
- The verifier's cleanup trap canceled its exact main/live sessions after proof. Both rows are honestly `canceled`; all three verification BatchSandboxes and Pods are absent. The original incident resource is also absent.

The verifier's final result was `ALL AGENT-SESSION CHECKS PASSED (egress-profile=production)`.

## Surface parity and simplification

REST, GraphQL, MCP, dashboard, and gateway use the same terminal phase/failure vocabulary and keep durable history readable after worker termination. Render has no coding-agent-session product; this is a deliberate Bex AI-native extension. The closest Render parity is durable operation/event history: worker death does not erase its durable record. The existing ADR047/ADR051/ADR059/ADR065 contract and ADR018 parity ledger record that distinction; no residual surface drift was found that requires a follow-up.

The `/simplify` pass applied the safe findings: single scrub promise/verdict, typed terminal/provider errors, per-cycle stale `statusFailures` pruning (including lost-CAS cleanup), a unified visited-entry bound, precomputed scan overlap, and Git `cat-file --unordered`. It deliberately did not remove the pre-exec Pod GET: moving phase classification only after exec failure creates a terminal/transient ambiguity and a TOCTOU regression at this trust boundary; an informer/cache would be a separate design. Buffer-window reuse, DB startup round-trip batching, verifier log streaming, and CI cache tuning were rejected as nonessential/riskier micro-optimizations for this incident closeout.
