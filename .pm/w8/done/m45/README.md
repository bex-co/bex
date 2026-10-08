# w8 · m45 — A service name given to the CLI resolves in the caller's default workspace, not the selected one, so name-based verbs can act on another workspace's service

**Worker:** worker8 **Goal:** a service addressed by **name** resolves only within the workspace the caller selected. When no workspace is selected and the name is ambiguous across the caller's workspaces, it is refused with the candidate ids instead of silently picking one. `bex deploys create <name>` can never act on a service in a workspace other than the one `bex workspace current` shows. **Status:** done (2026-10-08, live closeout on bex v0.3.2) 

## Tasks (in order)

| id   | title                                                                                                       | est | depends_on       |
| ---- | ----------------------------------------------------------------------------------------------------------- | --- | ---------------- |
| t001 | Decide how the selected workspace reaches name resolution (request hint vs. refuse-ambiguous), record it — **DONE**     | 30m | —                |
| t002 | Server: name resolution refuses a name that matches services in more than one accessible workspace (409 + ids) — **DONE** | 45m | t001             |
| t003 | Carry the CLI's selected workspace to name-based by-path verbs, per t001's decision — **DONE**                           | 45m | t001             |
| t004 | Enumerate every name-accepting by-path verb (deploys, restart, instances, jobs, logs, …) and cover each — **DONE**       | 30m | t002, t003       |
| t005 | Render parity — **DONE**                                                                                                | 20m | t004             |
| t006 | Simplify — **DONE**                                                                                                     | 15m | t005             |
| t007 | Test coverage — **DONE**                                                                                                | 30m | t006             |
| t008 | Closeout — **DONE**                                                                                                     | 15m | t007             |

## Definition of done

- With a service named `N` in workspace A (selected via `bex workspace set A` or `BEX_WORKSPACE=A`) and another service named `N` in workspace B that the same user belongs to, `bex deploys list N`, `bex services instances N` and `bex deploys create N` all act on **A's** service.
- With no selected workspace, the same name either resolves in the single workspace that has it, or is refused with `409` naming both ids. It never silently picks one.
- Addressing by `srv-` id is unchanged: ids are global, and cross-workspace by-id reads for a member keep working, as they do on Render.

## Evidence (2026-09-26, `/qa-find-bugs-cli` sweep 16, production `726042a28`)

Released `bex v0.2.1` (pin v2.27.0), human device login. The QA identity belongs to three workspaces: `bex-canary` (selected), `bex`, and `tian-personal`, which holds a real service `tianpan-v4-web` (`srv-da40m1qii7bs73drbqlg`). Only **reads** were issued against it. A same-named fixture was created in `bex-canary` (`srv-darp4s8d0qnc73d79tk0`, deleted **by id** afterwards):

```text
$ bex services -o json   (BEX_WORKSPACE=bex-canary)  → tianpan-v4-web = srv-darp4s8d0qnc73d79tk0   # the selected workspace's own service
$ bex deploys list tianpan-v4-web -o json   (BEX_WORKSPACE=bex-canary)
→ [dep-daqeee3bdpcs73f5eghg, dep-daqd3pbbdpcs73f5egh0, …]      # tian-personal's service's deploys
   (the fixture's own deploy is dep-darp4s8d0qnc73d79tl0 — not returned)
$ bex services instances tianpan-v4-web   (either workspace selected)  → srv-da40m1qii7bs73drbqlg-…
```

Before the fixture existed, `bex deploys list tianpan-v4-web` with `bex-canary` selected already returned tian-personal's service, which shows that a name from an unselected workspace resolves at all. No mutation was sent to the real service. The mutation risk (`restart`, `deploys create`, `deploys cancel`, `jobs create` by name) follows from the same resolver, and t004 must prove it on fixtures only.

## Mechanism

