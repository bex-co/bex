# w6 · m132 — REGRESSION of `w2/m39`: the SSH gateway never sends KEXINIT

**Worker:** worker6 **Goal:** `ssh <service-id>@ssh.bex.co` completes a handshake and opens a shell again, and a dead SSH edge cannot go unnoticed for weeks. **Status:** todo

## Tasks (in order)

| id   | title                                                                              | est | depends_on |
| ---- | ---------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Reproduce and locate why the gateway writes its banner and never sends KEXINIT       | 50m | —          |
| t002 | Walk `w2/m39`'s definition of done item by item against production                    | 40m | t001       |
| t003 | Decide the honest-failure behaviour for a **pre-authentication** refusal              | 30m | t001       |
| t004 | Wire a guard so a dead SSH edge is loud (coordinate with `w6/m131/t004`)              | 40m | t001       |
| t005 | Render parity sweep (REST/GraphQL/MCP/dashboard + official Render CLI)                | 30m | t002, t003 |
| t006 | Simplify                                                                             | 20m | t005       |
| t007 | Test coverage                                                                        | 30m | t005       |
| t008 | Closeout                                                                             | 10m | t004, t007 |

## Background — found live, 2026-08-28, 71st `/qa-find-bugs` run, journey 8

**This is a regression, and the prior verification is on the record.** `docs/ADR035-ssh.md:3` states that running-instance SSH was "implemented behind production activation and **live verification (2026-07-14)**", and `:157` records that the CLI three-selector supplement "**passed against production on 2026-07-18**". It worked six weeks ago.

### The gateway completes the version exchange and then never advances the protocol

Raw-socket probe (Python, no OpenSSH involved):

```
connect ssh.bex.co:22
<- "SSH-2.0-bex"                 server banner arrives
-> "SSH-2.0-OpenSSH_9.6\r\n"     client version sent
<- (nothing for 6s)              NO SSH_MSG_KEXINIT (type 20) EVER ARRIVES
```

**Controls — two independent known-good servers, same probe, same run:**

| host          | banner                | bytes after version exchange | first msg type |
| ------------- | --------------------- | ---------------------------- | -------------- |
| `github.com`  | `SSH-2.0-20b2056`     | 800                          | **20 (KEXINIT)** ✅ |
| `gitlab.com`  | `SSH-2.0-GitLab-SSHD` | 800                          | **20 (KEXINIT)** ✅ |
| `ssh.bex.co`  | `SSH-2.0-bex`         | **0**                        | — ❌            |

A correct server sends KEXINIT immediately after the version exchange (RFC 4253); bex does not. The instrument is sound.

**Consistent, not transient:** 5 consecutive trials → `kexinit_received=0, no_kexinit=5`.

**Not target-dependent** — three targets, `peer KEXINIT received: 0` for all three:

- `srv-d9bkcspg9s7c73d0n8ug@ssh.bex.co` — `agentmarketcap-1`, plan `starter`, **paid and SSH-eligible**
- `srv-da8ffm7m2e9c73ft6t9g@ssh.bex.co` — own free fixture, ineligible
- `nonexistent-srv-zzz@ssh.bex.co` — no such service

So it is neither the eligibility gate nor per-service state.

**Not algorithm negotiation** — ruled out explicitly. With OpenSSH_10.2p1 the connection closes right after `SSH2_MSG_KEXINIT sent`, and `ssh -vvv` never logs a `peer server KEXINIT proposal` (grep count 0). Forcing conservative sets changes nothing:

```
default                                                          -> Connection closed by 49.12.20.236 port 22
-o KexAlgorithms=curve25519-sha256 -o Ciphers=aes128-ctr ...     -> Connection closed
-o KexAlgorithms=diffie-hellman-group14-sha256                   -> Connection closed
-o KexAlgorithms=ecdh-sha2-nistp256                              -> Connection closed
```

A server that had received the client's KEXINIT and found no common algorithm would send its own KEXINIT first, or a DISCONNECT. This one sends neither. The OpenSSH-visible close is most likely `BEX_SSH_HANDSHAKE_TIMEOUT` (10s) expiring — the raw probe waits only 6s and sees a silent socket rather than a FIN.

