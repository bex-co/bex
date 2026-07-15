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

<<<<<<< Updated upstream
| # | Root cause | Evidence | Owner |
| --- | --- | --- | --- |
| **RC1** | **Error envelope shape.** bex-api's error writer (`lego/backend/internal/core/http.go:63,66`, `WriteJSON(w, code, map[string]string{"error": err.Error()})`) emitted `{"error": "<message>"}`. Render's real contract (the CLI's generated `Error` type, `cli/pkg/client/types_gen.go:2120`) is a flat `{"id": "...", "message": "..."}` body. Because `{"error": "..."}` was still valid JSON, `encoding/json` silently unmarshaled it into a zero-value `Error{}` (no error to fall back on) — the CLI's `firstNonNilErrorField` (`cli/pkg/client/client.go:115-149`) then reported the unhelpful generic `"unknown error"` for **every** bex-api 4xx/5xx the CLI hit, discarding the real (often perfectly good) message bex-api sent. **Fixed 2026-07-15** (`dfff3034`: error bodies now also carry Render's `{id,message}` shape, `lego/backend/internal/core/http.go`), re-verified live — `postgres get <unknown-id>` now shows a real, specific message. This was the single highest-impact, lowest-effort fix in this whole checklist — one response-writer shape change, system-wide payoff. | `keyvalues create` via the CLI printed `Error: unknown error`; the same request through a logging proxy showed bex-api actually said `{"error":"bad request: unknown maxmemoryPolicy \"allkeys_lru\" ...}"` — see RC4 (pre-fix). Post-fix: `render postgres get pg-cli-test -o json` → `Error: No Postgres database named 'pg-cli-test' in workspace tea-…` (real message, not `unknown error`). | **Fixed** — `.pm/w1/done/022.md` closed. |
| **RC2** | **`Service.autoDeploy` wire type.** bex-api serializes `autoDeploy` as a JSON boolean; Render's OpenAPI spec (and the CLI's generated `AutoDeploy` string-enum type, `cli/pkg/client/types_gen.go:1605-1606`, values `"yes"`/`"no"`) expects a string. Breaks every CLI path that decodes a full `Service` object. Same note also covers `deploys list`'s sibling `Deploy.image` wire-type mismatch below. **Fixed 2026-07-15** (`dfff3034`: `autoDeploy` now serializes `"yes"`/`"no"`, `lego/backend/internal/apps/render.go`; `deploys list`'s `Deploy.image` now serializes as Render's `{ref,sha,registryCredential}` object instead of a bare string, `lego/backend/internal/deploys/rest.go`). Re-verified live (dev-4 harness, `b4e3f64f`): `services` list/create/update/delete and `deploys list` all confirmed working end to end. At that pass, `services instances` still 404'd as a genuinely separate route gap; it later shipped independently in `w6/m26` (see its census row), not as part of RC2. | `.pm/w9/done/m2/evidence/log.md` §services list/delete/logs: `json: cannot unmarshal bool into Go struct field Service.service.autoDeploy of type client.AutoDeploy` (pre-fix). | **Fixed** — `w2/008` and the later `w6/m26` follow-up are both retired. |
| **RC3** | **Datastore list "cursor envelope" shape.** `GET /v1/postgres` / `GET /v1/key-value` return flat arrays of objects; the CLI expects each item wrapped `{postgres: {...}, cursor: "..."}` / `{keyValue: {...}, cursor: "..."}` (matching how `GET /v1/services` already wraps `{service: {...}, cursor: "..."}`, confirmed by direct REST — see `.pm/w9/done/m2/evidence/log.md`). Because Go's `encoding/json` doesn't error on a merely-differently-shaped object, every field silently decodes to its zero value instead of failing loudly — the CLI reports a "postgres/keyvalue" with every field empty, and any client-side match-by-name/id logic downstream (`get`, `update`, `suspend`) then can't find the real record. **Both halves fixed 2026-07-15** (`dfff3034`: `keyValueWithCursor`/`postgresWithCursor` envelopes + `?name=` server-side filtering, `lego/backend/internal/{keyvalue,postgres}/rest.go`), re-verified live against a rebuilt dev-9 — `keyvalues`/`postgres` list and get-by-name both work end to end now. **w8/m13 completed the paging contract:** explicit `cursor`/`limit` now walks a stable id-ordered list exactly once, unknown/final cursors return an empty page, omission preserves the complete pre-pagination result, GraphQL exposes matching optional args without breaking its dashboard list shape, and the official MCP list tools remain no-argument/all-items. | `.pm/w9/done/m2/evidence/log.md` §postgres list / keyvalues list: every field `""`/zero despite a real record existing; §postgres get: `Error: postgres not found` for an instance that demonstrably exists; §keyvalues suspend/update: `Error: Multiple Key Value instances found with name '…'` (pre-fix). Unit coverage: `TestPostgresListPaginationAcrossRESTAndGraphQL`, `TestKeyValueListPaginationAcrossRESTAndGraphQL`, and MCP input-schema assertions. | **Fixed** — envelope/name filtering in `dfff3034`; paging and adapter parity in `w8/m13`. |
| **RC4** | **KeyValue `maxmemoryPolicy` underscore vs. hyphen.** The CLI only ever sends Render's real wire values (`allkeys_lru`, underscore — `cli/pkg/client/types_gen.go:688-695`); bex-api's key-value create only accepted hyphenated values (`allkeys-lru`) and 400s on anything else. No CLI flag combination could work around this — it was a hard block on `keyvalues create` end to end. **Fixed 2026-07-15** (`dfff3034`: `renderToCRD`/`crdToRender` normalize both `maxmemoryPolicy` and `persistenceMode` between Render's underscore wire format and the CRD's hyphenated form, accepting either on input; `lego/backend/internal/keyvalue/service.go`), re-verified live — `keyvalues create` now succeeds end to end. | `.pm/w9/done/m2/evidence/log.md`: proxy-captured `RESP 400: {"error":"bad request: unknown maxmemoryPolicy \"allkeys_lru\" (valid: [... allkeys-lru ...])"}` (pre-fix). | **Fixed** — `w8/004` closed (`.pm/w8/done/004.md`). |
=======
export RENDER_HOST='https://api.bex.co/v1/'
export RENDER_API_KEY="$(
  curl --fail --silent --show-error \
    --request POST 'https://oauth.bex.co/oauth2/token' \
    --data-urlencode 'grant_type=client_credentials' \
    --data-urlencode "client_id=$BEX_API_KEY_ID" \
    --data-urlencode "client_secret=$BEX_API_KEY_SECRET" |
    jq --exit-status --raw-output '.access_token'
)"
>>>>>>> Stashed changes

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
<<<<<<< Updated upstream
| `login` | ✅ | Recognizes `RENDER_API_KEY` and skips the browser dashboard-login dance: `Success: CLI is already authenticated.` |
| `logout` | ✅ | Two independent paths, both verified. **`RENDER_API_KEY` path** (the primary supported flow): correctly refuses to "log out" an env-var credential and explains how to revoke via the dashboard — CLI-native behavior, no bex-api call at all, not a bex gap. **OAuth-credential path** (`render login`'s device flow — not itself implemented by bex, but the logout endpoint it needs is): the CLI calls `POST {host}/oauth/revoke` with the caller's own bearer token, expecting `204 No Content` (`cli/pkg/client/oauth/oauth.go:119-140`) — bex-api had no route at all (404). **Fixed** (`b4e3f64f`: `POST /v1/oauth/revoke`, `lego/backend/internal/apikeys/rest.go` — self-revoke deletes the caller's own underlying Hydra client, the same effect `RevokeAPIKey` gives a dashboard user revoking their own key; a session caller 400s rather than silently no-op'ing, since it has no Hydra client to delete). Re-verified live end to end: hand-crafted a `~/.render/cli.yaml`-equivalent OAuth config pointing at a real minted key's access token, ran `render logout` for real (not just curl) — printed `Successfully logged out of Render.`, and confirmed the token was actually dead afterward (401 on reuse, and the underlying `client_credentials` grant itself now fails since the Hydra client is gone). |
| `whoami` | ✅ | **`GET /v1/users` exists** (`dfff3034`, 2026-07-15, RC6) — no more 404, and the CLI's `whoami` prints `Name:`/`Email:` regardless of `-o json` (a CLI-native quirk, not a bex issue). The populated-email case verified live 2026-07-15 after wiring `BEX_KRATOS_ADMIN_URL` into the dev-9 harness (a `kratos-admin` port-forward on `:57090` + the env var in `up.sh`; dev-4 got the same wiring): `render whoami` for the API-key caller now prints the key-minting user's real email (`Email: cli-compat-…@example.com`), resolved through `ownerEmail`'s Kratos-admin lookup — see RC6. `Name:` stays empty by design: Kratos' default identity schema carries only email (an honest subset, not a bug). |
| `workspace current` | ✅ | Returns the real minted tenant (`tea-…`). |
| `workspace set` | ◐ | Not driven interactively in this harness; `RENDER_WORKSPACE` achieves the same effect through `config.WorkspaceID()`'s env-var precedence (`cli/pkg/config/config.go:123-131`) — every other command below relies on exactly that. Not a bex gap; no owner needed. |
| `workspaces` | ✅ | Lists the real minted tenant. |
| `projects` | ✅ | `null` (valid empty list — no projects created in this harness run; endpoint responds, no error). |
| `environments <id>` | ✅ | Verified live end to end against dev-4 after w4/m22: seeded a real Project plus `staging` Environment directly over authenticated REST (the CLI has no Project-create command), then `render environments <real-project-id> -o json` returned the real `env-…` id, owning `prj-…`, name, protection status, and non-null resource lists. This pass exposed and fixed RC15's flat-list-vs-`{environment,cursor}` bug. The unknown-id leg was rerun afterward and still exits nonzero with the specific, correct `received response code 404: app not found: project: not found`, not RC1's old masked `unknown error`. Throwaway rows were deleted after verification. |
| `services` (list) | ✅ | **Fixed** (`dfff3034`, RC2; `w6/m28`, RC13) — re-verified live end to end: created a real service, `render services -o json` returns it with correct fields (`autoDeploy:"no"`, etc.), and an empty-workspace list returns `null` cleanly. The generated Service output carries owner ID/dashboard but has no workspace-name or region fields; the verifier therefore asserts nested owner, configured `serviceDetails.region`, exact dashboard route, and authoritative timestamps on the raw REST object. |
| `services create` | ✅ | **Fixed** (`dfff3034`, RC9 — the `{service, deployId}` envelope, not merely "the same class of issue as RC2" as first suspected). Re-verified live: `render services create --name … --confirm -o json` now prints the full created record instead of `null`. |
| `services update` | ✅ | `--num-instances` is rejected **client-side** before any HTTP call (`--num-instances is not supported for update`) — a CLI-native restriction, not a bex gap. A supported field (`--health-check-path`) verified live end to end: resolves the service by name and returns the updated record. |
| `services delete` | ✅ | **Fixed** (RC2/RC9). Re-verified live: `render services delete <name> --confirm -o json` returns the deleted record with `meta.deleted: true`. |
| `services instances` | ✅ | **Fixed and re-verified live (`w6/m26`).** `GET /v1/services/{id}/instances` now authorizes the target service and projects its live Deployment Pods to the generated client's exact bare `[]ServiceInstance` shape (`id`, `createdAt`); the bex `/v1/apps/{id}/instances` alias mirrors it. A rebuilt `.pm/w6/dev-6/` pass created a real one-replica image service, and the unmodified `./cli/render services instances <srv-id> -o json` returned a non-null array with a stable compound `srv-…-<suffix>` id and Pod-derived timestamp; text mode rendered the same id. Suspending the service made the same CLI command return `[]`; an unknown id exited nonzero with 404/not-found semantics. `scripts/cli-compat.sh verify` now self-creates/cleans this service, waits for a decodable live instance, asserts both fields and timestamp parsing, covers text/empty/missing behavior, and fails on the original 404/`null`/malformed-object regressions. Lifecycle policy is explicit: include non-terminating Pending/Running/Unknown rollout Pods; exclude terminating/Succeeded/Failed Pods and all cron/static Pods. |
| `postgres create` | ✅ | Full, correct response body — real record created and returned intact. RC13's metadata pass additionally verifies a tenant-store owner, configured region, exact dashboard route, and advancing `updatedAt`; official CLI text prints Workspace, Region, and Dashboard. |
| `postgres create --ip-allow-list` | ✅ | **Fixed** (RC12) — was a hard failure (`Error: unknown error` / raw `{"error":"bad request body"}`) for the entire create, not just the flag; re-verified live, round-trips through create→list→get. |
| `postgres` (list) | ✅ | **Fixed** (`dfff3034`, RC3) — re-verified live: a created record's real fields (not zero values) come back; an empty workspace returns a clean `{"data":[]}`. |
| `postgres get` | ✅ | **Fixed** (RC3 + RC1). Re-verified live: resolves a real record by name; a nonexistent id now returns a real, specific Render-style error (`No Postgres database named '…' in workspace tea-…`) instead of the old opaque RC3 failure. `-o text` now shows the real workspace and configured region; raw REST also carries the dashboard route and authoritative update timestamp (RC13 fixed by `w6/m28`). |
| `postgres update --plan` | ✅ | **Fixed** (RC3). Re-verified live: `update --plan basic-256mb` returns the record with a `diff` block. |
| `postgres update --disk-size-gb` / `--high-availability` / `--ip-allow-list` / `--clear-ip-allow-list` | ✅ | **Fixed** (RC11) — every one of these previously 400'd (`plan must be one of ...`) because the handler required `plan` on every update; now applies independently of plan, verified against the CR's `spec` via `kubectl` directly (the response's `highAvailabilityEnabled` is the operator-observed status field, unpopulated without a running operator, so the spec write can't be trusted from the response alone). |
| `postgres update --name` (rename) | ✅ | Implemented across REST, GraphQL, MCP, and dashboard by separating immutable `dpg-…` identity from mutable `spec.name`; legacy ids are preserved. The unmodified pinned CLI (`c23438e`) create → old-name get → rename → new-name get path and recorded Kubernetes-identity comparison passed in dev-1…dev-10 and production on 2026-07-15. Production ran digest `ba32bf76ab6e`; the existing legacy CR backfill was idempotent and preserved all 11 recorded UIDs, while the fresh `dpg-…` smoke preserved its Database, CNPG Cluster, PVC, credential Secret, and connection Service UIDs across rename. |
| `postgres suspend` / `resume` / `delete` | ✅ | **Fixed** (RC3). Re-verified live, full lifecycle: `suspend` returns `suspended:"suspended"` + `meta.suspended:true`; `resume` returns `status:"available"`/`suspended:"not_suspended"`; `delete` returns `meta.deleted:true`. |
| `keyvalues create` | ✅ | **Fixed 2026-07-15 (`dfff3034` RC4/RC1 + RC14).** Re-verified live, field-by-field, against a freshly rebuilt dev-9 bex-api: `render keyvalues create --name … --ip-allow-list "cidr=10.0.0.0/8,description=…" --confirm -o json` returns a real record with `ownerId` correctly populated (was silently `""`), `maxmemoryPolicy`/`persistenceMode` present under a nested `options`-equivalent shape (were silently omitted entirely), and `ipAllowList` as `{cidrBlock,description}` entries (was a flat-string 400). Earlier "✅ Fixed" claims for this row (RC4 alone) were true but incomplete — they checked the id/name came back, not that every field did. |
| `keyvalues` (list) | ✅ | **Fixed 2026-07-15 (`dfff3034` RC3 + RC14).** `GET /v1/key-value` returns Render's `{keyValue, cursor}` envelope with every field intact — re-verified live with the same field-by-field check as `create`, not just id/name presence. |
| `keyvalues get` | ✅ | **Fixed 2026-07-15 (`dfff3034` RC3/RC7 + RC14; `w6/m28` RC13).** Server-side `?name=` filtering resolves cleanly, and the resolved record carries `ownerId`/`options`/`ipAllowList` plus the tenant-store workspace, configured region, dashboard route, and authoritative timestamp — re-verified live in raw REST and official CLI JSON/text, with no more silently empty fields. |
| `keyvalues update` | ✅ | Re-verified live: `render keyvalues update … --plan free --confirm -o json` resolves and returns the full, correctly-shaped record. |
| `keyvalues suspend` | ✅ | Re-verified live: returns the full record with `meta.suspended: true`. |
| `keyvalues resume` | ✅ | Re-verified live: returns the full record with `status: "available"`. |
| `keyvalues delete` | ✅ | Re-verified live: returns `meta.deleted: true`; a follow-up `keyvalues list` correctly shows `data: []` — confirmed no leftover test resources across three separate verification passes this session. |
| `deploys list` | ✅ | **Fixed** (RC2's `Deploy.image` wire-type fix — bex now sends `{ref,sha,registryCredential}` instead of a bare string). Re-verified live: `render deploys list <service> -o json` returns real deploy records with `"image":{"ref":"…"}`. |
| `deploys create` | ✅ | **Fixed** (RC10 — the `clearCache` bool-vs-string-enum mismatch RC1's fix unmasked). Re-verified live: `render deploys create <service> --confirm -o json` returns the newly created deploy. |
| `deploys cancel` | ✅ | **Fixed** (RC1 + RC10, downstream of `deploys create` working at all). Re-verified live against a real in-progress deploy: `render deploys cancel <service> <deployId> --confirm -o json` → `"Deploy … successfully canceled"`. |
| `restart` | ✅ | **Fixed** (RC5 Service half). Focused dev-5 retest: the CLI accepted the listed `srv-…` id, returned `restarted successfully`, stamped `App.spec.restartedAt`, and rolled to a new ready pod. |
| `logs` | ✅ | **Fixed** (RC7 — the missing `?name=` filter that made the CLI's own name-to-id resolution silently fail — plus RC8, the `nextStartTime`/`nextEndTime` empty-string crash on a zero-result query; NOT RC2 as the original audit guessed — by the time this was retested RC2 was already fixed, and the failure mode had moved on to RC7). The focused dev-5 pass additionally fixed tenant-prefixed physical App selection: both typed-id and public-name requests returned the real pod startup line, with the response's `resource` label normalized to the typed `srv-…` id. |
| `jobs list` | ✅ | `GET /v1/services/{id}/jobs` returns `[]jobWithCursor`; empty/non-empty lists and status filters were verified against the dev-7 isolated environment. |
| `jobs create` | ✅ | `POST /v1/services/{id}/jobs` returns Render's `id`/`serviceId`/`startCommand`/`planId`/`status`/`createdAt` shape and creates a Kubernetes Job from the service image. |
| `jobs cancel` | ✅ | `POST /v1/services/{id}/jobs/{jobId}/cancel` returns the canceled job and returns 409 once terminal; verified live with `render jobs cancel --confirm`. |
| `workflows list` | — | 404 — Render Workflows is an already-documented deliberate non-goal (ADR018 §Gap backlog: "Deliberate non-goals … workflows"). |
| `blueprints validate` | ✅ | **Fixed and re-verified live with the unmodified CLI (2026-07-15).** `POST /v1/blueprints/validate` now accepts Render's exact multipart request (`ownerId` field + raw YAML `file` part) while retaining bex's JSON body for existing dashboard/direct callers. `ownerId` is bound into the shared workspace authorization seam (a forged workspace re-test returns 403). The response matches the CLI's `ValidateBlueprintResponse`: valid files return `valid:true` plus a resource `plan`; invalid Blueprint content is a **200 validation result**, not a transport error, with `errors:[{error,line?,column?,path?}]`. Real `./cli/render` evidence against dev-6: `examples/hello-go/bex.yml` succeeds in JSON and interactive modes (1 service); `examples/stack-demo/bex.yml` reports 2 services + 1 database; `examples/whoami-app.yaml` cleanly reports `valid:false`; piped malformed YAML reports `line:3`; a missing service name reports `path:"services[0].name"`. Backend `go build ./...`, `go test ./...`, and `make lint-backend` pass. **Honest divergence:** bex's plan is a declaration summary (`totalActions` = declared supported resources), not Render's existing-resource field-level diff; Blueprint Key Value entries remain unsupported and `keyValue` is omitted. |
| `ssh` | ◐ | **Running-instance SSH implemented by w2/m39:** the CLI-required `sshAddress`, live-deploy data, instances route, and raw OpenSSH gateway contract are present and protocol-tested. The CLI is interactive by design, so it requires the live PTY harness in `scripts/ssh-verify.sh`; ◐ until that public TCP/22 run is captured. The current official release, v2.21.0 at `c398207`, accepts service name, service id, or full instance id, but its service-id instance-picker callback assigns the service id instead of the selected instance id (`cmd/ssh.go`); the verifier therefore proves name resolution and exact targeting through the supported direct-instance-id argument without patching the CLI or falsely claiming the menu works. `--ephemeral` remains a deliberate non-goal. [ADR035](ADR035-ssh.md). |
| `kv-cli` | — | Same client-side interactive-only refusal; hosted-exec non-goal. |
| `pgcli` | — | Same client-side interactive-only refusal; hosted-exec non-goal. |
| `psql` | ◐ | The one session command that supports non-interactive mode (`--command`). It reaches bex's real Postgres and is correctly turned away at the network layer: `IP address (…) not in allow list for` — expected in this harness (`BEX_DB_DOMAIN` unset in dev-9 ⇒ internal-only, per the root `CLAUDE.md` env-var table), not a bug; would need `BEX_DB_DOMAIN` configured to verify end to end. Operational config, not a code gap — no owner needed. |
| `docs` | ✅ | Opens the (Render, not bex) docs URL in a browser — no bex-api call by design. |
| `skills` | ✖ | Panics (`nil pointer dereference`, `cli/pkg/tui/stack.go:105`) before any HTTP call reaches bex-api — a bug in `render-oss/cli` itself, reproducible against api.render.com too in principle. Not a bex compatibility finding; nothing for bex-api to fix; no owner needed (bex never patches the CLI, `.pm/DO_NOT_DO.md`). |
| `ea` (early access) | — | Lists early-access subcommands (`objects`, `sandbox`, `sandbox-groups`) — object storage and hosted sandboxes are both off bex's roadmap today (sandboxes: pillar 5, `.pm/DO_NOT_DO.md`); not walked further. |
=======
| `login` (browser/device flow) | ✖ | The official CLI uses a fixed public OAuth client, `/device-grant`, `/device-token`, refresh tokens, and `/token/refresh/`. bex does not yet provide that compatibility flow; `RENDER_API_KEY` only bypasses it. Tracked by `w4/m25`. |
| `logout` | ◐ | Correctly explains that it cannot clear `RENDER_API_KEY`. Real device-token logout and refresh-chain revocation depend on the missing browser login flow. |
| `whoami` | ✅ | Returns the key owner's email when the Kratos admin URL is configured. |
| `workspace current`, `workspaces` | ✅ | Return the caller's real `tea-…` workspace. |
| `workspace set` | ◐ | The interactive flow was not tested. `RENDER_WORKSPACE` is the supported deterministic override and is used by the production setup above. |
| `projects`, `environments <project-id>` | ✅ | Lists projects and returns Render-compatible environment records. |
| `services`, `services create`, `services update`, `services delete` | ✅ | List and lifecycle operations return the expected Render wire shapes. The upstream CLI does not support `--num-instances` on update; supported fields such as `--health-check-path` work. |
| `services instances` | ✅ | Returns live, non-terminal Deployment pods; suspended services return an empty list. |
| `postgres create`, list, get, update, suspend, resume, delete | ✅ | Includes plan, disk, high-availability, and IP allow-list changes. |
| `postgres update --name` | ✅ | Rename preserves the immutable `dpg-…` ID and Kubernetes identity. Production passed on 2026-07-15 using digest `ba32bf76ab6e` (deploy run `29406643202`): the legacy backfill preserved all 11 recorded UIDs, and a fresh resource passed the official-CLI identity smoke test. |
| `keyvalues create`, list, get, update, suspend, resume, delete | ✅ | Owner, options, persistence policy, and structured IP allow-list fields round-trip correctly. |
| `deploys list`, `deploys create`, `deploys cancel` | ✅ | Deploy creation accepts the CLI's cache enum and returns Render-compatible deploy records. |
| `restart`, `logs` | ✅ | Accept typed service IDs and public names. Logs return stable cursors even when the result is empty. |
| `jobs list`, `jobs create`, `jobs cancel` | ✅ | One-off jobs use the service image and return the expected job shape. |
| `blueprints validate` | ✅ | Accepts the CLI's multipart Blueprint request. The plan is a declaration summary, not Render's field-level diff; Blueprint Key Value entries are not supported. |
| `psql --command` | ◐ | Reaches managed Postgres, but end-to-end access requires `BEX_DB_DOMAIN` and an IP allow-list that permits the caller. |
| `docs` | ✅ | Opens Render's documentation; it makes no bex API call. |
| `skills` | ✖ | The tested CLI revision panics before making an HTTP request. This is an upstream CLI issue, not a bex API incompatibility. |
| `workflows list` | — | Render Workflows is a deliberate bex non-goal. |
| `ssh`, `kv-cli`, `pgcli` | — | Interactive hosted exec is a deliberate non-goal. The CLI also rejects these commands in non-interactive mode before contacting bex. |
| `ea` (`objects`, `sandbox`, `sandbox-groups`) | — | These early-access surfaces are outside the current compatibility target. |
>>>>>>> Stashed changes

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

[`scripts/cli-compat.sh`](../scripts/cli-compat.sh) exchanges the development key pair, sets `RENDER_HOST`, `RENDER_API_KEY`, and `RENDER_WORKSPACE`, then runs the unmodified CLI. Its `verify` mode covers the automated compatibility assertions and cleans up the resources it creates. Browser login is deliberately not counted as verified merely because `RENDER_API_KEY` makes `render login` short-circuit as already authenticated.

Postgres rename has a focused smoke test in [`scripts/postgres-rename-cli-smoke.sh`](../scripts/postgres-rename-cli-smoke.sh). Run [`scripts/postgres-name-migrate.sh`](../scripts/postgres-name-migrate.sh) first for legacy records; [ADR009](ADR009-postgresql-management.md#rename-and-legacy-migration) documents the rollout and rollback order. Production used that sequence through deploy run `29406643202` and digest `ba32bf76ab6e`.

The cross-surface backlog remains in [ADR018-render-parity.md](ADR018-render-parity.md); this document does not duplicate ownership tracking.
