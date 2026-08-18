# w5 · m72 — Agent-session production recovery: stream grants, large-repo scrub, and terminal convergence

**Worker:** worker5 **Goal:** restore trustworthy agent-session conversation replay and make every agent turn converge to a durable terminal state even when credential cleanup, sandbox termination, or Git publication is ambiguous **Status:** done (2026-08-18; reopened delivery edge corrected and verified on `bex-co/bex`)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Restore the production gateway replay privilege — **DONE** | 45m | — |
| t002 | Make gateway grants deploy-convergent and self-checking — **DONE** | 60m | t001 |
| t003 | Make credential scrubbing safe for large repositories — **DONE** | 60m | — |
| t004 | Preserve a readable failed turn when cleanup fails — **DONE** | 45m | t003 |
| t005 | Project terminal child-Pod state through OpenSandbox — **DONE** | 60m | — |
| t006 | Add a terminal-state fallback in gateway + Completer — **DONE** | 60m | t005 |
| t007 | Deploy, repair the stranded session, and run the production workflow — **DONE** | 60m | t002, t004, t006 |
| t008 | Render parity — **DONE** | 30m | t007 |
| t009 | Simplify — **DONE** | 30m | t008 |
| t010 | Test coverage — **DONE** | 60m | t008, t009 |
| t011 | Closeout — **DONE** | 20m | t008, t010 |
| t012 | Make final Git publication idempotent and protocol-correct — **DONE** | 60m | t011 |
| t013 | Recheck user-surface parity and amend ADR047 — **DONE** | 30m | t012 |
| t014 | Simplify the delivery recovery patch — **DONE** | 30m | t013 |
| t015 | Add and run delivery regression coverage — **DONE** | 45m | t013, t014 |
| t016 | Deploy and verify the corrected `bex-co/bex` workflow — **DONE** | 60m | t015 |
| t017 | Recovery closeout — **DONE** | 20m | t016 |

## Definition of done

1. [x] The production `bex_ssh_gateway` role can read `agent_session_turns`; an authenticated `GET /v1/agent-sessions/{id}/stream` replays durable user turns and assistant parts without SQLSTATE `42501`. Every deploy applies the current least-privilege grant set after migrations without rotating the gateway password or Secret, and a missing required privilege fails a visible preflight rather than a user's stream.
2. [x] A legitimate repository larger than 1 GiB can complete the persisted-credential cleanup under bounded CPU, memory, file-size, and time controls. Injected credential needles are still removed from every writable persistence root, including security-relevant Git metadata/object paths; the fix does not merely raise or disable the safety limit.
3. [x] Any scrub, ACP child, or top-level turn failure produces one durable/readable failed status, never delivers or pushes unsafe output, forgets in-memory credentials in `finally`, and cannot be converted into an unhandled duplicate-cleanup process exit before the control plane observes it.
4. [x] A terminal sandbox child Pod reaches an OpenSandbox terminal state. Independently, the gateway/control plane recognizes a failed or terminated Pod/container as terminal, so the Completer advances the session out of `creating`/`running` exactly once even if BatchSandbox status is stale; it does not poll `pods/exec` forever or flood `container not found` logs.
5. [x] The reported session `ags-da1prbt040bc73aj5230` no longer appears live, its dead sandbox residue is reclaimed, and a fresh run against `bex-co/beancount-cms-v2` reaches an honest terminal result with refreshable conversation history and no missing delivery hidden behind a green state.
6. [x] REST, GraphQL, MCP, dashboard, gateway, and operational metrics expose consistent terminal/error semantics. Focused DB-role, driver, controller, sandbox, Completer, attach, dashboard, and live workflow regressions pass; the normal backend, agent-image, dashboard, lint, and GitOps validation gates are green.
7. [x] Git delivery treats the exact scanned candidate already present at the remote ref as idempotent success, rejects a different concurrent ref, and resolves an ambiguous push error only by freshly proving the exact candidate is published. A new retry never displays the previous terminal `failureReason` as its current state.
8. [x] The corrected image is deployed and a fresh `bex-co/bex` agent workflow reaches a truthful terminal result with its draft PR and refreshable transcript; no final-push 403 is hidden by `Everything up-to-date`.

Completion evidence: [`evidence/2026-08-18-production-recovery.md`](evidence/2026-08-18-production-recovery.md) and [`evidence/2026-08-18-git-delivery-recovery.md`](evidence/2026-08-18-git-delivery-recovery.md).

## Source + Goal linkage

- **Source:** user production report on 2026-08-17 for `https://dashboard.bex.co/agents/ags-da1prbt040bc73aj5230` (“The conversation stream is unavailable right now”), followed by a read-only end-to-end incident investigation and the explicit request to hand the complete fix to w5. This milestone absorbs `w3/012`'s already-recorded crash-path stranding fault. It also reopens the deploy-convergent `dbrole.sql` work removed from `w2/019`: that risk is no longer hypothetical because migration 0081 added `agent_session_turns` and production retained the old role grants.
- **Observed production failure:** the stream first dies in the gateway because `bex_ssh_gateway` lacks `SELECT` on `agent_session_turns`. Independently, this 1.57 GiB repository exceeds the driver's 1 GiB persisted-credential scan budget; the catch path invokes the same failing scrub again and exits the driver. The child Pod is `Failed`/exit 1 while BatchSandbox remains `Pending`, so two API replicas repeatedly exec a nonexistent `sandbox` container and leave the database row `running`.
- **Reopen evidence (2026-08-18):** after the original closeout, `ags-da2daqui706c739sqvtg` on `bex-co/bex` ended its first turn with `git push` HTTP 403 plus `Everything up-to-date`. Credential mint and discovery had succeeded; the receive-pack/publication result was not modeled idempotently, and a retry retained the old `failureReason` while already running. A second turn later delivered `e010a3c5…` and draft PR #57, proving the broad “Git delivery is fixed” conclusion was unsupported by the earlier one-repository proof.
- **Goal linkage:** ADR008's AI-native platform pillar requires agent work to be durable, reconnectable, and operationally honest. ADR042, ADR047 D3/D9, ADR051, ADR059, and ADR065 promise a disposable sandbox with durable control-plane state; this incident violates that promise at the database privilege, driver cleanup, substrate projection, and lifecycle convergence seams.
- **Expected outcome:** conversation replay works after every schema rollout; large repositories do not crash solely because of aggregate scan size; every fatal sandbox path becomes a bounded, visible terminal session; and the exact production workflow that failed is verified end to end.
- **Why now:** durable replay is currently broken globally for the new turn table, the reported session is stranded in `running`, and production is emitting repeated failed execs every 15 seconds. Shipping additional agent-session features on top would compound data loss and operational noise.
- **Render parity task included:** the fix changes user-visible session status, error, and dashboard conversation behavior. Render has no equivalent AI-session product, so the parity pass must document that deliberate extension while enforcing identical semantics across Bex REST, GraphQL, MCP, and dashboard surfaces and comparing the closest durable operation/event-history behavior.
- **Out of scope:** changing model providers or ACP protocol, making credential scanning unbounded or fail-open, reconstructing already-lost assistant output, redesigning general sandbox quotas, or expanding the gateway's least-privilege database surface beyond methods it actually calls.