**Reachability is fine and correctly scoped:**

```
dig +short ssh.bex.co          -> 49.12.20.236
curl telnet://ssh.bex.co:22    -> SSH-2.0-bex        (gateway answering)
curl telnet://ssh.bex.co:2222  -> connection refused (internal port correctly not public)
```

### What the product promises while this is true

- `serviceDetails.sshAddress` is advertised on REST **and** GraphQL for every eligible paid service: `GET /v1/services/srv-d9bkcspg9s7c73d0n8ug` → `"sshAddress":"srv-d9bkcspg9s7c73d0n8ug@ssh.bex.co"`, and `{ service(id:…){ sshAddress } }` returns the same string.
- The dashboard Shell page reads "Open a shell on a running service instance from your terminal" and links "Manage SSH public keys".
- `ADR035` and `docs/ADR018-render-parity.md` both document native SSH as shipped.

### The guard that exists and does not run

`scripts/ssh-verify.sh` (17,976 bytes) describes itself as "Redacting public-edge acceptance for `docs/ADR035-ssh.md`", and `ADR035:157` documents the full matrix it runs — host-key pinning, PTY resize, any-instance and specific-instance raw SSH, exit status, stale-instance denial, suspended/free/shell-less behaviour, unknown/deleted keys. It is not wired to anything that runs on a schedule, so a total loss of the SSH handshake went unnoticed.

**This is the second instance of one institutional gap.** `w6/m131/t004` is the identical problem for `scripts/logs-verify.sh`: two production acceptance scripts, each written as the gate for its feature, neither running. Solve it once — `t004` cross-references m131.

## Definition of done

- A raw TCP connection to `ssh.bex.co:22` that completes the version exchange receives an `SSH_MSG_KEXINIT` (packet type 20), exactly as `github.com` and `gitlab.com` do under the identical probe. Today bex sends **0** bytes after the banner while both controls send 800.
- `ssh <service-id>@ssh.bex.co` with a registered key opens a shell on an eligible paid running service, and `ssh -vv` logs a `peer server KEXINIT proposal`. Today the handshake never reaches authentication for **any** target — paid-eligible, free-ineligible, or nonexistent alike.
- `scripts/ssh-verify.sh` has been run against production with its output recorded, and `w2/m39`'s definition of done has been walked item by item, each guarantee marked live-verified or still owed.
- A pre-authentication refusal gives the client something better than a silent close, per `t003`'s decision — **without** weakening `ADR035:106`'s deliberate non-disclosure of _authentication_ causes.
- Something that runs on its own would now fail while the handshake is dead.
- The plan-gate path still behaves as it does today (verified live this run, must not regress): a free service's `POST /v1/services/{id}/shell-ticket` returns 409, its `serviceDetails.sshAddress` is absent, and the dashboard Shell tab explains that shell access requires a paid service.

## Triage re-verification (2026-08-27, this session) — still broken, reproduced from outside the cluster

Re-probed `ssh.bex.co:22` with a raw socket (no OpenSSH client involved, so no client-specific quirk is in play):

```
BANNER: b'SSH-2.0-bex\r\n'          <- server identification string arrives normally
-> client sends "SSH-2.0-probe\r\n"
AFTER-CLIENT-BANNER: 0 bytes, EOF   <- no KEXINIT; the server closes the connection
```

