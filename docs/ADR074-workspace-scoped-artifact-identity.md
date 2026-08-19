# ADR074: workspace-scoped registry and static-prefix identity

**Status:** Accepted (w2/m75) · **Date:** 2026-08-18

Numbered ADR074 to resolve the collision with [ADR073-security-review-round15](ADR073-security-review-round15.md), which landed on `main` while this was in flight.

Closes the code half of [ADR055](ADR055-security-review-round4.md) F2/F3: workspace-local `App.Name` must not be a global identity at the two remaining shared sinks (Zot repositories/users/ACLs, static-site S3 prefixes). Production cutover of existing artifacts stays runbook-gated ([registry-static-identity-migration](runbooks/registry-static-identity-migration.md)).

## Decision

Every App that carries the `app.bex.co/workspace` label (`tea-<xid>`, [ADR020](ADR020-identifiers.md)) mints **workspace-scoped** identities. An App without the label (legacy / hand-applied) keeps the pre-m75 bare-name identities byte-identically. Dual-read keeps already-published artifacts reachable until a verified migration tombstones the legacy location.

The Go derivation lives in **one** place: `lego/operator/internal/identity`. Build, registry, publish, static-server, finalizer purge, and the migration tool all consume it. Call sites must not concatenate `app.Name` into a repo, username, Secret name, or object-key prefix.

## Formats

Workspace id `W` is the `app.bex.co/workspace` label (`tea-` + 20-char xid, DNS-1123). App name `A` is `metadata.name` (already a Kubernetes name). An empty `W` selects the legacy column.

| Identity | Legacy (no workspace label) | Workspace-scoped |
| --- | --- | --- |
| OCI repository | `A` | `W/A` |
| Image tag | `gen-N` (unchanged) | `gen-N` (unchanged) |
| Image reference | `<registry>/A:gen-N` | `<registry>/W/A:gen-N` |
| Zot htpasswd user | `app-A` | `app-W-A` |
| Pull Secret | `reg-pull-A` | `reg-pull-W-A` |
| Static object prefix | `A/<revision>/` | `W/A/<revision>/` |
| Future build-cache repo (w7/m86) | `A-cache` | `W/A-cache` |

`CacheRepo` appends `-cache` to the **last path component** (`identity.Repo()+"-cache"`), so a workspace-scoped cache is `tea-…/hello-cache` and not a third path segment. w7/m86 consumes this helper; this ADR does not mint cache repositories.

### Length and charset

- `W` and `A` are DNS-1123 labels (lowercase alphanumeric + hyphen). The OCI distribution name grammar accepts each as a path component; `/` is the repository separator.
- Kubernetes Secret names are DNS subdomains, max 253. `reg-pull-` + `W` + `-` + `A` is 9+24+1+|A|. If that exceeds 253, the helper truncates to a stable `prefix-<12-hex>` binding the full identity tuple (same pattern as build Job names). Htpasswd usernames have no Kubernetes cap; they use the untruncated `app-W-A` form (colon is illegal in the username and is not generated).
- An App whose workspace label is empty **always** uses the legacy column, even if a later migration will stamp the label.
- **Accepted residual — htpasswd username join.** `app-W-A` concatenates with `-`. A hand-applied CR whose `metadata.name` is literally `<ws>-<name>` can mint the same username as a labeled App `name` in workspace `ws`. The OCI repo path uses `/` and is unambiguous. This is the same threat class this ADR already scopes to legacy/hand-applied CRs (store-managed names are `<ws>-<name>`). Do not change the join without a user-migration.

## Dual-read

**Registry.** A labeled App's new builds push to `W/A`. Existing Deployments keep their baked-in `<registry>/A:…` refs until the next rollout — those blobs stay in the legacy repo. The labeled App's new user is granted **read** on the legacy repo `A` as a second Zot policy (existing policies are not replaced), so kubelet pulls of old refs succeed with the new Secret. After tombstone that extra grant is removed. An unlabeled App is never granted onto a sibling's workspace-scoped repo.

**Static.** A revision prefix is immutable. The static-server does **not** probe S3 twice. `App.status.staticPrefix` records the prefix the current `activeRevision` was published under. When unset (every pre-m75 site), the resolver falls back to the legacy `A/<revision>/`. A new publish of a labeled App writes the workspace-scoped prefix into status and serves from it. Reconcile of an already-active revision **must not** invent a prefix: an upgrade-time labeled App whose objects still live at `A/<rev>/` keeps `status.staticPrefix` empty until a publish or the migration tool records the new location.

**Unlabeled Apps** stay on the legacy column for both sinks. No dual-read grant is added.

## Tombstone

The migration tool, after copy+verify, leaves:

1. Annotation `app.bex.co/identity-tombstone=true` on the App (operator dual-read off; new writes only to the workspace-scoped location).
2. S3 object `A/.bex-tombstone` (JSON: `{migratedTo, at}`) at the **legacy** prefix root. Dual-read of static no longer consults `A/` once status.staticPrefix points at `W/A/…`.
3. Zot tag `bex-tombstone` on the legacy repository, pointing at an already-copied digest (no new blob). The tag is a marker, not a deletion.

Tombstone means **no new writes + dual-read stops**. Legacy image blobs and S3 objects are **not** deleted here. Deletion is phase 4 of the runbook, after Deployments have rolled onto the new refs and dual-read hits are zero.

A tombstone is refused when another live App still keys the same legacy repo or prefix (the original collision). Copy to the destination may still proceed; only the shared legacy location stays authoritative for the sibling.

## Mapping

| From (legacy) | To (workspace-scoped) |
| --- | --- |
| repo `A`, tags `gen-N` | repo `W/A`, same tag names, digest-preserving copy |
| user `app-A`, Secret `reg-pull-A` | user `app-W-A`, Secret `reg-pull-W-A` (newly minted; legacy user retired at tombstone if this App owned it) |
| prefix `A/<rev>/` | prefix `W/A/<rev>/`, object-count + ETag/size verified per key |

Idempotence: a destination tag/object whose digest/ETag already matches is skipped. A mismatch aborts that App; the legacy location stays authoritative.

## Why workspace id, not App UID

ADR055's sketch mentioned UID-scoped repos. The workspace id is the tenant boundary ([ADR043](ADR043-tenant-namespace-isolation.md), [ADR020](ADR020-identifiers.md)), already on every store-projected App, DNS-safe, and stable across an App's lifetime. UID in the path would churn the repo on every recreate and is already the **ownership** key (`app.bex.co/app-uid`) on build-namespace objects. The collision class is two tenants with the same `metadata.name`; scoping by workspace is necessary and sufficient. Store-managed CR names are already `<ws>-<name>` ([ADR067](ADR067-security-review-round12.md) finding 1) — the scheme still applies so hand-applied / never-renamed CRs cannot collide at the sink.

## Consequences

- New labeled Apps are isolated at both sinks immediately.
- Running unlabeled (and pre-migration labeled) artifacts keep working.
- w7/m86 `-cache` repos are born workspace-scoped instead of doubling the migration surface.
- F2/F3 stay **prod-cutover-gated** until the runbook's phase 4; the residual register records "code landed".
