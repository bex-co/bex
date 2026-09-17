# w2 · m163 — Credential offboarding: a removed member's machine credentials

**Worker:** worker2 **Goal:** removing a member removes what they can still act with — their API keys stop being a back door into a workspace they were just removed from **Status:** t001–t007 done 2026-09-16; t008 closeout open (needs a live OpenFGA-enforced walk)

## Tasks (in order)

| id   | title                                                                               | est | depends_on | status     |
| ---- | ----------------------------------------------------------------------------------- | --- | ---------- | ---------- |
| t001 | Policy decision: revoke, orphan-with-visibility, or reassign a removed member's keys | 40m | —          | — **DONE** |
| t002 | Implement the decided disposition in the member-removal path                        | 1h  | t001       | — **DONE** |
| t003 | Make the same disposition run on the leave path and the ADR086 offboarding path     | 40m | t002       | — **DONE** |
| t004 | Surface the outcome: the removal response and the API-keys list tell the truth      | 40m | t002       | — **DONE** |
| t005 | Render parity check for key ownership and removal semantics                         | 30m | t004       | — **DONE** |
| t006 | Simplify the code this milestone changed                                            | 30m | t005       | — **DONE** |
| t007 | Test coverage: a removed member's key cannot act                                    | 45m | t005       | — **DONE** |
| t008 | Closeout                                                                            | 15m | t006, t007 | open       |

## t001 decision (2026-09-16) — **revoke**

Confirmed first, not assumed: `store.BindClient` (`store/store.go:807`) gives each API key **its own** `tenant_members` row at role `developer`, keyed by the key's Hydra client id. `members.Remove` (`members/service.go:956`) revokes only the *creator's* tuple and deletes only the *creator's* row. So the key outlives the membership with full developer access, and nothing on the Team surface attributes it to the person who just left.

The three options and what each actually costs:

| option | failure mode |
| --- | --- |
| **Revoke** | A departing engineer's key may be what a production CI pipeline authenticates with. Removing them kills the pipeline with no warning — an outage caused by an HR action, which is the worst possible coupling to debug. |
| **Retain, mark as orphaned** | The access stays open indefinitely; the badge only helps if somebody reads the API-keys page. "Visible" is not "closed" — and the person removed is often removed *because* their access should end now. |
| **Reassign to the removing admin** | The key keeps working and is attributable, but the admin silently inherits a credential they never minted and whose blast radius they have not reviewed. It launders provenance: the audit trail says the admin owns a key they may not know exists. |

**Decided: revoke** — the same disposition ADR086 already chose for account deletion, applied to every exit. The argument that settles it: revocation that does not cover a subject's delegated credentials is not revocation. A key created by a member is that member's authority in machine form; ending the membership while leaving the delegation live means the removal only *appears* to have happened. Retention and reassignment both keep a credential alive whose authorizing relationship has ended.

**The tension, recorded because it is real.** [ADR078 §1](../../../docs/ADR078-github-workspace-connections.md) praises bex for *not* having Render's member-departure breakage in the git-connection model, and revoking here deliberately introduces exactly that failure for API keys. The two are not inconsistent, and the distinction is the point:

- A **git connection** is workspace-owned in substance: it is bound by an admin proving they administer an installation, and no member's continued employment is part of what authorizes it. Nothing breaks when they leave because nothing about the binding referred to them.
- An **API key** is a delegation of one subject's authority. `CreatedBy` is not decoration — it is the authorizing relationship. When that relationship ends, the credential has no basis left.

So the rule is: bex avoids member-departure breakage wherever the resource never depended on the member, and accepts it exactly where it did. The mitigation for the CI case is a key minted by a remaining admin (or a future workspace service account), **not** a credential that outlives its authorization. Render diverges by making keys user-owned so they leave with the user — which reaches the same end state by a different route, and is worth noting as convergence rather than divergence on *this* row.

**Scope of the rule:** keys created by the departing subject **and bound to the workspace being exited** — never that subject's keys in other workspaces. All three exits (admin removal, self-leave, account deletion) converge on it; account deletion stays global by subject, which is correct for that path.

**ADR amendments owed:** [ADR024](../../../docs/ADR024-members.md) gets the membership rule (removal disposes the member's machine credentials) and [ADR086](../../../docs/ADR086-account-deletion.md)'s disposition table gets the cross-reference that the rule is no longer deletion-only.

## Definition of done

On a store-backed environment with OpenFGA enforced:

- A member who created API keys is removed from a workspace; every key they created against that workspace reaches the decided terminal state (revoked, or explicitly retained and visibly attributed), and the behavior is identical whether the removal came from an admin, from the member leaving, or from account deletion.
- If the decision is revocation: the key's Hydra client is deleted, its `tenant_members` binding row and OpenFGA tuple are gone, and a request bearing that key returns 401 — verified live, not only in a unit test.
- If the decision is retention: the API-keys surface marks each such key with its creator's removed status, and the decision plus its reasoning is written into ADR024/ADR086.
- No key belonging to a **remaining** member is affected.

## Source + Goal linkage

- **Source:** the 2026-09-16 offboarding research (filed alongside `w5/m101`–`m103`). `apikeys.AccountTeardown.CleanupSubject` (`lego/backend/internal/apikeys/service.go:114`) unbinds and deletes every key whose `CreatedBy` matches the departing subject — that is the ADR086 account-deletion path. `members.Remove` (`lego/backend/internal/members/service.go:956`) does none of it: the key's own `tenant_members` binding row (role `developer`, written by `store.BindClient`, `store/store.go:800`) and its OpenFGA tuple both survive, so a removed teammate's key keeps full developer access to the workspace they were removed from.
- **Goal linkage:** ADR012 auth / ADR024 members — revocation that does not cover a subject's delegated credentials is not revocation; ADR086 already establishes the correct disposition for the deletion path, so this is closing an asymmetry, not inventing policy.
- **Expected outcome:** one offboarding rule, applied by every path that ends a membership.
- **Why now:** `w5/m103` is about to filter machine bindings off the Team surface — which makes these keys *less* visible while they stay live. The disposition has to be settled before that lands, or the gap becomes invisible as well as open.
- **Render parity task included:** yes — key ownership semantics and the removal response are user-facing API behavior (note: Render's API keys are user-owned and leave with the user; bex's are workspace-owned, which is the crux of t001).