The defect is confirmed live and unchanged: the gateway completes the identification exchange and then hangs up instead of sending its KEXINIT, so no client can negotiate a key exchange. This milestone is real and is the most severe open item on the board — a shipped, advertised surface that is completely dead.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt, 71st run, 2026-08-28, journey 8 (Shell / SSH). Workspace `tea-d98210cbbpdc73dcrkvg`. Fixture `qa-20260828-shell` (`srv-da8ffm7m2e9c73ft6t9g`, free web service) created, probed and **deleted** in the same visit (`deleteService: true`, `GET` → 404). `agentmarketcap-1` was used read-only as the paid control — only SSH handshakes were attempted against it; no session was ever established and nothing ran in any container. Every probe above is a complete command + response so it can be re-run.
- **Goal linkage:** `docs/ADR035-ssh.md` (the SSH design and its acceptance matrix) and `docs/ADR018-render-parity.md`'s SSH row; `ADR006` for the `sshAddress` contract across REST/GraphQL/MCP.
- **Regression lineage:** `w2/m39` ("SSH into running service instances", done 2026-07-17) is the milestone whose guarantee is broken. `w2/m55` (Browser Web Shell) and `w2/m65` (Open in Zed, whose `ags-…` targets resolve through this same gateway) both depend on it and may be affected — **neither was tested**.
- **Expected outcome:** `ssh <service-id>@ssh.bex.co` works again, and a dead SSH edge cannot go unnoticed for weeks.
- **Why now:** the product advertises an SSH address on every eligible paid service across REST, GraphQL and the dashboard, and that address currently cannot complete a handshake. It last passed production acceptance on 2026-07-18.
- **Render parity:** the standing task is **included** — `sshAddress` is a Render-compatible field and the official Render CLI drives this exact path (`ADR035:157` records driving the real v2.21.0 binary against production).
- **Boundary note (`DO_NOT_DO.md` item 12 — product code vs platform GitOps responsibilities):** if `t001` finds the fix belongs to the load balancer or GitOps, it stays on that side; `t001` locates and records it.
- **Blast radius:** everything reaching the gateway over TCP/22 — native `ssh` to `srv-…` App targets, the official Render CLI's shell path, and `ADR054`/`w2/m65`'s `ags-…@ssh.bex.co` agent-session targets with their multi-channel + sftp-subsystem exception. The Browser Web Shell (`w2/m55`) and `sandbox exec` use **other** gateway transports (wss shell port, sandbox-exec port) and may be unaffected — untested.
- **Adjacent classes:** unauthenticated (no key registered) must stay non-disclosive per `ADR035:106`; free/ineligible must keep its current 409 + absent `sshAddress` + dashboard explanation; a nonexistent service must remain indistinguishable from unauthorized at the auth stage; and an **infrastructure fault** — the case here — must be distinguishable from all of them, since it is not a security boundary.

## Unverified this run — carried onto the board, not presented as observed

- **The root cause.** No cluster access; only wire behaviour was observed. The PROXY-protocol hypothesis below is the leading candidate but is entirely unmeasured — and a grep of `deploy/` and `infra/` for PROXY-protocol configuration returned **nothing**, so even its configuration point is unlocated.
- The **Browser Web Shell** (`w2/m55`) was not tested. Exercising it needs a paid eligible service; the only paid service available is a customer's production workload, and opening a shell there would execute in their container. The free fixture is correctly plan-gated, so it cannot reach the path.
- `sandbox exec` and the `ags-…` agent-session SSH targets were not tested.
- **No SSH key was registered**, so all authentication behaviour past KEXINIT is unobserved — including whether `w2/m66`'s `RequiresSshKey` gate still behaves.
- One client only (OpenSSH_10.2p1 / LibreSSL 3.3.6 on macOS) plus the raw Python socket. The raw probe makes a client-specific quirk very unlikely — the failure is protocol-level, not implementation-level — but no second OpenSSH version was tried.
- Whether this affects the `ssh.bex.co` DNS/LB path only or the gateway pod directly was not separable from outside the cluster.

## Verified working this run — recorded so the fix does not break it

The free-plan gate is correct end to end and is **not** a defect: `sshEligible` (`lego/backend/internal/apps/service.go:982-995`) refuses free plans by design ("Render only offers SSH on paid services"), `sshSessionReady` (`:998-1005`) adds the runtime gate, `POST /shell-ticket` returns `409 conflict: service is not eligible and running`, `sshAddress` is correctly absent from `serviceDetails`, and the dashboard renders "Shell access isn't available — Shell access requires a running paid web, private, or background service and an active SSH gateway" with a Manage SSH public keys link. That is plan-gating working as intended.
