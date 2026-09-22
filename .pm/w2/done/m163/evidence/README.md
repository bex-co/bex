# m163 live credential offboarding acceptance

The walk ran on 2026-09-21 in America/Los_Angeles (the JSONL timestamps use UTC,
2026-09-22). It exercises the production HTTP API composition and auth gate with
real PostgreSQL, Hydra, Kratos, and enforced OpenFGA. The Kubernetes client in the
temporary runner is in memory; the fixtures contain no applications or managed
databases. This proves credential and membership teardown, not Kubernetes
resource reconciliation.

## Environment

`bash scripts/dev-env.sh 2 up` failed its own preflight because the refreshed local
CAPD API endpoint at `127.0.0.1:55002` was unreachable. Docker container starts also
stalled for several minutes on the shared, overloaded host. The acceptance walk
therefore used isolated native processes, with loopback listeners and disposable
state under `/tmp/bex-w2-m163-live/`. No production dependency or other workstream's
stack was changed.

| Dependency | Version and implementation | Local listener |
| --- | --- | --- |
| Control-plane PostgreSQL | PostgreSQL 17.11 (Homebrew), real `bex` database | `127.0.0.1:55120` |
| Hydra | v26.2.0 native macOS binary, real `hydra` PostgreSQL database | Admin `52120`, public `58120` |
| Kratos | v26.2.0 official macOS arm64 SQLite binary, in-memory database | Admin `57120`, public `51120` |
| OpenFGA | v1.21.0 native Darwin arm64 binary, in-memory store, preshared-key auth | `127.0.0.1:63120` |
| bex-api | Production `api.NewServer` and feature services; temporary runner | `127.0.0.1:54020` |

OpenFGA used the repository's `deploy/gitops/authz/model.json`. The runner wires the
real tenant resolver, key binder, role granter/revoker, Ory identity/session cleaner,
durable OAuth revocations, and account deletion worker. It does not use an
allow-all authorization checker. The fixture proves this directly: deleting an
otherwise valid key's FGA tuple, while leaving its SQL binding intact, makes a
fresh HTTP capability check return **403**; restoring that tuple returns **200**.

Human identities are synthetic `@example.test` identities created through the
local Kratos admin API with verified email addresses and random passwords. Native
Kratos login then issues real session tokens. Workspace creation, invitations,
key creation, member removal, self-leave, and account deletion all use actual
bex HTTP requests. Tokens, passwords, client secrets, database credentials, and
the FGA preshared key are omitted from the recorded evidence.

## Reproduction

[`api_runner.go`](api_runner.go) is the exact temporary API entrypoint. Copy it
under `lego/backend/cmd/` while building so Go's `internal` import rules apply:

```sh
mkdir -p lego/backend/cmd/.w2-m163-acceptance
cp .pm/w2/done/m163/evidence/api_runner.go lego/backend/cmd/.w2-m163-acceptance/main.go
(cd lego/backend && GOWORK=off go build -p 2 -o /tmp/bex-w2-m163-api ./cmd/.w2-m163-acceptance)
```

The private environment file supplies the following contract (secret values are
generated for this run and are never checked in):

```text
BEX_CP_DB_URI=postgres://postgres@127.0.0.1:55120/bex?sslmode=disable
BEX_OPENFGA_URL=http://127.0.0.1:63120
BEX_OPENFGA_TOKEN=<generated local preshared key>
BEX_HYDRA_ADMIN_URL=http://127.0.0.1:52120
BEX_KRATOS_URL=http://127.0.0.1:51120
BEX_KRATOS_ADMIN_URL=http://127.0.0.1:57120
BEX_API_ADDR=127.0.0.1:54020
BEX_OAUTH_ISSUER=http://127.0.0.1:58120/
```

The issuer's trailing slash must match Hydra's discovery/introspection value.
The API correctly rejected initial fixture tokens when this private configuration
omitted the slash; the configuration was corrected before the recorded offboarding
walks.

PostgreSQL uses a dedicated data directory initialized with `initdb`, listening
only on loopback. Hydra runs `migrate sql -e --yes` against its separate `hydra`
database, then `serve all --dev`; its issuer is the local public URL above. Kratos
runs `serve --dev --watch-courier=false` with `dsn: memory`, the two loopback ports,
a password-enabled identity schema with email identification and verification,
and random cookie/cipher secrets. OpenFGA runs with `--datastore-engine memory`
and preshared-key authentication. The probe installs the repository model.

