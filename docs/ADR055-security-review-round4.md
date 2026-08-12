# Security review round 4: codex-security repository scan (2026-08-10)

**Status:** partially implemented · **Scope:** disposition of the 12 findings (6 high, 6 medium) from the codex-security static repository scan against revision `1b1e5dcd`. Fourth pass in the ADR028 → w1/m53 → ADR045 → this lineage. Eight findings are fixed in place with tests; four require coordinated data migration, upstream/infra work, or an external process and are tracked here as deliberate deferrals rather than silent skips.

## Root-cause note

Five of the six high findings (F1–F5) share one root cause: a **workspace-local `App.Name` used as a global identity at a shared sink**. Because bex runs one namespace per workspace (ADR043), two tenants can own the same App name; any sink keyed on the bare name (S3 prefix, Zot repo/user/ACL, reserved-host exemption, cross-namespace secret copies) collides across tenants. The immutable identities that DON'T collide already exist — the globally-unique `spec.subdomain` slug and the App UID — and the fixes below move each sink onto one of them.

## Disposition summary

| # | Finding | Severity | Disposition |
| --- | --- | --- | --- |
| F5 | Service-name alias bypasses the reserved platform-host guard | High | **Fixed in place** — reserved-host exemption now compares `spec.subdomain`, not the CR name |
| F6 | Production accepts audience-less human OAuth tokens from third-party clients | High | **Fixed in place** — fail-closed startup guard + manifest enforces `BEX_OAUTH_REQUIRE_AUDIENCE=1` (bootstrap-first) |
| F1 | Shared backup credentials mountable by a co-located tenant App | High | **Fixed in place** — projected backup creds labeled protected; App reconcile rejects references to them |
| F4 | Tenant pre-deploy jobs can mount platform secrets from the shared build namespace | High | **Fixed in place** — mirror fails closed on a missing source that collides with a foreign build-namespace secret |
| F8 | Concurrent webhook endpoint creation bypasses the per-workspace quota | Medium | **Fixed in place** — transaction-scoped advisory lock keyed on tenant serializes the empty-set race |
| F12 | Retained terminal-session pods can mint fresh repository write tokens | Medium | **Fixed in place** — mint loads the session and refuses a terminal/foreign/sandbox-cleared session |
| F11 | Request bodies fully buffered before authentication and rate limiting | Medium | **Fixed in place** — body cap moved per-route, inside each cheap admission (auth/rate/IP limiter) |
| F10 | Cluster-admin bootstrap trusts a first-seen SSH host key when the pin is absent | Medium | **Fixed in place** — `BEX_SSH_REQUIRE_KNOWN_HOSTS=1` in CI fails closed when the pin is missing |
| F2 | Static-site object prefixes / purge targets omit tenant identity | High | **Deferred — migration-gated** (see below) |
| F3 | Registry identities and repositories keyed only by App name across tenants | High | **Deferred — migration-gated** (see below) |
| F7 | Mutable image tags execute with source/registry/object-store credentials | Medium | **Deferred — infra work** (see below) |
| F9 | Tenant sites share `onbex.co` without browser-enforced cookie isolation | Medium | **Accepted risk — external (PSL)** (see below) |

## Fixed in place

