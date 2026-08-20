# ADR063: Security review round 9 (codex-security repo scan)

- **Status:** Accepted (remediation in place)
- **Date:** 2026-08-15
- **Scan:** codex-security `bex/codex-security-bex-5qGbtu` (standard single-pass static review at revision `4561a4e`; 13 findings — 2 high, 6 medium, 5 low, all high confidence)
- **Lineage:** ninth pass in the ADR028 → ADR045 → ADR055 → ADR056 → ADR057 → ADR072 → ADR061 lineage. The four repeats below re-confirm standing diserrals rather than opening new ones.

## Summary

| # | Finding | Severity | Disposition |
| --- | --- | --- | --- |
| 1 | Agent can publish an encoded reusable model credential through Git | high | **Fixed in place** — encoded-form needles + object-level blob scan before push |
| 2 | Routine operator credential can escalate through arbitrary cluster-wide Jobs | high | **Fixed in place** — `jobs:create` removed from the day-to-day ClusterRole (break-glass for manual triggers) |
| 3 | Shared production tenant suffix permits cross-tenant cookie tossing | medium | Accepted (6th report — `onbex.co` PSL, ADR055 F9 lineage) |
| 4 | Agent Git proxy permits unbounded slow request bodies | medium | **Fixed in place** — pre-lookup concurrency caps + body byte/read deadlines |
| 5 | Developers can delete administrator-controlled protected environments | medium | **Fixed in place** — ACL-bearing environment delete requires `can_manage`, fresh |
| 6 | Revocation does not terminate established gateway streams | medium | **Fixed in place** — live-stream revalidation watchdogs on all three transports |
| 7 | Sensitive/destructive data operations accept stale authorization after revocation | medium | **Fixed in place** — fresh checks at the five remaining sinks |
| 8 | Contributors can change executable build inputs reserved for developers | medium | **Fixed in place** — `SetRootDir`/`SetDockerfilePath` reclassified to `can_create` |
| 9 | Production metrics-server disables kubelet certificate verification | low | **Closed by w2/m81** (3rd report — round-7 #5 → ADR061 #11 → this) |
| 10 | DNS registry names permit blind node-origin SSRF during image pulls | low | Accepted residual (repeat — round-7 #6 / ADR061 #12) |
| 11 | Mobile live-log stream retains the entire unbounded SSE response | low | **Fixed in place** — byte budget recycles the stream; parser caps pending frames |
| 12 | Privileged deployment and runtime inputs lack independent integrity verification | low | Deferred (3rd report — digest-pinning inventory, ADR055 F7 lineage) |
| 13 | Operator admission permits host-kernel capabilities and direct platform-node placement | low | **Fixed in place** (targeted half) — admission grammar extended to the fields the generators already set |

Nine findings addressed with regression tests; four are unchanged repeats of tracked deferrals/accepts.

## 1 — Encoded credential exfiltration through delivery (high): fixed in place

The round-5 #6 pre-push guard matched only the raw and JSON-escaped renderings of the model credential, and its `git log -p` scan omits binary blob bytes — so a base64/hex rendering, or the raw key inside a binary blob, reached `git push`. An agent that cannot print the key can trivially print an encoding of it.

Fixed at both layers (`lego/agent-image/driver/src/credentials.ts`, `delivery.ts`):

- **Needle set** — the credential manager now derives every common reversible encoding (base64 std and URL-safe, padded and unpadded; hex both cases; gated to secrets ≥16 bytes so short values cannot false-positive) and applies the whole set everywhere the literals applied: `containsSecret`, `redact`/`redactPart`, and the persisted-state file scrub.
- **Object-level scan** — delivery enumerates every object the branch newly reaches (`rev-list --objects --no-object-names base..branch`), reads each blob's raw bytes (`cat-file blob` with `encoding:"buffer"`, so binary content is not mangled by a UTF-8 string decode), and byte-scans every needle. A blob too large to inspect (>32 MiB, matching the scrubber's file ceiling) or a payload past the 256 MiB total scan budget **refuses the push** — uninspectable new objects are exactly what an exfiltration hides behind. The textual patch scan stays as the cheap first pass.

Tests (`test/delivery.test.ts`): a base64-encoded credential in a text file and the raw key between NUL bytes in a binary blob both refuse the push and leave the remote untouched; a clean branch with the full needle set active pushes unchanged.

**Residual:** pattern DLP remains defense-in-depth. The finding's full remediation landed in [ADR062-sandbox-credential-vault.md](ADR062-sandbox-credential-vault.md); [ADR064](ADR064-security-review-round10.md) subsequently made the proxy mandatory and removed the direct-key path entirely.

## 2 — Routine operator `jobs:create` (high): fixed in place

The day-to-day `bex-operator` ClusterRole held cluster-wide `jobs: create`, which is arbitrary pod-template authoring (privileged, host-mounted, any ServiceAccount, any node) — while the workload admission policies constrain only `bex-controller-manager`. The standing `w7/020` deferral (retired to `.pm/FUTURE-MAYBE.md` with the trigger "the ops role is broadened") had already named this exact gap; the round-9 high is that trigger firing.

Fix: **removed `jobs:create` from the routine ClusterRole entirely** (`deploy/gitops/base/operator-daytoday-rbac.yaml`). The documented use — `kubectl create job --from=cronjob/...` in the backup/rekey runbooks — is restore-grade, rare, and deliberate: those runbooks (ADR011, ADR015, ADR037) now say to use the break-glass admin kubeconfig for the trigger step. `jobs: delete` alone stays for stuck-run cleanup. `scripts/operator-kubeconfig.sh` gained a smoke test asserting the minted credential **cannot** create Jobs. A CEL admission allowlist for "exact approved maintenance templates" was rejected: the approved templates (etcd/openbao backup) are themselves hostNetwork + control-plane hostPath jobs in `kube-system`, so any grammar permissive enough to run them is permissive enough to escalate with; removing the verb is the only clean boundary.

## 3 — Shared tenant suffix cookie tossing (medium): accepted, unchanged

Sixth report of the `onbex.co` PSL finding (ADR055 F9 → ADR056 9 → ADR057 1 → ADR061 4 → this). The control plane stays on `*.bex.co`; tenant-app isolation under the shared suffix awaits PSL eligibility (~2k distinct users vs ~17 today) or a per-tenant sub-suffix restructure — `.pm/w1/050.md` tracks the operator action. No code change.

## 4 — Git proxy slow-body exhaustion (medium): fixed in place

The agent Git smart-HTTP proxy did its Pod lookup and credential mint before any body read, and `validateReceivePack`'s blocking `io.ReadFull` calls had no deadline — a verified session Pod could hold many goroutines, descriptors, and Kubernetes API lookups by dripping request bodies at the shared gateway.

Fix (`agentcred/agentcred.go`, `cmd/ssh-gateway/main.go`):

- A `SessionLimiter` (global 64, per source Pod IP 4 — `BEX_AGENT_GIT_MAX_CONNS[_PER_POD]`) whose slot is acquired **before** the Pod lookup and held across the mint, pkt-line validation, and upstream exchange; a shed request answers 429 and is metered.
- The request body is wrapped in `http.MaxBytesReader` (256 MiB).
- The dedicated git-proxy listener gets a whole-request `ReadTimeout` (`BEX_AGENT_GIT_READ_TIMEOUT`, default 10m — orders of magnitude past any in-cluster pack exchange) plus an `IdleTimeout`; response streaming (the clone direction) is a write and stays unbounded.

Test: the over-limit concurrent request from one source answers 429 **without** a second Pod lookup.

## 5 — Developer deletion of protected environments (medium): fixed in place

`environments.Service.Delete` required only `can_create` while `SetACL` on the same protected-environment state requires `can_manage` — a developer could dismantle the administrator boundary (member protection, network-isolation labels, inbound-IP layer) by deleting the environment that carries it, since Delete clears all of those on the way to the row delete.

Fix: when the environment is ACL-bearing (`protectedStatus == protected`, network isolation enabled, or a non-empty IP allowlist), Delete additionally asserts `can_manage` — **fresh** (uncached), immediately before the destructive fan-out. A bare environment keeps the historical developer-level delete. A refused delete leaves the row and every member control untouched (asserted by test across all three ACL dimensions).

## 6 — Established streams survive revocation (medium): fixed in place

Round-6/7/8 hardened every admission edge (fresh authorization before a channel opens, a shell upgrades, a ticket redeems), but an admitted stream then ran to its disconnect or the 4h cap — a revoked member kept shell access, transcript visibility, or agent steering for hours.

Fix: one shared watchdog, `sshgateway.WithRevalidation` (`revalidate.go`), re-runs the transport's authoritative check every `BEX_SSH_REVALIDATE_INTERVAL` (default 1m; negative disables) and cancels the stream context on failure:

- **Native SSH** — per-channel on the single-channel path; one transport-level watchdog on the multiplexed path (ends every live channel at once). The check is the same `reauthorize` round-8 added at channel-open. Audit result `revoked` distinguishes a watchdog end from a client close.
- **Web shell** — re-runs ticket-time target resolution (which asserts `can_operate` uncached) and cancels the exec context; the watchdog owns the exec context exclusively so a canceled context under a live request is unambiguously `revoked`.
- **Agent attach** — re-runs the redemption-time `RevalidateAttach` (round-6 #11's seam) for the life of the read/turn stream and aborts replay/splice on failure.

Tests: mid-stream revocation with no new channel/client input ends the active exec/stream and records `:revoked` on every transport, plus unit tests for the watchdog itself (cancel-on-fail, stay-alive-on-pass, disabled degrades to plain cancel).

## 7 — Stale authorization at sensitive sinks (medium): fixed in place

The 30s positive cache was still the final gate at five sinks round-8's `AuthorizeFresh*` sweep missed: env-group file reveal, `Query`/`ExecuteQuery` (arbitrary SQL), `ListExports` (presigns 15-minute full-dump bearer URLs), and `DeletePostgres`/`DeleteKeyValue`. Each now re-asserts its relation uncached immediately before the sink (`AuthorizeDatabaseFresh`/`AuthorizeKeyValueFresh`/`AuthorizeFreshOn`), following the round-8 `PostgresConnectionInfo` pattern. Tests extend each feature's `fresh_gate_test.go`: a stale positive is refused before the credential resolves, the URL signs, the SQL runs, or the CR delete — and the resource survives.

## 8 — Executable build inputs on a contributor relation (medium): fixed in place

`SetRootDir`/`SetDockerfilePath` gated on `can_operate` (contributor) although both select which repository bytes BuildKit executes. The m68 executable-selection audit had deliberately classified them as "build configuration" — round-9 reverses that carve-out: an API role and git access are independent axes (a workspace contributor may hold no write access to the bound repository at all), and every pre-existing subtree or Dockerfile in the repo is selectable, so repointing the build context/Dockerfile IS choosing what executes. Both verbs now require `can_create`; the m68 class table and `representativeVerbRelations` pins were updated with the reversal rationale so the boundary is re-documented, and the class tests now enforce contributor-refused/developer-allowed for both.

## 9 — metrics-server kubelet TLS (low): closed by `w2/m81` (2026-08-19)

Third report (round-7 #5 → ADR061 #11 → this), now closed: a digest-pinned kubelet-serving CSR approver (`deploy/gitops/base/kubelet-csr-approver.yaml`) plus `--kubelet-certificate-authority` on the base metrics-server Application replaced `--kubelet-insecure-tls`, which was dropped from the CAPD-local overlay once local kubelets (`rotate-server-certificates: "true"`, `infra/clusterapi/overlays/local-capd/cluster.yaml`) also presented approver-signed certs. Full disposition in `docs/ADR072-security-review-round7.md` #5; the one documented residual is already-running prod nodes, which keep self-signed certs until a kubelet restart or the next ADR053 template rotation (not forced by this milestone). Cilium WireGuard node encryption remains a compensating control for that residual window.

## 10 — DNS-name registry SSRF (low): accepted, unchanged

Repeat of the documented residual in `ValidImage` (round-7 #6, ADR061 #12): internal DNS names are lexically indistinguishable from public ones; the pull is blind, protocol-constrained, and carries no platform credential for external hosts. The allowlist/mirror remediation remains the standing direction if this is ever raised.

## 11 — Unbounded mobile SSE retention (low): fixed in place

React Native's XHR keeps every received byte in `responseText` for the request's lifetime, so a long-lived high-volume log stream grew the JS heap until the process died — the visible `LogBuffer` cap cannot release the transport's copy. Fixed in the mobile transport (`mobile/src/features/logs/`):

- A per-stream byte budget (32 MiB) recycles the connection when exceeded: the emitted network-class error rides the session's **existing** reconnect path, which resumes from the newest buffered timestamp on a fresh XHR (the old one becomes garbage) — the remediation the finding asked for, composed with the machinery that already existed.
- The SSE parser caps its pending frame at 1 MiB: an unterminated oversized frame is dropped with an error (and the parser reset) instead of buffering forever.

Tests: a tiny-budget transport aborts and errors past the budget and is terminal afterwards; the parser drops an oversized frame and parses the next one cleanly.

## 12 — Unverified privileged inputs (low): deferred, unchanged

Third report of the digest-pinning inventory (ADR055 F7 → ADR061 #1 → this): Argo install manifest, Helm charts, bootstrap checksums, and remaining mutable platform image tags (the KV exporter was already noted). The incremental pinning work continues on the standing register; nothing in this window changed its calculus.

## 13 — Admission grammar gaps (low): fixed in place (targeted half)

`bex-operator-workloads` admitted pod shapes the generators never emit: `nodeName`, `procMount: Unmasked`, wildcard/control-plane tolerations, added capabilities, privilege escalation, and non-RuntimeDefault seccomp outside the build boundary, and no user-namespace requirement inside it. Extended to exactly the fields every generator already sets, so legitimate flows are unchanged (inventory: app Deployments, cron pods, DB export/purge Jobs, KV StatefulSet/backup Jobs all carry `tenantSecCtx()`; build/publish/pre-deploy pods all carry `execution.HardenPod`):

- Everywhere: deny direct `nodeName`; deny `procMount: Unmasked`; deny tolerations with an empty key (`Exists` tolerates every taint) or a `node-role.kubernetes.io/control-plane|master` key.
- In execution-boundary (`untrusted`) namespaces: **require `hostUsers=false`** — the pod user namespace is what confines BuildKit's mount capabilities and unconfined seccomp off the host kernel, so even a compromised controller authoring a BuildKit-shaped Job cannot reach node root.
- Outside the execution boundary (tenant hosting + legacy bootstrap): every container must be restricted — no added capabilities, no privilege escalation, RuntimeDefault seccomp (container- or pod-level).

The one generator gap this exposed is fixed: `dbBackupPurgeJob`'s container now carries `tenantSecCtx()` (pinned by test). The remaining half of round-6 #4's milestone — a dedicated build identity and exact BuildKit-shape modeling inside `bex-build` — stays deferred as before.

## Verification

- Backend: `go build ./... && go test ./...` (lego/backend) — all packages green, including the new `reauth`, fresh-gate, limiter, and environment-delete tests.
- Operator: `go build ./... && go test ./...` (lego/operator) green; the purge-Job hardening test pins the generator.
- Agent driver: `npm run typecheck && npm test` — 31/31, including the three new encoded/binary leak-refusal tests.
- Mobile: `yarn test:unit` — 318/318, including the stream-budget and parser-cap tests.
- Docs: runbooks (ADR011/015/037), `CLAUDE.md` env table, and `.env.example` updated for the new gateway knobs; `.pm/FUTURE-MAYBE.md` w7/020 entry closed.
