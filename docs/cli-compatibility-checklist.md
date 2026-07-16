# Render CLI compatibility

bex supports the official, unmodified [Render CLI](https://render.com/docs/cli) through its Render-compatible REST API. Most commands work today; interactive `render login` is the main authentication gap.

Legend: ✅ supported · ◐ supported with a limitation · ✖ broken · — deliberate non-goal.

**Maintenance Mode applicability.** The pinned CLI has neither a maintenance-mode command nor a generic service PATCH flag for it, so there is no unmodified CLI case to grade. See [the cross-surface evidence](render-artifacts/maintenance-mode.md).

## Set up the Render CLI with production bex

Until browser login is implemented, install the official CLI using Render's [installation instructions](https://render.com/docs/cli#1-install-or-upgrade), then create an API key in the bex dashboard under **Settings → API Keys**. Save both values shown at creation:

- key ID (`client_id`)
- key secret (`client_secret`), which is shown only once

From a bex checkout, the setup helper prompts for those values, discovers the workspace, and exports the CLI configuration into the current shell:

```sh
source ./scripts/setup-render-cli.sh
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

`render login` is unnecessary for this CI flow. The CLI sees `RENDER_API_KEY` and treats the process as authenticated. `render logout` cannot revoke an environment variable; revoke or rotate the underlying key from the bex dashboard.

## Compatibility summary

The following commands were exercised against live bex environments with real Services, Postgres databases, and Key Value instances.

| Commands | Status | Notes |
| --- | --- | --- |
| `login` (browser/device flow) | ✖ | The official CLI uses a fixed public OAuth client, `/device-grant`, `/device-token`, refresh tokens, and `/token/refresh/`. bex does not yet provide that compatibility flow; `RENDER_API_KEY` only bypasses it. Tracked by `w4/m25`. |
| `logout` | ◐ | Correctly explains that it cannot clear `RENDER_API_KEY`. Real device-token logout and refresh-chain revocation depend on the missing browser login flow. The OAuth-credential revoke path (`POST /v1/oauth/revoke`) is implemented and regression-guarded: `scripts/cli-compat.sh verify` mints a throwaway key, drives the unmodified CLI's real `render logout`, and proves the underlying Hydra client is deleted. |
| `whoami` | ✅ | Returns the key owner's email when the Kratos admin URL is configured. |
| `workspace current`, `workspaces` | ✅ | Return the caller's real `tea-…` workspace. |
| `workspace set` | ◐ | The interactive flow was not tested. `RENDER_WORKSPACE` is the supported deterministic override and is used by the production setup above. |
| `projects`, `environments <project-id>` | ✅ | Lists projects and returns Render-compatible environment records. |
| `services`, `services create`, `services update`, `services delete` | ✅ | List and lifecycle operations return the expected Render wire shapes. The upstream CLI does not support `--num-instances` on update; supported fields such as `--health-check-path` work. |
| `services instances` | ✅ | Returns live, non-terminal Deployment pods; suspended services return an empty list. |
| `postgres create`, list, get, update, suspend, resume, delete | ✅ | Includes plan, disk, high-availability, and IP allow-list changes. Allow-list entries round-trip as `{cidrBlock, description}` objects — per-entry descriptions persist and come back on every read (w4/m24; previously accepted but dropped). |
| `postgres update --name` | ✅ | Rename preserves the immutable `dpg-…` ID and Kubernetes identity. Production passed on 2026-07-15 using digest `ba32bf76ab6e` (deploy run `29406643202`): the legacy backfill preserved all 11 recorded UIDs, and a fresh resource passed the official-CLI identity smoke test. |
| `keyvalues create`, list, get, update, suspend, resume, delete | ✅ | Owner, options, persistence policy, and structured IP allow-list fields round-trip correctly — including per-entry allow-list descriptions (w4/m24; previously accepted but dropped). |
| `deploys list`, `deploys create`, `deploys cancel` | ✅ | Deploy creation accepts the CLI's cache enum and returns Render-compatible deploy records. |
| `restart`, `logs` | ✅ | Accept typed service IDs and public names. Logs return stable cursors even when the result is empty. |
| `jobs list`, `jobs create`, `jobs cancel` | ✅ | One-off jobs use the service image and return the expected job shape. |
| `blueprints validate` | ✅ | Accepts the CLI's multipart Blueprint request. The plan is a declaration summary, not Render's field-level diff; Blueprint Key Value entries are not supported. |
| `psql --command` | ◐ | Reaches managed Postgres, but end-to-end access requires `BEX_DB_DOMAIN` and an IP allow-list that permits the caller. |
| `docs` | ✅ | Opens Render's documentation; it makes no bex API call. |
| `skills` | ✖ | The tested CLI revision panics before making an HTTP request. This is an upstream CLI issue, not a bex API incompatibility. |
| `workflows list` | — | Render Workflows is a deliberate bex non-goal. |
| `ssh` | ◐ | **Running-instance SSH is implemented by w2/m39:** the CLI-required `sshAddress`, live-deploy data, instances route, and raw OpenSSH gateway contract are present and protocol-tested. The CLI is interactive by design, so this remains ◐ until `scripts/ssh-verify.sh` records the public TCP/22 PTY run. The current official release, v2.21.0 at `c398207`, accepts a service name, service id, or full instance id, but both instance-picker callbacks assign the service id instead of the selected instance id (`cmd/ssh.go`); the verifier therefore proves name resolution and exact targeting through the supported direct-instance-id argument without patching the CLI or claiming that broken menu works. `--ephemeral` remains a deliberate non-goal. [ADR035](ADR035-ssh.md). |
| `kv-cli`, `pgcli` | — | Interactive datastore clients remain outside the current compatibility target. |
| `ea` (`objects`, `sandbox`, `sandbox-groups`) | — | These early-access surfaces are outside the current compatibility target. |

## Verification

The compatibility pass used `render-oss/cli` at `c23438e` on 2026-07-15 without patches. Raw transcripts are in [`.pm/w9/done/m2/evidence/log.md`](../.pm/w9/done/m2/evidence/log.md).

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

Postgres rename has a focused smoke test in [`scripts/postgres-rename-cli-smoke.sh`](../scripts/postgres-rename-cli-smoke.sh). Run [`scripts/postgres-name-migrate.sh`](../scripts/postgres-name-migrate.sh) first for legacy records; [ADR009](ADR009-postgresql-management.md#rename-and-legacy-migration) documents the rollout and rollback order. Production used that sequence through deploy run `29406643202` and digest `ba32bf76ab6e`.

The cross-surface backlog remains in [ADR018-render-parity.md](ADR018-render-parity.md); this document does not duplicate ownership tracking.