- **F5** — `lego/backend/internal/apps/domains.go`: `reservedHost` now takes the App's own `<slug>.<base>` platform host (from `ownPlatformHost`, derived from `spec.subdomain`), not the caller-supplied `<appName>.<base>`. `ensureHostsClaimable` (create/blueprint/deploy-manifest) and the maintenance-URI guard were updated the same way. Regression test: `TestReservedHostExemptsSlugNotAppName` (slug ≠ CR name is the exploit condition).
- **F6** — `cmd/api/main.go`: in the multi-tenant posture (control-plane store on), an OAuth resource configured with audience enforcement off now **fails startup** unless `BEX_ALLOW_INSECURE_AUTHZ=1` (the same override the OpenFGA fail-closed uses). `config/api/deployment.yaml` flips `BEX_OAUTH_REQUIRE_AUDIENCE` to `"1"`. **Prerequisite:** `scripts/auth-bootstrap-client.sh` must have stamped the official Render CLI + bex-mobile clients with `bex.co/platform-client` before this deploys, or their legitimately audience-less device-flow logins are refused (docs/ADR012-auth.md §7).
- **F1** — `lego/operator/internal/execution/security.go` defines `LabelProtectedFromTenantMount`; `database_controller.go`'s `projectBackupCredential` (shared by Database + KeyValue) stamps it on the projected S3 backup credential; `app_controller.go`'s `rejectProtectedSecretRefs` fails the App reconcile if any `envFromSecret(s)` / `filesFromSecrets` / `env[].secretKeyRef` names a protected Secret. Tests: `TestRejectProtectedSecretRefs`.
- **F4** — `app_controller.go`'s `mirrorPreDeploySecrets`: a tenant-referenced secret absent in the App namespace no longer silently skips. If a same-named Secret already occupies the shared build namespace and is not owned by this App, the reconcile fails closed (the pre-deploy Job can no longer bind a platform secret by name-collision). Absent-in-both stays the tolerated optional case. Test: `TestMirrorPreDeploySecretsFailsClosedOnForeignCollision`.
- **F8** — `lego/backend/internal/store/webhooks.go`: `CreateWebhookEndpoint` takes `pg_advisory_xact_lock(hashtext(tenant_id))` before the count+insert. The prior `SELECT … FOR UPDATE` over existing rows locked nothing for an empty/sub-limit workspace, so two parallel creates both passed the cap.
- **F12** — `lego/backend/internal/agentsession/mint.go`: `Minter` gained a `SessionStore`; `Mint` loads the durable session and returns `ErrForbidden` unless it is non-terminal (`completed`/`failed`/`canceled` denied), in the request's workspace, and still holds a live sandbox. Tests: `TestMinterRefusesTerminalOrForeignSession`.
- **F11** — `lego/backend/internal/api/server.go`: `withBodyLimit` is applied per route inside `rootMux` (after each webhook/deploy-hook IP limiter, and inside `auth(rl(…))` for the `/v1`,`/graphql`,`/mcp` wildcards) instead of as a blanket outer wrapper, so an unauthenticated or throttled caller is shed before the server buffers up to `MaxBodyBytes`.
- **F10** — `scripts/lib/ssh-hostkey.sh` honours `BEX_SSH_REQUIRE_KNOWN_HOSTS`: when set and no pinned known-hosts file is present it fails closed instead of `accept-new`. Both admin.conf-fetching workflows (`deploy.yml`, `openbao-restore-drill.yml`) set it, making the pin mandatory in CI. **Prerequisite / open question:** the `BEX_SSH_KNOWN_HOSTS` secret must be provisioned in every CI environment, or these workflows now abort (the intended forcing function).

## Deferred, with rationale

- **F2 (static object keys) and F3 (registry keys) — migration-gated.** The correct fix keys each shared sink on immutable workspace + App UID (e.g. `workspace/uid/revision` for S3; UID-scoped Zot repo/user/ACL). Both are **destructive to change blindly**: re-keying the S3 prefix orphans every already-published static site until it redeploys, and re-pathing the registry repo invalidates the image ref baked into every running Deployment (ImagePullBackOff until a rebuild+re-push). A migration-free ownership guard is not available either — the Zot htpasswd/ACL entries carry no owner record, and scoping only the username still leaves both tenants sharing the `<name>` repo. These require a coordinated rollout: (1) add the new scheme with dual-read fallback, (2) run a migration job over existing objects/repos, (3) redeploy tenants, (4) drop the fallback. Tracked for a dedicated milestone; **not** landed in this pass to avoid a live-tenant outage.
- **F7 (mutable image tags) — infra work.** Pinning `moby/buildkit` / `alpine/git` / `amazon/aws-cli` to reviewed multi-arch digests requires resolving and verifying real digests (guessing them is worse than a tag), and replacing the runtime `apk add age` needs a purpose-built digest-pinned image published to the platform registry. Exploitation requires an upstream registry/tag compromise. Deferred to a supply-chain hardening milestone; do it alongside admission-time signature/attestation verification.
- **F9 (`onbex.co` cookie isolation) — accepted risk (external).** `onbex.co` is not on the browser Public Suffix List, so sibling tenant sites can set `Domain=onbex.co` cookies at each other. The manager already logs this and continues by design (owner-decided in ADR045 round 3, PSL waived). The only real fixes are a PSL submission (external, multi-week) or moving tenants to per-tenant registrable domains (major architecture change); fail-closing the validator in production would take down **all** tenant static/web hosting under the base domain and is therefore not an autonomous change. Remains tracked as accepted risk.

## Open questions carried forward

- Is `BEX_SSH_KNOWN_HOSTS` provisioned in every production CI environment? (F10 now makes its absence fail the deploy — verify before the next run.)
- Are all agent-connected repositories protected against direct writes to default/release branches? (F12 bounds token lifetime to active sessions; branch protection is still delegated to GitHub per ADR047.)
- Have the shared S3 backup credentials been replaced by per-resource, prefix-scoped credentials in the live environment? (F1 blocks the tenant-mount vector; per-resource scoping is the deeper F2/F3-adjacent follow-up.)
