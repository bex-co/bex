# A registry credential can be created in a workspace it can then never be read, updated or deleted from

Why: `createRegistryCredential` and `registryCredentials` both take `ownerId` and bind it, while `registryCredential`, `updateRegistryCredential` and `deleteRegistryCredential` take only an id and scope their store lookup to the caller's **default** workspace — so a credential created in any other workspace is listable but unreachable, and it keeps consuming that workspace's credential quota with no product path to remove it.

Found by live `/qa-find-bugs` 2026-09-21 pass 150 (w4-targeted, `muse.env` credentials). Fixture `qa-20260921-rc` created and deleted.

## The asymmetry, in one package

`lego/backend/internal/registrycreds/service.go`:

| verb | line | binds the requested workspace? |
| --- | --- | --- |
| `Create` | `:202` | **yes** — `ctx = core.WithWorkspace(ctx, req.OwnerID)` |
| `List` | `:148` | **yes** — `ctx = core.WithWorkspace(ctx, ownerID)` |
| `Get` | `:167` | no — no owner parameter; `s.Store.GetRegistryCredential(ctx, s.WorkspaceOrDefault(ctx), id)` (`:175`) |
| `Update` | `:270` | no — `workspaceID := s.WorkspaceOrDefault(ctx)` (`:277`) |
| `Delete` | `:335` | no — `workspaceID := s.WorkspaceOrDefault(ctx)` (`:342`) |

The GraphQL surface matches: `registryCredentials(ownerId:)` and `createRegistryCredential(ownerId:, …)` carry the argument; `registryCredential(id:)`, `updateRegistryCredential(id:, …)` and `deleteRegistryCredential(id:)` do not — verified by introspection.

`Base.Tenant(ctx)` resolves the caller's default workspace when none is named (`core/base.go:957-972`), so for an account in more than one workspace the last three verbs can only ever address one of them.

## Why this is worse than the sibling case

[`w4/122`](122.md) recorded the same shape for sandboxes. This one is sharper for two reasons:

- **Delete is affected.** A credential created in a non-default workspace cannot be removed through the product at all. `w4/122`'s sandboxes at least expire on `timeoutSeconds`; a registry credential is durable.
- **It consumes a quota that then cannot be freed.** `Create` enforces `MaxCredentials` per workspace and refuses with `REGISTRY_CREDENTIAL_LIMIT` — _"workspace already owns %d registry credentials (limit %d); **delete unused credentials** or raise the limit"_ (`:230-236`). The remedy that error names is the verb that cannot reach them.

## Fix

Give `Get`, `Update` and `Delete` the same `ownerId` that `Create` and `List` already accept, bound the same way (`core.WithWorkspace`), so `ValidateNamedWorkspace` supplies the membership check for free. Keep the argument optional so omitting it keeps today's default-workspace behaviour and no client breaks.

Do it **with** `w4/122` — same mechanism, same one-line binding, and the sweep below says whether anything else needs it. Estimate: ~40m for the three verbs plus a test that a credential created with `ownerId: B` is readable, updatable and deletable with `ownerId: B`.

## The sweep that bounds this

`grep -rn "WorkspaceOrDefault(ctx)" lego/backend/internal/*/service.go` returns call sites in exactly five packages: `registrycreds`, `sandbox` (`w4/122`), `github`, `events` (`GetServiceEvent`), and `clitelemetry`. Everything else resolves a resource by its globally-unique opaque id and authorizes against that resource's **own** workspace — `core.Base.AuthorizeApp`'s documented behaviour — which is why `service(id:)`, `database(id:)` and friends need no workspace argument and work across workspaces. So the affected set is small and enumerable, not a sprawling audit.

**`github` and `events` were examined in pass 151:**

- **`github` is clean.** All nine of its GraphQL verbs carry `ownerId` — `gitConnections`, `repos`, `repoBranches`, `repoRuntimeDetection`, `gitClaimSelection`, `connectGit`, `disconnectGit`, `claimGit`, `selectGitClaim` — so its `WorkspaceOrDefault(ctx)` calls are _resolving_ a bound argument, exactly like `registrycreds.List`. Nothing to do. (`setRepo` has no `ownerId` but is a service mutation resolved by service id, outside this family.)
- **`events` has a narrow instance, on the hydration path.** `serviceEvent(id:)` takes only an id and `GetServiceEvent(ctx, s.WorkspaceOrDefault(ctx), eventID)` (`events/service.go:763`) scopes to the default workspace; REST's `GET /v1/events/{eventId}` behaves the same and ignores an `ownerId` query param. The list sibling `serviceEvents(serviceId:)` is fine — it resolves through the service id, which carries its own workspace.

  This matters more than a by-id read usually would, because **a webhook delivery is how a caller obtains an event id in the first place**: ADR018's webhook row promises that "every advertised `webhook-id`/`data.id` resolves through the owner-scoped event index to the same REST/GraphQL/MCP event". For a subscriber whose endpoint covers a non-default workspace, that resolution is exactly what fails. Same one-line fix as the three verbs above.

- **`clitelemetry`** was not examined; it writes rather than reads and is unlikely to have a tenant-visible consequence, but it is the last unchecked name on the list.

## Unverified

- **Not reproduced live.** Demonstrating it needs a credential created in a non-default workspace, and the QA rules forbid creating in a workspace other than the hunt's own. The evidence above is the schema (introspected) and the five call sites (read directly); the behaviour follows from `WorkspaceOrDefault`'s documented fallback rather than from an observed orphan.
- Whether the REST and MCP adapters carry the same asymmetry was not checked — GraphQL was.

## Also checked this pass, and found correct

The registry-credential surface is otherwise in good shape, and the parts that matter most are the parts that hold:

- **The secret never comes back.** `RegistryCredential` has no `authToken` field at all, and a marker token written at create appears in neither the REST by-id body (`id, name, host, username, ownerId, status, createdAt, updatedAt`) nor the REST list — checked by substring against the raw payloads, not by reading fields.
- **Partial update is pointer-based and honest.** `authToken: ""` is refused with `"secret cannot be set to empty"`; a `nil` secret means keep. The site records the invariant it is protecting (`:302-306`): _"a rejected update must leave name, username, expiry, updatedAt, and the stored token exactly as they were, so a private-image deploy keeps using the last accepted credential (w7/044)"_.
- **That invariant holds live.** Three refused updates — empty secret, empty name, a 5000-byte name (`"name must be at most 253 bytes"`) — left `updatedAt` at its original value to the second, and a subsequent rename-only update preserved `username` while advancing `updatedAt`.
- **Delete is bound-aware.** `Delete` checks `appBoundToCredential` before removing, so a credential in use by a service is not silently pulled out from under it.
