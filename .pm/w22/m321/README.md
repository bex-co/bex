# w22 · m321 — The storage meter's registry credentials: the operator mints a read user and a delete user for bex-api

**Worker:** worker22 **Goal:** bex-api can read a suspended sandbox's image size from Zot and delete a terminated sandbox's `snap-gen` tags, through two operator-managed Zot users scoped to `snapshots/**`, with no admin credential in bex-api **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Design: where the two Zot credentials live, and how the operator may write them | 45m | — |
| t002 | Operator: mint the two Zot users, their ACL entries and the Secret | 1h30m | w22/m321/t001 |
| t003 | Hook it into the operator: reconcile, RBAC and manifests | 1h | w22/m321/t002 |
| t004 | bex-api: read the Secret into the storage meter's environment | 45m | w22/m321/t003 |
| t005 | Production check: a suspended sandbox is sized, a terminated one's images are deleted | 1h | w22/m321/t004 |
| t006 | Simplify | 20m | w22/m321/t005 |
| t007 | Test coverage | 40m | w22/m321/t005 |
| t008 | Closeout | 10m | w22/m321/t006, w22/m321/t007 |

## Definition of done

- Two Zot users exist, minted by the operator: `bex-sandbox-meter` with `read` and `bex-sandbox-reaper` with `read` and `delete`, both on `snapshots/**` and nothing else; the Secret bex-api reads them from is created and self-heals
- bex-api starts with and without the Secret; with it, retained images are sized and a terminated sandbox's tags are deleted
- A production check shows one image sized and one terminated sandbox's tags deleted, and a repository the controller did not write to untouched

## Source + Goal linkage

- **Source:** w22/006 (filed by w22/m313 t004 on 2026-11-10), promoted 2026-11-10 by `/loopx w22`; [docs/ADR023-usage-metering.md](../../../docs/ADR023-usage-metering.md) § Sandbox snapshot storage, [docs/ADR104-sandbox-render-parity.md](../../../docs/ADR104-sandbox-render-parity.md) D11
- **Goal linkage:** Render parity first (ADR008): a suspended sandbox pays snapshot storage as on Render, which needs the meter to read sizes; D11's bound on registry growth needs the reaper
- **Expected outcome:** the storage meter and the reaper work on production without anyone hand-setting a credential, and the registry stops keeping a terminated sandbox's images forever
- **Why now:** w22/m313 shipped the meter and the reaper with no credential to run them: sizing is off, so no storage is billed and `SandboxStorageUnsized` fires once a sandbox is suspended
- **Render parity:** omitted. No REST, GraphQL, MCP or UI surface changes.
- **Open decision, mine to make in t001, not the user's:** where the Secret lives (the operator caches Secrets only in the apps namespace, and bex-api is in `bex-system`)
- **Safety:** the reaper is the first registry writer bex-api has had; it deletes only the controller's `snap-gen<N>` tags and its credential covers `snapshots/**` only
