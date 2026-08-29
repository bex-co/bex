# w1 · m136 — A session that failed for a missing model key says so

**Worker:** worker1 **Goal:** a session whose workspace has no BYO model key terminalizes in seconds with the actionable reason the code already defines, instead of riding the agent CLI's retry loop into a raw JSON-RPC string **Status:** done

## Tasks (in order)

| id   | title                                                                     | est | depends_on |
| ---- | ------------------------------------------------------------------------- | --- | ---------- |
| t001 | Give the minter a distinct "no model key provisioned" error — **DONE** | 30m | —          |
| t002 | Report the auth failure when the mint is refused for a missing key — **DONE** | 45m | t001       |
| t003 | Render parity — the terminal `failureReason` across REST/GraphQL/MCP/UI — **DONE** | 30m | t002       |
| t004 | Simplify the code this milestone changed — **DONE** | 30m | t003       |
| t005 | Test coverage for the missing-key terminalization — **DONE** | 40m | t003       |
| t006 | Closeout — **DONE** | 15m | t005       |

## Definition of done

A session created in a workspace with **no** `agent-sessions/<ws>/model-key` in OpenBao ends as:

- `phase = failed` with `failure_reason` **equal to** `agentsession.ModelAuthFailureReason` — reworded by t003 to name both causes: "this workspace's model API key is missing or was rejected by the model provider (authentication failed); set a valid model key and start a new session",
- terminalized on the **first** refused mint, not after the agent CLI exhausts its own retry/backoff — the ~13 `agent-session model credential mint denied` lines over ~5s collapse to one **of that class** (see Outcome: post-terminal authorization refusals from in-flight sandbox requests are a separate, correct class),
- with no raw JSON-RPC text (`code -32603`, `API Error: 403 forbidden`) reaching `failure_reason` on this path.

A mint refused because the caller is **not** the session's current sandbox (a stale or cross sandbox in the ADR054 grace window) still gets a bare 403 and does **not** terminalize the session — asserted separately, so the fix cannot become a way for a stale pod to kill a live session.

## Source + Goal linkage

- **Source:** live bug hunt in `dev-1`, 2026-08-29 (session `ags-da9674i9086hnle27720`). With no key provisioned, `ModelMinter.Mint` takes its "no key" branch (`lego/backend/internal/agentsession/model.go:294-299`) and returns `ErrForbidden`; `modelproxy.go:162-171` maps **any** `ErrForbidden` from the mint to a bare 403 and — unlike the vendor-rejection path 40 lines below it (`modelproxy.go:200-210`) — never calls `reportAuthFailure`. Observed terminal reason: `Internal error: Failed to authenticate. API Error: 403 forbidden\n (code -32603)`.
- **Goal linkage:** pillar 5 (ADR008), the ADR047 D9 session surface. It closes a gap in the mechanism `w5/m80` t003 built — that milestone added the gateway→bex-api auth-failure report precisely so a bad key terminalizes "in seconds rather than the agent CLI riding its full retry/backoff (~3min observed)"; the **missing**-key case has the identical user-visible consequence and was left on the un-reported path.
- **Expected outcome:** the most likely first-run failure for a new workspace names its own fix. Today it reads as an internal crash.
- **Why now:** repo-less chat is the zero-config first session a new tenant runs (`validateCreate` needs no GitHub App), and the BYO model key is the one thing it *does* require — so this is the single most probable failure a tenant meets first. It is also cheap: `ModelAuthFailer.Fail` already re-checks `currentSandboxCaller` (`model.go:357-362`), so the reporting verb is already safe to call here; the work is distinguishing the cause, not building a new path.
- **Relationship to `w4/m89` (not a duplicate):** m89 t004 makes the composer refuse **before** submit using the existing `capabilities.modelKeyReady` projection, and m89 t001/t002 re-code `ErrAgentSessionsUnavailable` on the create/read path. Neither touches the **terminal `failureReason`** written by the sandbox→gateway→bex-api mint path, which is the only thing this milestone changes. m89 prevents the session; m136 makes the session that still happens (a key deleted mid-flight, a pre-flight bypassed by REST/MCP callers) explain itself.
- **Render parity:** **included.** Render ships no coding-agent product, so per `docs/ADR018-render-parity.md` parity here is bex's own cross-surface discipline: `failureReason` is already exposed on REST, GraphQL and MCP as well as the dashboard failure card, so t003's job is to confirm one reason string reaches all four identically and that no new field is invented.

## Outcome

Shipped in `lego/backend` only — no dashboard or schema change, so no new field
reached REST/GraphQL/MCP and the failure card renders the new string verbatim.

**The wire problem the plan missed.** `ErrModelKeyMissing` wrapping `ErrForbidden`
is enough inside bex-api, but the refusal crosses an HTTP hop: `postSignedMint`
(`http.go:150-155`) rebuilds a bare `ErrForbidden` from the 403 status, so the
sentinel never reached the gateway and the first live run still produced the old
behavior. Fixed with a **response header** (`RefusalHeader` /
`RefusalModelKeyMissing`) rather than a field on the HMAC-signed request envelope,
which is shared with the mint verb and would have been a wire break between two
separately deployed images. It degrades cleanly both directions: an old gateway
ignores the header, a new gateway against an old bex-api never sees it. It is not
a trust boundary — it unlocks only the *report*, which `ModelAuthFailer.Fail`
re-authorizes from scratch.

**t003 wording decision: one constant, reworded.** Splitting into two reasons would
have meant carrying a cause through that same signed envelope for a distinction
that changes nothing the tenant does — the remedy is identical. Instead
`ModelAuthFailureReason` now names both causes:
`"this workspace's model API key is missing or was rejected by the model provider (authentication failed); set a valid model key and start a new session"`.
The old text asserted the provider "rejected this workspace's API key", which is
false when no key was ever sent.

**Live evidence (dev-1, 2026-08-29).**

| case | session | result |
| --- | --- | --- |
| no key provisioned | `ags-da9j4f29086o67dn7o80` | `failed` with the new reason |
| key provisioned | `ags-da9j4p29086o67dn7ogg` | `completed`, 1 turn — no regression |

Refusal counts for the keyless run, measured from `bex-api.log`: **1** missing-key
refusal (was 13) and 12 plain authorization refusals. Read that second number
carefully — it is not leftover retry. Terminalization now succeeds on the first
refusal, so the sandbox's already-in-flight requests arrive at a session that is
already `failed` and are refused by `currentSandboxCaller`, which is the designed
behavior. The total happens to still be 13; the number that matters, refusals of
the retried missing-key class, went 13 → 1.

**Found while verifying (filed as `w1/081`, not fixed here):**
`scripts/dev-env.sh:908` guards the `bex-lego:dev` build with
`docker image inspect … || build`, so `agent-up` silently reuses a stale image and
a gateway code change never reaches the cluster. That is what made the first live
run reproduce the old behavior with the fix already written.
