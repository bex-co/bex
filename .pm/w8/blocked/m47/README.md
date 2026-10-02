# w8 · m47 — A renamed service keeps answering to its old name: rename skips the uniqueness rule, and name lookup ignores the name you see

**Worker:** worker8 **Goal:** the name a user sees for a service (`bex services`, the dashboard) is the name every by-name verb acts on, and it stays unique within the workspace, so `bex restart <name shown in the list>` always acts on the service shown with that name. **Status:** blocked

## Tasks (in order)

| id   | title                                                                                                        | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | Rename (`SetDisplayName`) enforces the same workspace-uniqueness rule as create: 409 against every displayed and creation-time name — **DONE** | 40m | —          |
| t002 | Server name resolution matches the displayed name (display name, falling back to the creation name) and refuses ambiguity — **DONE** | 45m | t001       |
| t003 | Audit production for existing duplicate displayed names per workspace; record the handling (no silent renames) — **DONE** | 20m | t001       |
| t004 | Render parity — **DONE**                                                                                                | 20m | t002, t003 |
| t005 | Simplify — **DONE**                                                                                                     | 15m | t004       |
| t006 | Test coverage — **DONE**                                                                                                | 30m | t005       |
| t007 | Closeout                                                                                                     | 15m | t006       |

## Definition of done

- `bex services update <srv-B> --name <name shown for another service in the same workspace>` → `409 (CONFLICT): name "…" is already in use`, the same answer as `services create`. Concurrent renames of two services to one new name: exactly one succeeds.
- After renaming A from `x` to `y`: `bex deploys list y`, `bex services instances y`, and `bex restart y` act on A. `x` no longer resolves to A once another service is displayed as `x`, and never silently targets a different service than the one listed under that name.
- Existing duplicates in production are listed and resolved by decision (t003).

## Evidence (2026-09-26, `/qa-find-bugs-cli` sweep 19, production `726042a28`)

Released `bex v0.2.1` (pin v2.27.0), human device login, workspace `bex-canary`. Fixtures: A `srv-darql5psmc7s73cq5ik0` (created `qa-20260926-99c31d-a`) and B `srv-darql5od0qnc73d79u50` (created `…-b`), both deleted.

```text
# 1. rename skips uniqueness: concurrent (3/3 rounds) AND sequential
$ bex services update A --name …-same1 & bex services update B --name …-same1      # both exit 0; list shows two "…-same1"
$ bex services update A --name …-seqA ; bex services update B --name …-seqA        # both exit 0
$ bex services update B --name hello-go                                            # exit 0: now two services listed as "hello-go"
#    (B renamed back immediately; the pre-existing hello-go was never mutated)
#    control: services create with an in-use name → 409 (CONFLICT) "already in use" (w4/m19)

# 2. by-path name lookup ignores the displayed name
$ bex deploys list …-seqA     # both A and B displayed as …-seqA → 404 Not Found
$ bex services update …-seqA …  → "Multiple services found with name '…-seqA'. Pass the service ID" (the pinned CLI's client-side resolver refuses safely)

# 3. swap → wrong target
$ bex services update A --name …-b ; bex services update B --name …-a
$ bex services -o json      → B (…79u50) is listed as "…-a",  A (…q5ik0) as "…-b"
$ bex deploys list …-a      → [dep-darql5psmc7s73cq5ikg]   # A's deploy — not B, the service listed as "…-a"
$ bex services instances …-a → srv-darql5psmc7s73cq5ik0-…   # A
```

So `bex restart …-a`, `bex deploys create …-a` and `bex deploys cancel …-a` would act on the service the list calls `…-b`. No mutation by name was sent in this sweep; the reads prove the target.

## Mechanism

- `lego/backend/internal/apps/service.go:4203-4232` `SetDisplayName` writes `spec.displayName` (and the `apps.display_name` mirror) with **no uniqueness check**. Create's check (w4/m19, `duplicatename_test.go`) covers only the immutable creation name.
- Server name resolution (`lego/backend/internal/core/base.go:1054-1080` `AuthorizeApp`, `:1505-1530` `GetApp`) matches the CR object name (`CRName(acting, name)`) and the `bex.co/service-name` label (`LabelServiceName`, `base.go:184`), both of which hold the **creation-time** name. It never consults `displayName`.
- Everything a user sees shows `displayName` (REST `name` = display name when set, dashboard, `bex services`). The pinned CLI's client-side resolver matches the listed `name` and refuses duplicates, but by-path verbs (`deploys`, `restart`, `instances`, `jobs`, `logs`) send the typed string to the server, which uses the hidden name.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs-cli` sweep 19 (w8), 2026-09-26, while probing a rename-uniqueness race.
- **Goal linkage:** tenant-safe targeting and the w4/m19 name-uniqueness invariant ("names are unique within a workspace"); w8/m8 introduced the mutable display name without extending either.
- **Expected outcome:** a service can be renamed without leaving a trap where its old name silently targets it while another service is listed under that name.
- **Why now:** renames are a normal operation, and by-name CLI use in scripts is common.
- **Coordination:** `w8/m45` changes the same resolver (workspace scoping of names). Land together, or sequence m45 → m47 so t002 builds on m45's ambiguity refusal.
- **Render parity included:** rename conflict behavior and name resolution change on REST/GraphQL/MCP and the dashboard Settings rename control.

## Unverified

- Render's own rename-uniqueness behavior (w4/m19 records creates as unique on Render; renames not checked). t004 must confirm before t001's 409 ships.
- GraphQL `setDisplayName` / MCP `update_service(displayName:)`: same verb, presumed identical.

## Blocked (2026-09-27)

t001–t006 are done. Renames follow create's workspace-uniqueness rule (409, race-safe under a shared advisory lock), and by-name resolution matches the name each service is shown as, so the sweep-19 swap now targets the listed service. Production has no duplicate displayed names. Only **t007** remains: the live closeout on production fixtures, which needs the deploy to land (`blocked/m42`) and a logged-in `bex` CLI.

## Live recheck and additional fix (2026-10-02)

Using isolated QA login and two owned free services in `bex-canary`, duplicate rename returned 409 and concurrent renames produced exactly one winner. However, after renaming a service, `deploys list <newName>` and `services instances <newName>` returned **404**: displayed-name lookup searched only the account's default workspace; cross-workspace fallback searched the immutable creation label. Both fixtures were restored to their original names after the check.

The local core resolver now searches current displayed names across visible workspaces when no explicit workspace is supplied, validates access, detects renamed ambiguity without exposing foreign IDs, and restricts explicit selections to their workspace. m45 supplies the CLI selection. Regression tests include a renamed service outside the account default and prevent explicit selection from reaching another workspace through a legacy creation-name alias.

**Remaining:** ship, and repeat the renamed deploys/instances/restart checks live. m47 is not closed on the basis of the earlier partial fix.


## Verification and remaining release gate (2026-10-02)

Full backend suite against isolated Postgres/OpenFGA/OpenBao, operator `make test`, full CLI suite, backend/CLI lint, targeted Go race tests, workflow guards and ci-red-streak fixtures passed. The final affected backend packages passed again after review changes. Overlay mutation checks confirmed the image, workspace and rename regressions fail with their fixes removed. Markdown was formatted; QA fixtures and the isolated credentials were cleaned up.

Implementation and review are complete locally. Repository `AGENTS.md` requires an explicit `$ship` before commit/push. After ship, observe CI/production and complete this milestone's remaining live closeout; m45 also needs the updated CLI released. The earlier policy/sign-off/login blockers are resolved.
