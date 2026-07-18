# Render CLI compatibility

bex supports the official, unmodified [Render CLI](https://render.com/docs/cli) through its Render-compatible REST API. Most commands work today, including interactive browser login, verified live against production 2026-07-15.

Legend: ✅ supported · ◐ supported with a limitation · ✖ broken · — deliberate non-goal.

**Maintenance Mode applicability.** The pinned CLI has neither a maintenance-mode command nor a generic service PATCH flag for it, so there is no unmodified CLI case to grade. See [the cross-surface evidence](render-artifacts/maintenance-mode.md).

## Set up the Render CLI with production bex

For automation, install the official CLI using Render's [installation instructions](https://render.com/docs/cli#1-install-or-upgrade), then create an API key in the bex dashboard under **Settings → API Keys**. Save both values shown at creation:

- key ID (`client_id`)
- key secret (`client_secret`), which is shown only once

From a bex checkout, the setup helper prompts for those values, discovers the workspace, and exports the CLI configuration into the current shell:

```sh
source ./setup-render-cli.sh
```

bex API keys are OAuth clients, not static bearer tokens. Exchange the key pair at the public Hydra endpoint, then give the resulting short-lived bearer token to the Render CLI:

```sh
export BEX_API_KEY_ID='<key-id>'
export BEX_API_KEY_SECRET='<key-secret>'
export RENDER_WORKSPACE='<tea-workspace-id>'

export RENDER_HOST='https://api.bex.co/v1/'
export RENDER_API_KEY="$(
  curl --fail --silent --show-error \
    --request POST 'https://oauth.bex.co/oauth2/token' \
    --data-urlencode 'grant_type=client_credentials' \
    --data-urlencode "client_id=$BEX_API_KEY_ID" \
    --data-urlencode "client_secret=$BEX_API_KEY_SECRET" |
    jq --exit-status --raw-output '.access_token'
)"

render whoami
render services
```

Replace `api.bex.co` and `oauth.bex.co` if the production installation uses different public domains. To find the workspace ID, omit `RENDER_WORKSPACE`, run `render workspaces`, and copy the desired `tea-…` ID.

The production access-token lifetime is 15 minutes. Exchange the key pair again before a later command or at the start of every CI job. Do not pass the key secret directly as `RENDER_API_KEY`, commit either credential, or print them in CI logs.

For CI, store the key ID, key secret, and workspace ID in the CI provider's secret store and perform the exchange and CLI call in one step:

```sh
set -eu

export RENDER_HOST='https://api.bex.co/v1/'
export RENDER_WORKSPACE="$BEX_WORKSPACE_ID"
export RENDER_API_KEY="$(
  curl --fail --silent --show-error \
    --request POST 'https://oauth.bex.co/oauth2/token' \
    --data-urlencode 'grant_type=client_credentials' \
    --data-urlencode "client_id=$BEX_API_KEY_ID" \
    --data-urlencode "client_secret=$BEX_API_KEY_SECRET" |
    jq --exit-status --raw-output '.access_token'
)"

render deploys create "$BEX_SERVICE_ID" --confirm
```

`render login` is unnecessary for this CI flow. The CLI sees `RENDER_API_KEY` and treats the process as authenticated. `render logout` cannot revoke an environment variable; revoke or rotate the underlying key from the bex dashboard. Human browser login is a separate device flow: one permanent secretless platform client issues expiring access tokens and rotating refresh tokens.

## Compatibility summary

The following commands were exercised against live bex environments with real Services, Postgres databases, and Key Value instances.

| Commands | Status | Notes |
| --- | --- | --- |
| `login` (browser/device flow) | ✅ | The official CLI (pinned `c23438e`) completed real Chrome → Kratos → consent device login against **production** (`api.bex.co`) with `RENDER_API_KEY` unset, 2026-07-15 (`w4/m27`, `scripts/render-cli-auth-e2e.sh`; deployed image `sha256:d1681b2b…238dc19`): human personal workspace resolved, forced `api.expires_at` expiry rotated both the access and refresh tokens on the next unmodified command. **Bad-key 401 dialect (w9/m39/t003, 2026-07-15, dev-9):** with a garbage `RENDER_API_KEY`, `render whoami` now surfaces `Error: failed to get current user: unauthorized` — the `message` field of the m38-unified Render-shaped 401 (`{"error":"unauthorized","id":"unauthorized","message":"unauthorized"}`, `application/json`) verbatim, rather than the generic "unknown error" the old text/plain body produced. |
| `logout` | ✅ | Production E2E (2026-07-15) proves access-token rejection (within the documented ≤30s per-replica introspection-cache window — Hydra-side revocation is immediate), refresh-chain `invalid_grant`, shared-client preservation, and second-user continuity. The machine override still requires dashboard key revocation. |
| `whoami` | ✅ | Returns the key owner's real name and email (w4/m25). The key's `created-by` binding resolves to the minting user's Kratos identity, so an API-key caller sees the same `Name:`/`Email:` as a session caller; re-verified live 2026-07-15 against dev-4 (`render whoami` → `Name: Ada Lovelace / Email: ada@dev4.test` for both a session-derived token and a `client_credentials` API-key token). A key with no resolvable owning human degrades to the workspace's earliest-admin email with an empty name. Requires the Kratos admin URL. |
| `workspace current`, `workspaces` | ✅ | Return the caller's real `tea-…` workspace. |
| `workspace set` | ✅ | **Live-verified (2026-07-15, official CLI v2.21.0, dev-9, w9/m39/t004):** `render workspace set <tea-…>` → `Workspace set to: tea-…`, and `render workspace current` reflects the persisted selection on the next command — the full set→persist→read round-trip works against bex-api. The no-arg **interactive picker** is a client-side TUI that the CLI itself auto-degrades to text on a non-TTY (its `-o` default), and it consumes only the already-✅ `render workspaces` list plus a local CLI-config write — no bex-api call the deterministic path doesn't already exercise, so no wire gap. `RENDER_WORKSPACE` remains the supported deterministic override used by the production setup above. |
| `projects`, `environments <project-id>` | ✅ | Lists projects and returns Render-compatible environment records. **Populated pagination proof (2026-07-15, official CLI v2.21.0):** the unmodified CLI walked 101 projects through the real bex projects REST handler in exactly two requests (100 + 1), terminated, and emitted every id once; `TestOfficialCLIProjectsWalksPopulatedPages` is the opt-in executable capture. The ordinary backend suite independently walks 41 records at the default page size (20 + 20 + 1), asserts no duplicates, and verifies the final cursor produces an empty tail. |
| `services`, `services create`, `services update`, `services delete` | ✅ | List and lifecycle operations return the expected Render wire shapes. `services create --image … --registry-credential rgc-…` binds the named workspace credential and reaches a Running App; `scripts/cli-compat.sh registry-credential-verify` is the self-cleaning live leg. For a repository Docker service, the CLI's same flag emits `serviceDetails.envSpecificDetails.registryCredentialId`; bex now accepts and echoes that exact shape, and the build-plane live proof covers authenticated private `FROM` plus a wrong-credential `401` control. The upstream CLI does not support `--num-instances` on update; supported fields such as `--health-check-path` work. |
| `services instances` | ✅ | Returns live, non-terminal Deployment pods; suspended services return an empty list. |
| `postgres create`, list, get, update, suspend, resume, delete | ✅ | Includes plan, disk, high-availability, and IP allow-list changes. Allow-list entries round-trip as `{cidrBlock, description}` objects — per-entry descriptions persist and come back on every read (w4/m24; previously accepted but dropped). |
| `postgres update --name` | ✅ | Rename preserves the immutable `dpg-…` ID and Kubernetes identity. Production passed on 2026-07-15 using digest `ba32bf76ab6e` (deploy run `29406643202`): the legacy backfill preserved all 11 recorded UIDs, and a fresh resource passed the official-CLI identity smoke test. |
| `keyvalues create`, list, get, update, suspend, resume, delete | ✅ | Owner, options, persistence policy, and structured IP allow-list fields round-trip correctly — including per-entry allow-list descriptions (w4/m24; previously accepted but dropped). |
| `keyvalues update --name` | ✅ | **Live official-CLI proof captured (2026-07-15, v2.21.0, dev-9):** `keyvalues create --name loop-kv-probe` minted `red-d9c4qdpjg4raahi17kc0`; `keyvalues update --name loop-kv-renamed` preserved that id and every recorded Kubernetes object identity (no recreation, `keyvalue-rename-verify.sh compare` green); `keyvalues get loop-kv-renamed` re-resolved the new name to the same `red-…` id. Rename is the KeyValue mirror of `postgres update --name`, on the identical mechanism. New stores mint `red-<xid>` as `metadata.name` with the mutable name in `spec.name`; `PATCH /v1/key-value/{id}` accepts a `name` field so the unmodified CLI routes by the opaque id and survives. Backed by one core (`UpdateKeyValue`) shared with GraphQL `renameKeyValue` / MCP `rename_key_value` / dashboard. Reproduce: `bash scripts/keyvalue-rename-cli-smoke.sh --namespace <ns> --keyvalue red-… --name <new>` (legacy stores first backfilled by [`scripts/keyvalue-name-migrate.sh`](../scripts/keyvalue-name-migrate.sh); [ADR021 §6](ADR021-keyvalue-management.md) documents rollout/rollback order). **Production smoke green (2026-07-17, w9/m40):** create → rename ×2 → get → delete of `red-d9db3fp07a5s73dj32i0` against `api.bex.co`; all five recorded Kubernetes identities (KeyValue/StatefulSet/PVC/Secret/Service) preserved across renames; backfill idempotent and complete in every dev-N namespace and all production namespaces. |
| `deploys list`, `deploys create`, `deploys cancel` | ✅ | Deploy creation accepts the CLI's cache enum and returns Render-compatible deploy records. |
| `restart`, `logs` | ✅ | Accept typed service IDs and public names. Logs return stable cursors even when the result is empty. |
| `jobs list`, `jobs create`, `jobs cancel` | ✅ | One-off jobs use the service image and return the expected job shape. |
| `blueprints validate` | ✅ | Accepts the CLI's multipart Blueprint request. The plan is a declaration summary, not Render's field-level diff. Blueprint Key Value entries are supported: `keyvalue`/`redis` service entries parse, validate, and appear in the plan's `keyValue` array. `fromService` references to a key-value store (type: keyvalue/redis) resolve to SecretKeyRefs for `connectionString`/`host`/`port`/`password`. `GET /v1/blueprints/{id}` and MCP `get_blueprint` are also implemented (w2/m41). |
| `psql --command` | ◐ | Reaches managed Postgres, but end-to-end access requires `BEX_DB_DOMAIN` and an IP allow-list that permits the caller. **Friction assessment (w9/m39/t005):** neither prerequisite is bex-specific friction to fix. (1) The **IP allow-list is parity** — Render's own managed Postgres blocks all external `psql`/`pgcli` connections until you add a source CIDR; requiring an allow-list entry is the identical Render experience, not a bex gap. (2) `BEX_DB_DOMAIN` is a **platform-install config** (an operator env, like Render running its own DB hostnames), not a per-user step: a standard bex deploy that wants external `psql` sets it once; unset it stays internal-only by design ([ADR009](ADR009-postgresql-management.md) §SNI proxy). The one theoretical improvement — clearer error text on refusal — is out of `render psql`'s path (the CLI opens a direct TCP/SNI connection to the DB host; bex-api is not a hop and cannot inject a message). **Conclusion: no bex-api work to promote;** the row stays ◐ pending only the live allow-listed connection capture, exactly as Render's would require. |
| `docs` | ✅ | Opens Render's documentation; it makes no bex API call. |
| Dashboard deep links (TUI "Open dashboard" shortcuts) | ✅ | The CLI constructs `<dashboard>/{web\|worker\|pserv\|static\|cron\|d\|r}/{id}` (+ `/deploys/{dep-id}`) URLs client-side (`pkg/dashboard/dashboard.go`) from its `dashboard_url` config (`~/.render/cli.yaml`; defaults to `dashboard.render.com`). With `dashboard_url` pointed at the bex dashboard, every shape it can mint resolves: bex serves them as redirect aliases onto the canonical routes, and bex-api's own `dashboardUrl` metadata emits the same Render shapes (w5/m39, [render-artifacts/dashboard-routes.md](render-artifacts/dashboard-routes.md)). Alias behavior is test-enforced (`dashboard/src/routes/__tests__/render-alias-routes.test.ts`, `dashboard/src/common/lib/__tests__/render-alias.test.ts`). |
| `skills` | ✖ | The tested CLI revision panics before making an HTTP request. This is an upstream CLI issue, not a bex API incompatibility. |
| `workflows list` | — | Render Workflows is a deliberate bex non-goal. |
| `ssh` | ◐ | **Running-instance SSH is implemented by w2/m39:** the CLI-required `sshAddress`, live-deploy data, instances route, and raw OpenSSH gateway contract are present and protocol-tested. The CLI is interactive by design, so this remains ◐ until `scripts/ssh-verify.sh` records the public TCP/22 PTY run. The current official release, v2.21.0 at `c398207`, accepts a service name, service id, or full instance id, but both instance-picker callbacks assign the service id instead of the selected instance id (`cmd/ssh.go`); the verifier therefore proves name resolution and exact targeting through the supported direct-instance-id argument without patching the CLI or claiming that broken menu works. `--ephemeral` remains a deliberate non-goal. [ADR035](ADR035-ssh.md). |
| `kv-cli`, `pgcli` | — | Interactive datastore clients remain outside the current compatibility target. |
| `ea` (`objects`, `sandbox`, `sandbox-groups`) | — | These early-access surfaces are outside the current compatibility target. |

## Verification

The broad compatibility pass used `render-oss/cli` at `c23438e` on 2026-07-15 without patches. The private-image credential leg uses the current unmodified CLI at `72b3fbd59068ae84d024ec2ded9df6b27dc8dd68`; `scripts/registry-credential-cli-verify.sh` first requires an anonymous pull of the supplied image to fail, then creates and cleans up its credential and service, asserts the canonical REST summary plus persisted App/Secret/Deployment binding, and requires a kubelet pull event and `Running` Pod. Raw broad-pass transcripts are in [`.pm/w9/done/m2/evidence/log.md`](../.pm/w9/done/m2/evidence/log.md).

For the local isolated harness:

```sh
bash .pm/w9/dev-9/up.sh
kubectl -n dev-9-auth port-forward service/hydra-public 59090:4444 &
bash .pm/w9/dev-9/bootstrap-key.sh
(cd cli && go build -o ../.pm/w9/dev-9/bin/render .)
scripts/cli-compat.sh workspaces -o json
scripts/cli-compat.sh verify
```

[`scripts/cli-compat.sh`](../scripts/cli-compat.sh) exchanges the development key pair, sets `RENDER_HOST`, `RENDER_API_KEY`, and `RENDER_WORKSPACE`, then runs the unmodified CLI. Its `verify` mode covers the automated compatibility assertions — every census family that makes a bex-api call, with whole-shape (`checkFields`) assertions — and trap-cleans the resources it creates (one green run recorded 2026-07-15, bex-api `a4771886`). Its `mutation-check` mode proves those assertions are non-vacuous: a response-mutating proxy ([`.pm/w9/done/m4/mutation-proxy.py`](../.pm/w9/done/m4/mutation-proxy.py)) reintroduces one previously-fixed wire-shape regression per family and requires the matching leg to fail — either the official CLI's own decode errors nonzero, or `checkFields` reports the dropped field. Browser login is deliberately not counted as verified merely because `RENDER_API_KEY` makes `render login` short-circuit as already authenticated.

The interactive-auth verifier uses two disposable users and a real Chrome process:

```sh
CREATE_TEST_IDENTITIES=1 \
  KRATOS_ADMIN_URL=http://localhost:57030 \
  BEX_API_URL=http://localhost:54030 \
  HYDRA_ADMIN_URL=http://localhost:52030 \
  scripts/render-cli-auth-e2e.sh
```

For production, omit `CREATE_TEST_IDENTITIES`, provide two existing disposable users through `CLI_USER_{A,B}_{EMAIL,PASSWORD}`, and point `HYDRA_ADMIN_URL` at a private admin port-forward. The script never prints credentials or tokens.

Postgres rename has a focused smoke test in [`scripts/postgres-rename-cli-smoke.sh`](../scripts/postgres-rename-cli-smoke.sh). Run [`scripts/postgres-name-migrate.sh`](../scripts/postgres-name-migrate.sh) first for legacy records; [ADR009](ADR009-postgresql-management.md#rename-and-legacy-migration) documents the rollout and rollback order. Production used that sequence through deploy run `29406643202` and digest `ba32bf76ab6e`.

Key Value rename has the mirror smoke test in [`scripts/keyvalue-rename-cli-smoke.sh`](../scripts/keyvalue-rename-cli-smoke.sh) (`keyvalue-rename-verify.sh` snapshots/compares the StatefulSet/PVC/Secret/Service/route identities). Run [`scripts/keyvalue-name-migrate.sh`](../scripts/keyvalue-name-migrate.sh) first for legacy records; [ADR021 §6](ADR021-keyvalue-management.md) documents the same rollout/rollback order (w9/m6).

## Sample lifecycle sweep

[`scripts/samples-lifecycle.sh`](../scripts/samples-lifecycle.sh) drives each `examples/` sample through the full Render lifecycle — deploy → verify-up → suspend → verify-down → resume → verify-up → delete → verify-gone — CLI-first, against whatever bex the `render` CLI targets (it reads `~/.render/cli.yaml`, or takes `RENDER_HOST`/`RENDER_API_KEY` like `cli-compat.sh`). Run one sample or `all`:

```sh
scripts/samples-lifecycle.sh hello-go        # one sample
scripts/samples-lifecycle.sh all             # every sample, sequentially
DRY_RUN=1 scripts/samples-lifecycle.sh all   # print the CLI/REST calls, execute nothing
```

It is an **on-demand guard** (like `cli-compat.sh verify`), never CI — it creates and deletes real services. verify-up asserts a unique per-run token (so a cached response can't pass); verify-down requires the suspended URL to stop serving for `DOWN_CONFIRM` (default 3) consecutive checks; cron is proven via a run's log line, not a URL; `RESIDUE_KUBE=1` adds pod/`reg-pull`/htpasswd residue checks when kubectl points at the target cluster. One green production run (w9/m44, 2026-07-17): hello-go, hello-node, whoami, cron-demo, stack-demo.

CLI gaps the sweep documents (each handled per-leg):

- **Service suspend/resume are dashboard-only** in Render's CLI — no `render services suspend|resume`. The harness falls back to raw REST `POST /v1/services/{id}/suspend|resume` (202, [ADR007](ADR007-restart-suspend-and-resume.md)), tagged `[via REST]`. (Postgres/Key Value **do** have CLI `suspend`/`resume`.)
- **Cron create requires `--cron-command`** even when the image supplies its own `ENTRYPOINT` — `render services create --type cron_job --cron-schedule …` alone errors `cron-command and cron-schedule are required for cron jobs`. The harness passes the image's entrypoint binary explicitly.
- **No blueprint apply** — the CLI has no `render.yaml` apply, so a Blueprint stack (web + worker + postgres) is composed leg-by-leg: `postgres create`, read its internal connection string, then two `services create` with `DATABASE_URL` wired via `--env-var` (the blueprint's `fromDatabase` secretRef has no CLI equivalent).

The cross-surface backlog remains in [ADR018-render-parity.md](ADR018-render-parity.md); this document does not duplicate ownership tracking.
