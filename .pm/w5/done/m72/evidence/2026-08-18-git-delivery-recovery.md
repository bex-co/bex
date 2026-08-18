# m72 Git-delivery recovery evidence — 2026-08-18

This record is intentionally credential-free. It contains immutable digests, session/ref/PR identifiers, lifecycle results, and cleanup counts, but no bearer token, OAuth client secret, OpenFGA key, model key, transcript body, or database credential.

## Reported regression and correction

- Production session `ags-da2daqui706c739sqvtg` on `bex-co/bex` failed its first turn at the final `git push --force-with-lease` with HTTP 403, `send-pack: unexpected disconnect`, and the contradictory `Everything up-to-date` suffix. A later turn delivered draft PR #57, proving that GitHub credentials were not globally unavailable and that the first closeout's one-repository proof had been overgeneralized.
- Commit `96f58943` makes the exact scanned candidate the publication verdict: an already-equal remote is success, the captured baseline may be advanced by the exact force-with-lease push, a different remote is a concurrent-update failure, and an ambiguous push error is accepted only after a fresh remote read proves the exact candidate is present. The gateway now accepts bounded, valid leading receive-pack `shallow <oid>` declarations, and a successful retry/dispatch clears the prior turn's `failureReason`.
- Commits `e008c232` and `b79a41bc` corrected the real-Postgres retry fixture without weakening the production uniqueness constraint. Commit `3e387d6c` independently upgraded `github.com/cilium/ebpf` past GO-2026-6238, which the required vulnerability gate found while this recovery was shipping.
- ADR047 D2/D4 records the receive-pack grammar and idempotent publication state machine. Driver bare-repository tests cover the already-published candidate and a conflicting remote; gateway tests cover valid and malformed shallow declarations; the store integration suite covers stale failure clearing.

## Production deployment

- Workflow [32194179340](https://github.com/bex-co/bex/actions/runs/32194179340) completed successfully for `b79a41bc6f425a0efca96b2b26e18ee4ec0dc889` at `2026-08-18T23:16:33Z`.
- Secret scan, supersession check, operator/envtest, real-Postgres + OpenFGA backend integration, 51-test agent driver suite, dashboard tests/build, five image builds, signatures/SBOM, every CRITICAL CVE gate, GitOps write-back, and Argo rollouts passed.
- Production `bex-api` and `bex-ssh-gateway` converged at 2/2 Ready on platform digest `sha256:add3e39ccf77ca15eb38f7857f8f8ab2847d5cb985692ee15cf27d1a617294b3`. `BEX_AGENT_SESSION_IMAGE` resolved to agent digest `sha256:324917770d39e6a6d3399d2b7fdd4297611537d94ee04e3d8042e69976966c4c`.

## Exact-repository live workflow

The operator-run verifier used a temporary isolated OAuth client authorized only for workspace `tea-d98210cbbpdc73dcrkvg`, the workspace's registered Claude provider, production egress, and repository `bex-co/bex`.

- Main session `ags-da2ege4qlbqc73e84d0g`, branch `bex-agent/verify-20260818231816-22473`, completed its first turn and published head `3810e43734f09e2c545b64667b58af5adf27a02a`.
- GitHub independently reported [draft PR #58](https://github.com/bex-co/bex/pull/58) `OPEN`, `isDraft=true`, on that exact branch. A steering turn used a fresh sandbox, advanced the same ref/PR to `8f4e03c4eecc83f0f5af456c59488ef6ac43983e`, and recorded `turns=2` with `deliveryMode=redispatch`.
- The completed session's attach ticket worked; a ticketless stream returned 401; terminal replay carried the v1 stream marker, 121 durable transcript parts, and `[DONE]`. Its `failureReason` was empty after both successful turns.
- Live-attach session `ags-da2eib88p6ds73ffuetg`, branch `bex-agent/verify-live-20260818231816-22473`, streamed parts while its sandbox was running, terminated with `[DONE]`, and a fresh attach replayed 40 durable parts before `[DONE]`.
- A non-`bex-agent/*` branch and an unknown adapter were refused. The bounded production log window contained neither replay `SQLSTATE 42501` nor a dead-container retry storm.
- The verifier reported `ALL AGENT-SESSION CHECKS PASSED (egress-profile=production)`. Its cleanup canceled its two exact sessions and removed every associated BatchSandbox and Pod. Read-back found zero temporary `tenant_members` rows, zero OpenFGA tuples, and HTTP 404 for the temporary Hydra client. The draft PR and branch remain as the reviewable delivery artifact; they were not merged.

This is the missing exact-repository proof: the corrected production image ran the same `bex-co/bex` Git delivery path that reported the 403, advanced it twice, and exposed both the draft PR and refreshable conversation without a stale failure reason or a hidden publication error.
