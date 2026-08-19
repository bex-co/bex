# w10 · m9 — Cross-stream note burn-down round 3: revocation watchdog, typed sandbox errors, attach-URL shape, admission-policy note

**Worker:** worker10 **Goal:** the four open code-shaped cross-stream notes not claimed by `w2/020`'s live-verification charter are closed with evidence — a live-log tail can no longer outlive revocation, sandbox quota denials return typed errors, the attach-ticket stream URL shape is decided and shipped, and the round-6 #4 admission-policy slice has a scoping note with a go/no-go **Status:** done

**Resolution (2026-08-19):** between this milestone's scoping (2026-08-18) and implementation start, an unrelated w6 session picked `w4/034` and `w3/011` directly from their inboxes and shipped both in full — t001 and t002 required **no new code**; verified against current `main` (`internal/logs/service.go`'s revalidation watchdog + six pinning tests; `internal/sandbox/service.go`'s `SANDBOX_CAPACITY_LIMIT` coded error) and closed as duplicates per `DO_NOT_DO`'s anti-duplication rule rather than re-implemented. t003 shipped the `streamUrl` field end to end (REST/GraphQL/MCP via one `View` struct + `agentSessionGQLType` wiring, dashboard consumer types corrected, `agent-session-verify.sh` cross-check) — genuinely new work, `w3/013` was still open. t004's design note found ADR057 #4's broad ask already shipped (the identity-scoped VAP `operator-workload-admission.yaml` predates this milestone) and scoped two narrow residuals (day-to-day identity VAP coverage, exact BuildKit-shape modeling) as `w2/022`, explicitly **not** a milestone. All four source notes are in their home workstreams' `done/`. `go test ./...` (all backend packages) + `yarn typecheck && yarn lint && yarn test` (344 files / 2330 tests) green.

## Tasks (in order)

| id   | title                                                                                     | est | depends_on             |
| ---- | ----------------------------------------------------------------------------------------- | --- | ---------------------- | --- |
| t001 | `w4/034` — fresh-authorization watchdog for established live-log SSE tails                | 45m | —                      | — **DONE** |
| t002 | `w3/011` — typed quota error (not opaque 502) on quota-denied sandbox creation            | 45m | —                      | — **DONE** |
| t003 | `w3/013` — attach-ticket stream URL shape: decide + ship explicit `streamUrl`             | 30m | —                      | — **DONE** |
| t004 | `w2/021` — design note scoping the ADR057 round-6 #4 ValidatingAdmissionPolicy slice      | 45m | —                      | — **DONE** |
| t005 | Render parity — error/response-shape consistency across REST/GraphQL/MCP                  | 20m | t001, t002, t003, t004 | — **DONE** |
| t006 | Simplify — /simplify over the changed code                                                 | 20m | t005                   | — **DONE** |
| t007 | Test coverage — revocation-ends-tail, typed quota error, stream-URL shape                 | 30m | t005                   | — **DONE** |
| t008 | Closeout                                                                                   | 15m | t007                   | — **DONE** |

## Definition of done

Revoking `can_view_logs` mid-tail ends the established `GET /v1/logs/subscribe` SSE stream within the revalidation interval (test-proven, not just admission-time); a quota-denied sandbox create returns a typed 4xx quota error across REST and MCP — never a 502 — and the client/server timeout inversion (`internal/sandbox/client.go` 30s client under the server's 60s wait) is fixed; the agent-session attach-ticket mint response carries a correct, documented stream URL shape (explicit `streamUrl` or a documented decision not to); the admission-policy scoping note exists with an explicit go/no-go; all four source notes (`w4/034`, `w3/011`, `w3/013`, `w2/021`) are moved to their home workstreams' `done/` folders; `make lint` + backend tests green.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` 2026-08-18 inbox survey (31 open notes triaged across w1–w11). These four are the code-shaped burn-down candidates **not** claimed by `w2/020`'s live-verification charter — this milestone coordinates with, never duplicates, that note (the m4/m6 precedent). Each source note moves to its home workstream's `done/` on completion.
- **Goal linkage:** t001 closes a real security-review residual (a live-log tail outliving revocation — re-owned from register `w1/046` F18, the last gap in the round-9 `WithRevalidation` watchdog family); t002/t003 are API-correctness fixes on pillar-5 surfaces; t004 unblocks the standing ADR057 round-6 #4 deferral with a concrete go/no-go.
- **Expected outcome:** four cross-stream leftovers that no other queue was going to reach are closed with evidence, and their notes leave the open board.
- **Why now:** w10's spare-capacity charter is exactly this pattern; t003 just lost its nominal owner when `w1/m64` closed (`ce37c0a5`, 2026-08-18) without shipping it; t001 is a live security gap today.
- **Render parity note:** included (t005) — t002/t003 change REST/MCP error and response shapes, which must stay consistent across surfaces and Render's error dialect; t001's stream-termination behavior is checked for surface-consistent audit/error semantics.
