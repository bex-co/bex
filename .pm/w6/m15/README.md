# w6 · m15 — Workspace lifecycle polish: invite-gate CTA · purger dedup · ChangePlan pending-invites guard

**Worker:** worker6 **Goal:** Three loose ends `w6/m11`/`w6/m12`/`w6/m13` each explicitly deferred are closed: the Hobby invite-gate error links to the upgrade path instead of dead-ending, the three resource purgers share one implementation instead of three copies of the same loop, and a plan downgrade with outstanding invites is refused instead of silently allowed. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                                                                             | est | depends_on             |
| ---- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --- | ----------------------- |
| t001 | Invite-gate error CTA: link the Hobby member-limit rejection in the invite dialog to the plan-change section (`w6/009`)                                                          | 25m | —                        |
| t002 | Dedupe the `postgres/purger.go` · `keyvalue/purger.go` · `apps/purger.go` List+delete loops into one shared helper or a `client.DeleteAllOf` call (`w6/010`)                     | 25m | —                        |
| t003 | `ChangePlan` downgrade guards count pending `tenant_invites` (member-count + role checks), refuse with a reason naming the invites; add `ListInvites` to the `WorkspaceStore` interface + test fakes (`w6/011`) | 40m | —                        |
| t004 | Acceptance: invite-gate CTA click-through; purger tests stay green post-dedup; a Pro→Hobby downgrade with a pending over-cap invite is refused with the invite named             | 25m | t001, t002, t003         |
| t005 | Render parity — verify `ChangePlan`'s downgrade-refusal error shape/semantics stay consistent across GraphQL/dashboard (t001/t002 have no surface semantics to check)            | 15m | t004                     |
| t006 | Simplify — `/simplify` over the code this milestone changed                                                                                                                       | 15m | t005                     |
| t007 | Test coverage — meaningful tests for the CTA render, the deduped purger helper, and the pending-invite downgrade guard                                                           | 30m | t005                     |
| t008 | Closeout — verify DoD met, then move the milestone to `done/`                                                                                                                     | 10m | t007                     |

## Definition of done

All three fixes verified live/in-tests exactly as each source note specifies: the invite-gate error links to the plan section and opens/navigates to it; all three purgers share one implementation with existing purger tests unchanged and green; a `ChangePlan` downgrade with an outstanding invite that would violate the target plan's member/role limits is refused (not silently allowed) with a reason naming the invite(s).

## Source + Goal linkage

- **Source:** `/pm-brainstorm more milestones to work on` 2026-07-13 — groups `w6/009` (invite-gate CTA, filed after `w6/m11`), `w6/010` (purger dedup, flagged by `w6/m11`'s own `/simplify` pass as out-of-diff), and `w6/011` (ChangePlan pending-invites downgrade guard, filed after `w6/m13` closed the accept-time invariant but left the downgrade UX rough).
- **Goal linkage:** workspace lifecycle correctness — `w6`'s founding mandate (Render workspace lifecycle parity); each item closes a loose end an earlier `w6` milestone explicitly deferred rather than forgot.
- **Expected outcome:** three small, independent correctness/UX gaps close in one pass instead of sitting as permanently-deferred inbox notes.
- **Why now:** all three are stable and unblocked — the `workspaces/service.go` lock from a concurrent session that held `011` back during `w6/m13` is clear, and grouping avoids three separate milestone-creation overheads for genuinely small, independent fixes (the same pattern `w6/m9` already used for five sub-hour security notes).
- **Render parity closing task:** included but scoped narrowly (t005) — only `ChangePlan`'s downgrade-refusal shape is a user-facing surface change; the CTA (t001) and purger dedup (t002) have no REST/GraphQL/MCP/UI semantics to compare.