- The pinned CLI passes the argument straight into the path for these verbs (for example `pkg/deploy/repo.go:17` `ListDeploysForService(serviceID)`) and sends no workspace. Render itself only accepts `srv-` ids there. Accepting names is a bex extension.
- `lego/backend/internal/core/base.go:655-663` `resolveWorkspaceUncached`: with no named workspace, `acting` = the caller's **default** tenant.
- `base.go:1054-1058` (`AuthorizeApp`) and `:1505-1509` (`GetApp`) try `appCandidateNames(acting, name)` (`:1412-1417`) in `AppNamespace(acting)` first, and only then fall back to a cluster-wide `LabelServiceName` match (`:1073`, `:1525`). So the default workspace's same-named service always wins, and a name that exists only in another accessible workspace still resolves. Neither step knows the CLI's selected workspace.
- The CLI's selected workspace is only used client-side (list filters, and the `services update`/datastore name resolvers, which say "No service named 'x' in workspace tea-…"). The by-path verbs bypass it.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs-cli` sweep 16 (w8), 2026-09-26.
- **Goal linkage:** tenant-safe targeting (w6/m14 deterministic workspace targeting covered the default pick; this is the name-selector gap), and Render compatibility (ADR006: Render accepts only ids on these paths).
- **Expected outcome:** a name typed in one workspace can never restart, deploy, or cancel another workspace's service.
- **Why now:** multi-workspace users are normal (the QA identity alone has three), and the failure mode is a silent wrong-target mutation.
- **Render parity included:** it changes REST/GraphQL/MCP resolution semantics.

## Related

- `w8/m47` (sweep 19): the same resolver ignores **display names** (renames), so a swapped rename also targets the wrong service within one workspace. Sequence m45 → m47 or land them together.

## Unverified

- Mutations by name (not exercised against the real service by design; t004 proves them on fixtures).
- Whether GraphQL/MCP name-addressed verbs share the same default-workspace preference (same `AuthorizeApp`, so likely).
- ~~Datastore resolution across workspaces~~ **checked in sweep 17: not affected.** `bex postgres get <name>` / `bex keyvalues get <name>` for a datastore that exists only in another member workspace answer `No Postgres database named '…' in workspace tea-daif…` (the pinned CLI resolves datastore names client-side, scoped to the selected workspace). By id, they correctly work across workspaces. The gap is specific to services addressed through by-path verbs.

## Blocked (2026-09-26)

t001, t002, t004–t007 are done. With no workspace named, a service name visible in several of the caller's workspaces is refused `409 SERVICE_NAME_AMBIGUOUS` with every candidate id, on every by-path verb. The silent wrong-target mutation is gone. Open:

1. **t003: user sign-off** on the launcher header (t001 option (a)) that lets the selected workspace win for names, which DoD bullet 1 needs.
2. **t008: live closeout** after deploy (`blocked/m42`) with a logged-in CLI, fixtures only.

## Unblocking work (2026-10-02)

The user approved t001's native-header option. The local launcher sends the saved or environment-selected workspace as `X-Bex-Workspace` to the configured control plane. The API resolves workspace names within the caller's memberships, validates access, and scopes name-addressed `/v1/services/{name}` paths only. Typed service IDs ignore the hint and keep resource-workspace authorization. An explicit selection never falls through to a foreign workspace; no-selection ambiguity still returns 409.

Tests cover ID and name selections, saved/environment precedence, renamed services, no-selection ambiguity, unknown/nonmember selections, nested paths and cross-workspace immutable IDs. GraphQL/dashboard retain their existing owner/resource-ID contract, MCP its `workspaceId`; the header is explicitly a bex extension, with no upstream fork.

**Remaining:** ship the API and launcher, release the CLI, and verify the two-workspace name/read/restart scenario live. The sign-off and login blockers are resolved.


## Verification and remaining release gate (2026-10-02)

Full backend suite against isolated Postgres/OpenFGA/OpenBao, operator `make test`, full CLI suite, backend/CLI lint, targeted Go race tests, workflow guards and ci-red-streak fixtures passed. The final affected backend packages passed again after review changes. Overlay mutation checks confirmed the image, workspace and rename regressions fail with their fixes removed. Markdown was formatted; QA fixtures and the isolated credentials were cleaned up.

Implementation and review are complete locally. Repository `AGENTS.md` requires an explicit `$ship` before commit/push. After ship, observe CI/production and complete this milestone's remaining live closeout; m45 also needs the updated CLI released. The earlier policy/sign-off/login blockers are resolved.

## Pinned CLI acceptance correction — 2026-10-03

