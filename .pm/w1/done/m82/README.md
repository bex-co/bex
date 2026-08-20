# w1 · m82 — Invite-of-existing-member must not silently overwrite a role

**Worker:** worker1 **Goal:** "inviting" someone who already belongs to the workspace can no longer silently overwrite (and possibly downgrade) their role — closing the High-severity member-invite bug found in the 2026-08-19 dashboard QA walk, plus the two minor invite-dialog gaps beside it. **Status:** done

## Tasks (in order)

| id   | title                                                                                        | est | depends_on   |
| ---- | -------------------------------------------------------------------------------------------- | --- | ------------ |
| t001 | Reject an invite whose email is already an active member; no role overwrite — **DONE**        | 1h  | —            |
| t002 | Invite dialog: client-side email validation + surface the server's reason — **DONE**          | 45m | —            |
| t003 | Render parity: invite verb + error shape across REST/GraphQL/MCP/UI — **DONE**                | 30m | [t001, t002] |
| t004 | Simplify the touched invite/redeem + dialog code — **DONE**                                   | 30m | t003         |
| t005 | Test coverage for the fixed behaviors — **DONE**                                               | 1h  | t003         |
| t006 | Closeout — **DONE**                                                                            | 10m | t005         |

## Definition of done

- Inviting an email that already belongs to an **active member** no longer changes their role: refused with a coded conflict (`MEMBER_ALREADY_EXISTS`, HTTP 409) identical across REST/GraphQL/MCP and surfaced inline by the dashboard. **Met.**
- The redeem path cannot silently overwrite/downgrade an established member's role even with a stale pending invite. **Met** — and it is the structural half, holding regardless of deployment config.
- The invite dialog rejects a malformed email before submit and shows the server's **specific** reason on rejection. **Met.**
- Regression tests fail on the pre-fix code. **Met** — proven red for all three (see Outcome).

## Outcome (2026-08-19)

Implemented in one session. **Backend suite green on a pristine Postgres (`go test ./...` exit 0), backend lint 0 issues, dashboard suite green (2357 tests), typecheck + lint clean; uncommitted pending `/ship`.**

**Root cause had two halves, and the deeper one was deliberate-turned-dangerous.** `members.Invite` never checked whether the address already belonged to a member, so it minted a pending invite; the auth gate then redeemed it by email match on the victim's next request, where `store.redeemInvite` ran `INSERT … ON CONFLICT (tenant_id, subject) DO UPDATE SET role = EXCLUDED.role`. That upsert was **intentional** — three comments described it as letting an invite "UPGRADE an existing membership's role" — but the identical write silently **downgrades**, which is how re-inviting a live admin at the dialog's default role demoted them.

- **t001 (backend, both halves).** `Invite` now refuses an existing member with `core.NewConflictError(MEMBER_ALREADY_EXISTS, …)` before the plan/seat guards, so the caller gets the real reason (Render's "already a member"). The address→member lookup goes through the same `Identities` seam `List` uses; with no identity reader (`BEX_KRATOS_ADMIN_URL` unset) it degrades to "not known to be a member" rather than failing closed. `redeemInvite` is the structural guard: `DO UPDATE SET role = tenant_members.role RETURNING role` keeps the existing role while still returning it, and — importantly — both acceptance paths now report that **effective** role. That detail was load-bearing: the callers write the OpenFGA tuple from the returned `inv.Role`, so skipping the DB write while returning the invited role would have granted the invited role in authz while the row said otherwise. Row and tuple can no longer disagree.
- **t002 (dashboard).** New shared `isValidEmail` gates Send (an address with no `@` no longer arms it) with an inline hint, and the hook surfaces the server's own reason inline via `refusal`/`clearRefusal` instead of the generic "Couldn't invite {email}" toast. **Correction to the original report:** the toast was never actually missing — the QA walk's "silent failure" reading came from an intermittent `ERR_CONNECTION_CLOSED` (filed separately as `w1/071`); the real gaps were the missing validation and the generic wording.
- **t003 (parity).** All three surfaces call the one `Service.Invite`, and `core.ErrConflict` maps to 409 in `core/http.go`, so the refusal is structurally identical everywhere; a new test pins the sentinel, the code, and the `email` param so the shape can't drift. Documented in `docs/ADR024-members.md` (new "An invite never re-roles an existing member" section) and the `ADR018` members row.
- **t004 (simplify).** The prefix-stripping I first wrote inline duplicated w1/m81's `classifyAddError`; hoisted to one `refusalReason` in `common/lib/graphql-error.ts` and **both** dialogs now use it (the custom-domain hook lost 22 lines and its now-redundant Apollo import).
- **t005 (coverage), all proven red on pre-fix code:** the real-Postgres `TestRedeemingAnInviteNeverRerolesAnExistingMember` reproduces the exact production bug against the old SQL (`role after redeeming a lower-role invite = "developer", want admin`) and additionally caught that the **emailed-link path** demoted the admin to `viewer`; `TestInviteRefusesAnExistingMember` (red when the guard is neutered); the dialog validation test (red on the old `!email.trim()` gating). Plus the cross-surface shape test and `refusalReason` unit tests.

**Testing note:** two unrelated failures appeared mid-session (`TestPGGitWebhookReplay*`, `dbrole`) and were chased to ground as **environmental, not regressions** — those tests don't clean their own state, so they fail when the suite is run twice against one database or when a cluster-wide Postgres role survives from a prior database. Both pass on a pristine cluster, verified with my changes stashed and unstashed.

**Post-ship follow-up:** re-verify on prod that inviting an existing member is refused inline and leaves the role untouched (the live repro that started this).

## Source + Goal linkage

- **Source:** 2026-08-19 customer-perspective QA walk of dashboard.bex.co (round 2). Proven live: re-inviting `puncsky@gmail.com` (an Admin, the current user) as Developer silently demoted them Admin→Developer; restored during the walk and re-verified via the API.
- **Goal linkage:** authz correctness + Render parity on the workspace members surface (`docs/ADR024-members.md`, `docs/ADR012-auth.md`, `docs/ADR018-render-parity.md`).
- **Expected outcome:** an admin can no longer change an existing member's privileges by "inviting" them; role changes stay confined to the audited `ChangeRole` verb.
- **Why now:** found live in production; admin-gated so not a cross-boundary escalation, but a real hazard (accidental demotion of an admin/owner → potential loss of admin control) triggerable by a routine action and applied later on the victim's next request. Render parity task **included** — the invite verb spans REST/GraphQL/MCP + UI.
