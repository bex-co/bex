# w5 · m83 — Agent-session Git proxy: loud, sized response budget (549 MiB pack truncated at 512 MiB)

**Worker:** worker5 **Goal:** clones of large repos stream through the gateway Git proxy intact; a genuinely over-budget or broken stream fails loudly instead of masquerading as success. **Status:** done (2026-08-30; shipped `b9d7336b`, pinned `a0d77144` → digest `323fdfe3…`, live E2E passed)

## Tasks (in order)

| id   | title                                                                | est | depends_on           |
| ---- | -------------------------------------------------------------------- | --- | -------------------- |
| t001 | Configurable response budget + loud abort/log/metric on cap & stream errors — **DONE** | 45m | —                    |
| t002 | Live E2E: fresh production session on `bex-co/web-beancount` completes — **DONE** | 30m | t001 (shipped image) |
| t003 | Closeout — **DONE**                                                               | 15m | t002                 |

## Definition of done

A fresh production agent session on `bex-co/web-beancount` (549 MiB clone pack — the repo whose session `ags-da9mh5vj596c73en5eq0` failed every clone attempt) clones through `bex-ssh-gateway:8082` without truncation and completes its turn. Unit tests pin: over-budget response → aborted + logged + `response_cap` metric + **no** `accepted`/allowed bookkeeping; broken upstream stream → aborted + logged + `stream` metric. Backend suite + lint green.

## Source + Goal linkage

- **Source:** user goal 2026-08-30 ("ensure agent sessions work in production, fix errors from ags-da9mh5vj596c73en5eq0"). Diagnosis: `maxResponseBodyBytes = 512 MiB` (hardening commit `c386eea4`, 2026-08-19) truncated the 549 MiB clone pack via `io.LimitReader` clean-EOF; `io.Copy`'s ignored error meant the exchange still counted `accepted` and audited `allowed`, while git reported only "unexpected disconnect while reading sideband packet" — every attempt, old code and (pre-this-fix) new code alike, ~30–36 s in at wire speed. Direct sibling of w5/m82 (same proxy, different silent-failure shape).
- **Goal linkage:** pillar 5 agent-session reliability; ADR047 D2.
- **Expected outcome:** the two on-GitHub repo profiles that broke the proxy (many-ref → gzip, large-pack → cap) both clone; any remaining upstream failure is one `kubectl logs` grep away.
- **Why now:** every session on `bex-co/web-beancount` fails deterministically today; the false `allowed` audit actively misleads diagnosis.
- **Render parity:** omitted — internal gateway mechanism, no API/UI surface.

## t002 live E2E evidence (2026-08-30)

- Shipped `b9d7336b`; pin `a0d77144` (`9ba80e4f20be`); prod `bex-ssh-gateway` on digest `323fdfe3…` verified before the run.
- Fresh production session **`ags-da9pa5a3el9c73c9jdq0`**, repo `bex-co/web-beancount` (549 MiB clone pack — measured locally with `git clone --bare`), branch `bex-agent/m83-response-budget-e2e2`: **completed** in ~100 s. Audit trail shows ONE clone attempt with the pack-fetch stream running **38 s to genuine completion** (mint 02:27:41 → `ProxyCredential allowed` 02:28:18) — previously it silently cut at 512 MiB ~30–36 s in on every attempt. Gateway logs: zero `response_cap`/`stream` failures. Only the familiar benign startup-race mint denial (retried +3.5 s, passed).
- First E2E attempt (`ags-da9p720k98cs738k20c0`) was a rollout-window casualty, not this bug: three bex-api ReplicaSets churned within ~10 min of the Argo sync, and the replica handling the create died between the OpenSandbox create and the dispatch-record write, leaving `sandbox_id` empty → every mint denied. Canceled; the non-crash-safe dispatch window is filed as `w5/050.md`.
- Caller: temporary Hydra API-key client `m83-e2e-verify`, developer-bound to the workspace, fully revoked after the run (FGA tuple, `tenant_members` row, Hydra client).