The CLI's `restart` command is ID-only: upstream v2.27.0 `cmd/restart.go:18` and `pkg/resource/service.go:223–244` reject an ordinary service name with `unknown resource type` before a restart request. This was reproduced on an owned renamed service after rollout; see [m47's full CLI evidence](../m47/cli-production-rename-20261003.json). Earlier restart-by-name CLI examples in this record describe an unsupported invocation; they are not evidence of a server failure and do not justify changing the pinned client. The goal/DoD now use `deploys create <name>` for the supported CLI mutation. Direct REST restart-by-name remains a separate server resolver test. The one-workspace rename retest passed; it does **not** establish this milestone's two-workspace isolation acceptance or close this milestone.

## Re-triage (2026-10-07, w8 /loopx)

- **API: shipped and live.** The `X-Bex-Workspace` resolution landed in `faaaba0ce` (2026-10-02) and has deployed since; m42 records three consecutive green deploys through `cd65ad10a`.
- **CLI: not released.** The launcher change is in `faaaba0ce`, but the newest tag, `bex-cli/v0.2.1`, predates it (`git merge-base --is-ancestor` fails), so installed CLIs don't yet send the header.
- **Live two-workspace check: attempted with a launcher built from `main`, but blocked.**
  - An isolated `bex login` device flow, approved in a browser carrying the QA Kratos session (`qa-login.sh --serve`), reached a Hydra login that demands **Reauthenticate** with the QA account's password.
  - The QA identity is the operator's own account, and its password is never handed to an agent. The run was cleaned up: the login process was killed, the QA session revoked with `--logout`, and temp config removed.

**Remaining gates (user):**

1. Approve `/release cli`, so the header reaches installed CLIs. Publishing a version is outward-facing.
2. Run the live t008 check yourself, or provide a reauth-free QA path. With the released (or `main`-built) CLI logged in as an identity in two workspaces, a same-named fixture in each, and `BEX_WORKSPACE=A`:
   - `bex deploys list N`, `bex services instances N` and `bex deploys create N` must act on A's fixture;
   - with no selection, the name must return `409 SERVICE_NAME_AMBIGUOUS`.

## CLI released (2026-10-08)

The user approved the release. `bex-cli/v0.3.0` (source `999eab321`) ships the `X-Bex-Workspace` launcher change, and `v0.3.1`/`v0.3.2` follow with w8/064/065 fixes. The release workflow, the cosign signature, Homebrew (checksums match), both installer paths and the update notice were all verified. **Remaining gate:** the live two-workspace t008 check. QA device login demands a password reauth that the agent must not perform, so a user-run check (or a reauth-free QA path) is needed.

## Live closeout (2026-10-08, w8 /loopx, released `bex v0.3.2`)

Human device login as the QA identity, which belongs to `bex`, `tian-personal` and `bex-canary`. The isolated CLI config had **no workspace selected**. The real service `tianpan-v4-web` (`srv-da40m1qii7bs73drbqlg`, tian-personal) received reads only. A same-named fixture `srv-db3vq48gu8gc73cl12ug` was created in bex-canary and deleted by ID afterwards.

| DoD check | Result |
| --- | --- |
| `BEX_WORKSPACE=bex-canary bex deploys list tianpan-v4-web` | ✅ identical to `deploys list <fixture-id>` |
| `BEX_WORKSPACE=bex-canary bex services instances tianpan-v4-web` | ✅ `srv-db3vq48gu8gc73cl12ug-…` only |
| `BEX_WORKSPACE=bex-canary bex deploys create tianpan-v4-web --confirm` | ✅ `dep-db3vqle2bdqc73aphs00` on the fixture (1 → 2 deploys); real service unchanged (20 → 20) |
| `BEX_WORKSPACE=tian-personal bex deploys list tianpan-v4-web` | ✅ identical to `deploys list srv-da40m1qii7bs73drbqlg` (read) |
| No selection: `deploys list` / `deploys create tianpan-v4-web` | ✅ `409` `SERVICE_NAME_AMBIGUOUS` naming both IDs; no deploy on either service |
| By-ID cross-workspace read (`BEX_WORKSPACE=bex-canary bex deploys list srv-da40…`) | ✅ 20 deploys, unchanged behavior |