```sh
set -a
. /tmp/bex-w2-m163-live/runner.env
set +a
/tmp/bex-w2-m163-api
# In a second shell, from the repository root:
python3 .pm/w2/done/m163/evidence/live_acceptance.py \
  --env-file /tmp/bex-w2-m163-live/runner.env
```

The probe uses a separate Pro workspace per exit scenario, leaving production
issuance quotas and burst limits enabled. Each departing member creates two keys
in the workspace being exited and one in their separate personal workspace. The
remaining admin creates a fourth, independent key. Every key is exercised before
the exit to populate authentication and authorization caches.

## Before-fix finding

The original source at `a6946aa5408b9c8d6ec066c095acea070dbf15a0` removed every
target key's Hydra client, membership row, and FGA tuple, but its already-cached
Hydra identity reached the permission layer and returned **403**, rather than the
milestone's required **401**. There was no successful post-removal resource access
in this walk. The original strict probe stopped at this mismatch; see
[`before-cached-token.jsonl`](before-cached-token.jsonl), records 61–66.

A private diagnostic copy then accepted either 401 or 403 only to collect all
remaining teardown observations. Its results are explicitly marked
`diagnostic_scenario_completed`, never `scenario_passed`, in
[`before-diagnostic.jsonl`](before-diagnostic.jsonl). All three routes produced
immediate 403s for previously used target tokens, while the actual credential,
membership, scope, and remaining-member state converged correctly. Final
acceptance uses the strict checked-in probe, which requires 401.

## Final strict acceptance — passed

[`live-results.jsonl`](live-results.jsonl) contains 235 sanitized records from the
successful run at `2026-09-22T06:21:36Z`–`06:21:45Z`. The tested source is base
`a6946aa5408b9c8d6ec066c095acea070dbf15a0` plus the closeout fix in
`lego/backend/internal/api/auth.go` and `lego/backend/internal/store/store.go`;
the recorded backend diff SHA-256 is
`bd57cac8815786ca4358fc7c80913573821f65a87fe38f10099392baa82c730a`.

Unbinding a key now atomically records its durable revocation alongside deleting
its membership binding. The HTTP auth gate rejects that revoked machine identity
even when its token was already cached. The rebuilt runner then passed the
original, strict 401 assertions without relaxing them.

| Exit | HTTP mutation | Both previously used target keys | Hydra clients / SQL bindings / FGA tuples | Departing member's other-workspace key | Remaining member's key |
| --- | --- | --- | --- | --- | --- |
| Admin removal | `DELETE /v1/workspaces/{id}/members/{subject}` → 204 | 401 / 401 | 404 / 0 / 0 for each key | 200, client and binding retained | 200, client and binding retained |
| Self-leave | `DELETE /v1/workspaces/{id}/members/me` → 204 | 401 / 401 | 404 / 0 / 0 for each key | 200, client and binding retained | 200, client and binding retained |
| Account deletion | `DELETE /v1/users` → 202; worker reaches `done` | 401 / 401 | 404 / 0 / 0 for each key | 401, client / binding / tuple removed globally | 200, client and binding retained |

The two target-key checks completed 48 ms after warming for admin removal, 25 ms
for self-leave, and 2,542 ms for account deletion, including its asynchronous
worker. No test waited for the 30-second positive-cache lifetime to expire. Each
route also removed the human's workspace membership row and FGA tuple; the
workspace API-key list omitted the revoked keys and retained the other member's
key. Account deletion additionally removed the real Kratos identity (admin GET
returned 404).

## Cleanup

The four unused Docker containers created during the first local bring-up attempt
were removed successfully: `bex-w2-m163-fga`, `bex-w2-m163-pg`,
`bex-w2-m163-hydra`, and `bex-w2-m163-kratos`. The native acceptance API, Hydra, Kratos, and acceptance OpenFGA processes were
stopped after the successful walk. PostgreSQL and a separate test-only OpenFGA
process are retained for the next workstream item's isolated integration tests;
the root drain owns their final teardown. Private runtime files remain under
`/tmp/bex-w2-m163-live/`. No `.env` or kubeconfig is
part of this evidence.
